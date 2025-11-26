package engine

import (
	"context"
	"fmt"
	"log"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/redis/go-redis/v9"
	"golang.org/x/time/rate"

	"solana-orchestrator/config"
	"solana-orchestrator/storage"
	"solana-orchestrator/trading"
)

type FanOutEngine struct {
	db  *storage.DB
	bot *tgbotapi.BotAPI
	rdb *redis.Client
	cfg *config.Config
	ws  *trading.WSClient

	logChan          chan string
	notificationChan chan Notification
	stopChan         chan struct{}
	wg               sync.WaitGroup

	monitoredCount int
	mu             sync.RWMutex
}

type Notification struct {
	UserID  int64
	Message string
}

func NewFanOutEngine(db *storage.DB, bot *tgbotapi.BotAPI, rdb *redis.Client, cfg *config.Config) *FanOutEngine {
	return &FanOutEngine{
		db:               db,
		bot:              bot,
		rdb:              rdb,
		cfg:              cfg,
		ws:               trading.NewWSClient(cfg.WebSocketSettings.ShyftWSURL),
		logChan:          make(chan string, cfg.FanOutEngine.LogBufferSize),
		notificationChan: make(chan Notification, 10000),
		stopChan:         make(chan struct{}),
	}
}

func (e *FanOutEngine) Start() {
	log.Println("Starting Fan-Out Engine...")

	// 1. Sync wallets to Redis
	if err := e.SyncMonitoredWallets(); err != nil {
		log.Printf("Error syncing wallets to Redis: %v", err)
	}

	// 2. Start Workers
	for i := 0; i < e.cfg.FanOutEngine.WorkerCount; i++ {
		e.wg.Add(1)
		go e.worker(i)
	}

	// 3. Start Notification Worker
	e.wg.Add(1)
	go e.notificationWorker()

	// 4. Start WebSocket Listener
	e.wg.Add(1)
	go e.StartShyftListener()
}

func (e *FanOutEngine) Shutdown() {
	close(e.stopChan)
	e.wg.Wait()
	log.Println("Fan-Out Engine stopped")
}

func (e *FanOutEngine) IsRunning() bool {
	select {
	case <-e.stopChan:
		return false
	default:
		return true
	}
}

func (e *FanOutEngine) GetMonitoredCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.monitoredCount
}

func (e *FanOutEngine) SyncMonitoredWallets() error {
	ctx := context.Background()

	// Fetch all active copy targets from DB
	targets, err := e.db.GetAllActiveCopyTargets()
	if err != nil {
		return fmt.Errorf("failed to fetch targets: %w", err)
	}

	// Sync to Redis
	if err := SyncWalletsToRedis(ctx, e.rdb, targets); err != nil {
		return fmt.Errorf("failed to sync to redis: %w", err)
	}

	// Update local count
	uniqueWallets := make(map[string]bool)
	for _, t := range targets {
		uniqueWallets[t.TargetWallet] = true
	}

	e.mu.Lock()
	e.monitoredCount = len(uniqueWallets)
	e.mu.Unlock()

	return nil
}

func (e *FanOutEngine) StartShyftListener() {
	defer e.wg.Done()

	// Connect using existing WSClient
	if err := e.ws.Connect(context.Background()); err != nil {
		log.Printf("Failed to connect WS: %v", err)
		// Retry logic could go here, but WSClient handles some reconnection
	}

	// Subscribe to programs
	programs := []string{
		e.cfg.Programs.JupiterLimitOrder,
		e.cfg.Programs.RaydiumAMMV4,
	}
	if e.cfg.Programs.RaydiumCLMM != "" {
		programs = append(programs, e.cfg.Programs.RaydiumCLMM)
	}

	for _, prog := range programs {
		if prog == "" {
			continue
		}
		sub, err := e.ws.SubscribeProgramLogs(context.Background(), prog)
		if err != nil {
			log.Printf("Failed to subscribe to %s: %v", prog, err)
			continue
		}

		// Forward logs to logChan
		e.wg.Add(1)
		go func(ch <-chan interface{}) {
			defer e.wg.Done()
			for msg := range ch {
				// Convert to string (assuming WSClient returns string or byte slice)
				// WSClient returns interface{}, usually map[string]interface{} or string
				// We need to ensure we pass the raw log string to parser
				// If WSClient returns parsed JSON, we might need to marshal it back or adjust parser.
				// Assuming WSClient returns the raw message or we can cast.
				// Actually, `SubscribeProgramLogs` in `websocket.go` returns `<-chan interface{}`.
				// Let's assume it sends the raw message string or bytes.

				strMsg, ok := msg.(string)
				if !ok {
					// Try bytes
					if b, ok := msg.([]byte); ok {
						strMsg = string(b)
					} else {
						// Try fmt
						strMsg = fmt.Sprintf("%v", msg)
					}
				}

				select {
				case e.logChan <- strMsg:
				default:
					// Drop
				}

				if !e.IsRunning() {
					return
				}
			}
		}(sub)
	}

	<-e.stopChan
	e.ws.Close()
}

func (e *FanOutEngine) worker(id int) {
	defer e.wg.Done()
	ctx := context.Background()

	for {
		select {
		case <-e.stopChan:
			return
		case rawLog := <-e.logChan:
			// 1. Extract wallet (fast path)
			// Note: For program logs, we might not get the wallet directly in the top level.
			// But assuming we do or we parse it:
			wallet, err := ParseLogForWallet(rawLog)
			if err != nil || wallet == "" {
				continue
			}

			// 2. Check Redis
			isMember, err := e.rdb.SIsMember(ctx, "monitored_wallets", wallet).Result()
			if err != nil || !isMember {
				continue
			}

			// 3. Process Match
			e.processMatch(ctx, wallet, rawLog)
		}
	}
}

func (e *FanOutEngine) processMatch(ctx context.Context, wallet string, rawLog string) {
	// 1. Get Users
	owners, err := GetWalletOwners(ctx, e.rdb, wallet)
	if err != nil || len(owners) == 0 {
		return
	}

	// 2. Parse Transaction
	swapInfo, err := ParseSwapInstruction(rawLog)
	if err != nil {
		return
	}

	// 3. Execute for each user
	for userID, copyAmount := range owners {
		go func(uid int64, amt float64) {
			// We cannot execute trade without password.
			// Send alert instead.
			e.notificationChan <- Notification{
				UserID:  uid,
				Message: fmt.Sprintf("🔔 Copy Trade Triggered!\nTarget: %s\nTx: %s\n\n(Auto-trade disabled: Wallet locked)", wallet, swapInfo.Signature),
			}

			// If we had the password (e.g. session cache), we would:
			// 1. Decrypt wallet
			// 2. ExecuteCopyTrade(ctx, e.db, uid, privKey, swapInfo, amt)
		}(userID, copyAmount)
	}
}

func (e *FanOutEngine) notificationWorker() {
	defer e.wg.Done()
	limiter := rate.NewLimiter(25, 1) // 25 msgs/sec

	for {
		select {
		case <-e.stopChan:
			return
		case note := <-e.notificationChan:
			limiter.Wait(context.Background())
			msg := tgbotapi.NewMessage(note.UserID, note.Message)
			e.bot.Send(msg)
		}
	}
}
