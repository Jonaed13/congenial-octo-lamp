package engine

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"github.com/tidwall/gjson"
)

// FanOutService manages the WebSocket connection and worker pool
type FanOutService struct {
	wsURL         string
	redisClient   *redis.Client
	workerCount   int
	logChan       chan string // Channel for raw log strings
	tradeExecChan chan TradeSignal
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
}

// TradeSignal represents a detected trade opportunity
type TradeSignal struct {
	WalletAddress string
	TokenAddress  string
	IsBuy         bool
	Amount        float64
	Signature     string
}

// NewFanOutService creates a new FanOutService
func NewFanOutService(wsURL string, redisClient *redis.Client, workerCount int) *FanOutService {
	ctx, cancel := context.WithCancel(context.Background())
	return &FanOutService{
		wsURL:         wsURL,
		redisClient:   redisClient,
		workerCount:   workerCount,
		logChan:       make(chan string, 1000), // Buffered channel
		tradeExecChan: make(chan TradeSignal, 100),
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Start initializes the WebSocket and worker pool
func (s *FanOutService) Start() {
	log.Println("🚀 Starting Fan-Out Engine...")

	// 1. Start Workers
	for i := 0; i < s.workerCount; i++ {
		s.wg.Add(1)
		go s.worker(i)
	}

	// 2. Start WebSocket Reader
	s.wg.Add(1)
	go s.wsReader()
}

// Stop gracefully shuts down the service
func (s *FanOutService) Stop() {
	log.Println("🛑 Stopping Fan-Out Engine...")
	s.cancel()
	s.wg.Wait()
	close(s.logChan)
	close(s.tradeExecChan)
}

// wsReader maintains the WebSocket connection
func (s *FanOutService) wsReader() {
	defer s.wg.Done()

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			// Connect to WebSocket
			c, _, err := websocket.DefaultDialer.Dial(s.wsURL, nil)
			if err != nil {
				log.Printf("❌ WebSocket dial error: %v. Retrying in 5s...", err)
				time.Sleep(5 * time.Second)
				continue
			}
			log.Println("✅ WebSocket connected")

			// Subscribe message (Shyft/Helius format)
			// This is a placeholder subscription message. Adjust based on actual provider API.
			subMsg := `{"jsonrpc":"2.0", "id":1, "method":"logsSubscribe", "params":[{"mentions": ["675kPX9MHTjS2zt1qfr1NYHuzeLXfQM9H24wFSUt1Mp8"]}, {"commitment": "processed"}]}` // Raydium V4
			if err := c.WriteMessage(websocket.TextMessage, []byte(subMsg)); err != nil {
				log.Printf("❌ Subscription error: %v", err)
				c.Close()
				continue
			}

			// Read Loop
			for {
				_, message, err := c.ReadMessage()
				if err != nil {
					log.Printf("❌ Read error: %v", err)
					break // Reconnect
				}

				// Non-blocking send to workers
				select {
				case s.logChan <- string(message):
				default:
					log.Println("⚠️ Log channel full, dropping message")
				}
			}
			c.Close()
			time.Sleep(1 * time.Second) // Backoff before reconnect
		}
	}
}

// worker processes incoming logs
func (s *FanOutService) worker(id int) {
	defer s.wg.Done()
	log.Printf("🔧 Worker %d started", id)

	for rawLog := range s.logChan {
		// 1. Fast Parse with gjson (Zero Allocation)
		// Extract relevant fields (adjust paths based on actual RPC response format)
		// Example: params.result.value.logs
		// For this example, we assume we are looking for a signer or a mentioned account

		// Let's assume the log contains the transaction signature and the accounts involved
		// In a real "logsSubscribe", we get logs. We might need to parse the instruction data if available,
		// or just trigger on the signature if we are monitoring a specific wallet.

		// Scenario: We are monitoring a set of "Target Wallets" (Copy Trading).
		// We check if any of our monitored wallets are the "signer" or "owner" in this transaction.

		// NOTE: Standard logsSubscribe usually gives logs + signature. It doesn't always give the full account list
		// unless we use "transactionSubscribe" (Helius/Geyser).
		// Assuming we use a provider that gives us the signer or we parse it from the log context.

		// For this implementation, let's assume `rawLog` contains a field `params.result.value.pubkey`
		// or we are parsing program logs to find "Transfer from <Wallet>".

		// Let's use a hypothetical field for the wallet address for demonstration.
		// In reality, you'd parse `params.result.value.signature` and fetch tx, OR use Geyser to get accounts.
		// If using Helius/Shyft enhanced WebSockets, we might get parsed transactions.

		// Let's assume we get a parsed transaction object.
		walletAddr := gjson.Get(rawLog, "params.result.value.accountData.0.account").String()

		// If empty, maybe it's a different message type
		if walletAddr == "" {
			continue
		}

		// 2. Redis O(1) Lookup (Step A)
		// Check if this wallet is in our monitored set
		isMonitored, err := s.redisClient.SIsMember(context.Background(), "monitored_wallets", walletAddr).Result()
		if err != nil {
			log.Printf("Redis error: %v", err)
			continue
		}

		if !isMonitored {
			continue // Not interested
		}

		// 3. Process Match (Step B)
		// We found a match! Now we parse deeper to see what they did (Buy/Sell).
		log.Printf("🎯 Match found for wallet: %s", walletAddr)

		// Extract signature
		signature := gjson.Get(rawLog, "params.result.value.signature").String()

		// Determine trade type (simplified logic)
		// In reality, check instruction discrimination
		isBuy := true

		// 4. Trigger Trade (Step C)
		s.tradeExecChan <- TradeSignal{
			WalletAddress: walletAddr,
			TokenAddress:  "So11111111111111111111111111111111111111112", // Placeholder
			IsBuy:         isBuy,
			Amount:        1.0,
			Signature:     signature,
		}
	}
}

// GetTradeChannel returns the channel for trade signals
func (s *FanOutService) GetTradeChannel() <-chan TradeSignal {
	return s.tradeExecChan
}
