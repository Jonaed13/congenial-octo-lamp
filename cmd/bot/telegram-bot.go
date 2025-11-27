package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"solana-orchestrator/analyzer"
	"solana-orchestrator/api"
	"solana-orchestrator/config"
	"solana-orchestrator/engine"
	iengine "solana-orchestrator/internal/engine"
	isolana "solana-orchestrator/internal/solana"
	"solana-orchestrator/storage"

	"github.com/gagliardetto/solana-go"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/redis/go-redis/v9"
)

type UserSession struct {
	State       string
	RequestedAt int64
	Winrate     float64
	StartCount  int
	ScanType    string // "realtime" or "slow"
	TempData    map[string]interface{}
}

type Scanner struct {
	db            *storage.DB
	mu            sync.RWMutex
	scannedCount  int
	totalWallets  int
	lastScanStart int64
	isScanning    bool
	walletsCache  map[string]*storage.WalletData // In-memory cache for fast lookups
	walletsList   []*storage.WalletData          // Ordered list for scalable iteration
}

type PendingScan struct {
	UserID      int64
	Results     []*storage.WalletData
	DeliverAt   time.Time
	ScanType    string
	Winrate     float64
	RealizedPnL float64
}

var (
	scanner        *Scanner
	sessions       = make(map[int64]*UserSession)
	sessMu         sync.RWMutex
	tempWalletAddr = make(map[int64]string) // Temporary storage for wallet addresses during input
	globalCfg      *config.Config           // Global config for use in handlers
	pendingScans   = make(map[int64]*PendingScan)
	pendingScansMu sync.RWMutex
	// copyEngine     *trading.CopyTradeEngine // Deprecated
	fanoutEngine *engine.FanOutEngine
	redisClient  *redis.Client
)

func main() {
	// Initialize random seed for slow scan delays
	rand.Seed(time.Now().UnixNano())

	cfg, err := config.Load("config/config.json")
	if err != nil {
		log.Fatal(err)
	}

	// Store config globally for handlers
	globalCfg = cfg

	// Initialize DB
	db, err := storage.New("bot.db")
	if err != nil {
		log.Fatal(err)
	}

	// Initialize scanner with DB and cache
	scanner = &Scanner{
		db:           db,
		walletsCache: make(map[string]*storage.WalletData),
		walletsList:  make([]*storage.WalletData, 0),
	}

	log.Printf("📦 Scanner initialized with empty cache")

	// Get bot token from environment
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN environment variable not set")
	}

	// Initialize bot
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Bot started: @%s", bot.Self.UserName)

	// Start cleanup routine
	go cleanupRoutine(db)

	// Start continuous scanning with reduced concurrency
	go continuousScanner(cfg, bot)

	// Initialize Redis
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	redisClient, err = engine.NewRedisClient(redisAddr, "", 0)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	// defer redisClient.Close() // We let it run until exit

	// Initialize Fan-Out Engine
	shyftAPIKey := os.Getenv("SHYFT_API_KEY")
	if shyftAPIKey == "" {
		// Fallback to config if env not set, though env is preferred
		shyftAPIKey = cfg.ShyftAPIKey
		if shyftAPIKey == "" {
			// Extract from URL as last resort
			shyftAPIKey = api.ExtractAPIKey(cfg.WebSocketSettings.ShyftWSURL)
		}
	}

	if shyftAPIKey == "" {
		log.Fatal("SHYFT_API_KEY environment variable required")
	}

	fanoutEngine = engine.NewFanOutEngine(db, bot, redisClient, cfg)
	fanoutEngine.Start()
	// defer fanoutEngine.Shutdown()

	log.Println("🚀 Fan-Out Engine started successfully")

	// Initialize Jito Client
	var jitoClient *isolana.JitoClient
	if cfg.TradingSettings.JitoPrivateKey != "" {
		jitoKey, err := solana.PrivateKeyFromBase58(cfg.TradingSettings.JitoPrivateKey)
		if err != nil {
			log.Printf("⚠️ Invalid Jito Private Key: %v. Jito features disabled.", err)
		} else {
			jitoClient = isolana.NewJitoClient(cfg.TradingSettings.JitoBlockEngineURL, jitoKey)
			log.Println("✅ Jito Client initialized")
		}
	} else {
		log.Println("⚠️ Jito Private Key not set. Jito features disabled.")
	}

	// Initialize Limit Order Manager
	// We need RPC endpoint. Assuming it's in config or we use a default.
	// Config doesn't have explicit RPC endpoint field in root, maybe in APISettings?
	// Let's check config.go again. It has Moralis/Birdeye keys but not generic RPC.
	// We'll use a default public RPC if not found, or maybe we can reuse the client's endpoint if exposed.
	// For now, let's use a hardcoded public RPC or add to config.
	// Better to add to config, but for bug fix, let's use a known public one or empty string if Manager handles it.
	rpcURL := "https://api.mainnet-beta.solana.com" // Fallback
	limitOrderManager := isolana.NewLimitOrderManager(rpcURL, jitoClient, db)

	// Initialize Janitor
	// Janitor needs JitoClient and LimitOrderManager
	janitor := iengine.NewJanitor(db, jitoClient, limitOrderManager)
	janitor.Start()
	log.Println("🧹 Janitor service started")

	// Start Copy Trade Engine (DEPRECATED - Replaced by Fan-Out Engine)
	// shyftKey := api.ExtractAPIKey(cfg.WebSocketSettings.ShyftWSURL)
	// copyEngine = trading.NewCopyTradeEngine(db, bot, shyftKey)
	// go copyEngine.Start()

	// Handle updates
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message != nil {
			handleMessage(bot, update.Message)
		} else if update.CallbackQuery != nil {
			handleCallback(bot, update.CallbackQuery)
		}
	}
}

func cleanupRoutine(db *storage.DB) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		deleted, err := db.CleanupOldData()
		if err != nil {
			log.Printf("❌ Cleanup error: %v", err)
			continue
		}
		if deleted > 0 {
			log.Printf("🧹 Cleaned up %d old wallet records", deleted)
		}
	}
}

func continuousScanner(cfg *config.Config, bot *tgbotapi.BotAPI) {
	client := api.NewClient(cfg.MoralisAPIKey, cfg.BirdeyeAPIKey, cfg.APISettings.MaxRetries, cfg.MoralisFallbackKeys)

	for {
		log.Println("🔄 Starting new scan cycle...")
		scanner.mu.Lock()
		scanner.lastScanStart = time.Now().Unix()
		scanner.scannedCount = 0
		scanner.isScanning = true
		scanner.mu.Unlock()

		// Publish scan start to Redis
		publishScanProgress(0, 0, true, 0)

		var tokens []api.Token
		var err error

		if cfg.APISettings.TokenSource == "moralis" {
			log.Printf("Fetching graduated tokens from Moralis...")
			tokens, err = client.FetchGraduatedTokens(context.Background(), cfg.APISettings.TokenLimit)
		} else {
			log.Printf("Fetching tokens from Birdeye...")
			tokens, err = client.FetchBirdeyeTokens(context.Background(), cfg.APISettings.TokenLimit)
		}

		if err != nil {
			log.Printf("❌ Token fetch error: %v", err)
			time.Sleep(5 * time.Minute)
			continue
		}

		walletSet := make(map[string]bool)
		for _, token := range tokens {
			// Get Holders
			holders, err := client.GetTokenHolders(context.Background(), token.TokenAddress)
			if err == nil {
				for _, h := range holders {
					walletSet[h.OwnerAddress] = true
				}
			}

			// Get Top Traders (if enabled)
			if cfg.APISettings.FetchTraders {
				traders, err := client.FetchTopTraders(context.Background(), token.TokenAddress)
				if err == nil {
					for _, t := range traders {
						walletSet[t] = true
					}
				}
				time.Sleep(200 * time.Millisecond) // Rate limit
			}

			time.Sleep(200 * time.Millisecond) // Faster fetching
		}

		wallets := make([]string, 0, len(walletSet))
		for w := range walletSet {
			wallets = append(wallets, w)
		}

		scanner.mu.Lock()
		scanner.totalWallets = len(wallets)
		scanner.mu.Unlock()

		log.Printf("📊 Scanning %d wallets...", len(wallets))

		// Publish initial scan progress
		publishScanProgress(0, len(wallets), true, 0)

		// Use filters from config
		a := analyzer.NewAnalyzer(6, cfg.AnalysisFilters.MinWinrate, cfg.AnalysisFilters.MinRealizedPnL)
		results, err := a.AnalyzeWallets(context.Background(), wallets, func(r *analyzer.WalletStats) {
			scanner.mu.Lock()
			w := &storage.WalletData{
				Wallet:      r.Wallet,
				Winrate:     r.Winrate,
				RealizedPnL: r.RealizedPnL,
				ScannedAt:   time.Now().Unix(),
			}

			// Save to DB and Cache
			if err := scanner.db.SaveWallet(w); err != nil {
				log.Printf("DB Error: %v", err)
			}

			// Check if new wallet for the list
			if _, exists := scanner.walletsCache[w.Wallet]; !exists {
				scanner.walletsList = append(scanner.walletsList, w)
			}

			scanner.walletsCache[w.Wallet] = w
			scanner.scannedCount++ // Increment progress counter

			// Publish progress update every 10 wallets
			if scanner.scannedCount%10 == 0 {
				publishScanProgress(scanner.scannedCount, scanner.totalWallets, true, len(scanner.walletsList))
			}
			scanner.mu.Unlock()
		})

		if err != nil {
			log.Printf("Analysis error: %v", err)
		}

		// Update final stats
		scanner.mu.Lock()
		scanner.scannedCount = len(results)
		scanner.isScanning = false
		foundCount := len(scanner.walletsList)
		scanner.mu.Unlock()

		// Publish scan complete to Redis
		publishScanProgress(len(results), len(results), false, foundCount)

		log.Printf("✅ Scan complete: %d wallets stored", len(results))
		time.Sleep(30 * time.Minute)
	}
}

func handleMessage(bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	chatID := msg.Chat.ID

	if msg.IsCommand() {
		switch msg.Command() {
		case "start":
			// Check if user exists
			user, err := scanner.db.GetUser(chatID)
			if err != nil {
				log.Printf("Error getting user: %v", err)
			}

			if user == nil {
				// Create new user
				if err := scanner.db.CreateUser(chatID); err != nil {
					log.Printf("Error creating user: %v", err)
				}
				// Show Welcome & Trial Options
				showTrialOptions(bot, chatID)
				return
			}

			// If user exists but has no plan (legacy or interrupted flow), show options
			if user.PlanType == "" {
				showTrialOptions(bot, chatID)
				return
			}

			// Show Main Menu
			showMainMenu(bot, chatID)

		case "status":
			sendStatus(bot, chatID)

		case "balance":
			handleBalanceCommand(bot, chatID)

		case "wallets":
			handleWalletsCommand(bot, chatID)
		case "admin":
			handleAdminCommand(bot, chatID)
		case "menu":
			showMainMenu(bot, chatID)
		case "copytrade":
			handleCopyTradeCommand(bot, chatID)
		case "buy":
			handleStartBuy(bot, chatID)
		case "sell":
			handleStartSell(bot, chatID)
		}
		return
	}

	// Handle Persistent Menu Commands
	if msg.Text == "🔍 Dev Finder" {
		showScanTypeModal(bot, chatID)
		return
	} else if msg.Text == "💰 My Credits" {
		handleBalanceCommand(bot, chatID)
		return
	} else if msg.Text == "❓ Help/FAQ" {
		send(bot, chatID, "📚 *Help & FAQ*\n\n*Credits*: 1 Credit is deducted for every wallet processed during a scan.\n*Dev Finder*: Scans Solana for profitable wallets based on your Win Rate and PnL filters.\n\nNeed more help? Contact support.")
		return
	} else if msg.Text == "⚙️ Settings" {
		handleSettings(bot, chatID)
		return
	}

	sessMu.RLock()
	session, exists := sessions[chatID]
	sessMu.RUnlock()

	if exists {
		if strings.HasPrefix(session.State, "admin_") {
			handleAdminInput(bot, msg)
			return
		}
		if session.State == "awaiting_winrate" {
			handleWinrateInput(bot, msg)
		} else if session.State == "awaiting_pnl" {
			handlePnlInput(bot, msg)
		} else if session.State == "awaiting_winrate_v2" {
			handleWinrateInputV2(bot, msg)
		} else if session.State == "awaiting_pnl_v2" {
			handlePnlInputV2(bot, msg)
		} else if session.State == "awaiting_wallet_address" {
			handleWalletAddressInput(bot, msg)
		} else if session.State == "awaiting_wallet_name" {
			handleWalletNameInput(bot, msg)
		} else if session.State == "awaiting_wallet_password" {
			handleWalletPassword(bot, msg)
		} else if session.State == "awaiting_buy_token" {
			handleBuyTokenInput(bot, msg)
		} else if session.State == "awaiting_buy_amount" {
			handleBuyAmountInput(bot, msg)
		} else if session.State == "awaiting_buy_password" {
			handleBuyPassword(bot, msg)
		} else if session.State == "awaiting_sell_password" {
			handleSellPassword(bot, msg)
		} else if session.State == "awaiting_copy_target" {
			handleCopyTargetInput(bot, msg)
		} else if session.State == "awaiting_copy_amount" {
			handleCopyAmountInput(bot, msg)
		}
	}
}

// handleWalletAddressInput processes wallet address input
func handleWalletAddressInput(bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	chatID := msg.Chat.ID
	address := strings.TrimSpace(msg.Text)

	// Validate Solana address
	_, err := solana.PublicKeyFromBase58(address)
	if err != nil {
		sendError(bot, chatID, "Invalid Solana address. Please try again:")
		return
	}

	// Check if wallet already exists
	wallets, _ := scanner.db.GetUserWallets(chatID)
	for _, w := range wallets {
		if w.WalletAddress == address {
			sendWarning(bot, chatID, "This wallet is already added!")
			sessMu.Lock()
			delete(sessions, chatID)
			sessMu.Unlock()
			return
		}
	}

	// Store address and ask for name
	sessMu.Lock()
	sessions[chatID].State = "awaiting_wallet_name"
	// sessions[chatID].Winrate = 0 // Reuse this field to store wallet address temporarily - this line is problematic, using tempWalletAddr instead
	sessMu.Unlock()

	// Store address in session (we'll use a temp variable)
	tempWalletAddr[chatID] = address

	send(bot, chatID, "✅ Valid address!\n\nNow give this wallet a name (e.g., 'Main Wallet', 'Trading'):")
}

// handleWalletNameInput processes wallet name input
func handleWalletNameInput(bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	chatID := msg.Chat.ID
	name := strings.TrimSpace(msg.Text)

	if len(name) > 50 {
		sendError(bot, chatID, "Name too long (max 50 characters). Please try again:")
		return
	}

	// Get stored address
	address, ok := tempWalletAddr[chatID]
	if !ok {
		sendError(bot, chatID, "Session expired. Please start again with /wallets")
		sessMu.Lock()
		delete(sessions, chatID)
		sessMu.Unlock()
		return
	}

	// Add wallet to database
	err := scanner.db.AddUserWallet(chatID, address, name)
	if err != nil {
		sendError(bot, chatID, fmt.Sprintf("Error adding wallet: %v", err))
		sessMu.Lock()
		delete(sessions, chatID)
		sessMu.Unlock()
		delete(tempWalletAddr, chatID)
		return
	}

	// Set as active if this is the first wallet
	wallets, _ := scanner.db.GetUserWallets(chatID)
	if len(wallets) == 1 {
		scanner.db.SetActiveWallet(chatID, address)
	}

	// Cleanup
	sessMu.Lock()
	delete(sessions, chatID)
	sessMu.Unlock()
	delete(tempWalletAddr, chatID)

	send(bot, chatID, fmt.Sprintf("✅ Wallet added successfully!\n\n*%s*\n`%s`", name, address))
	handleWalletsCommand(bot, chatID)
}

func handleCallback(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery) {
	bot.Request(tgbotapi.NewCallback(callback.ID, ""))
	chatID := callback.Message.Chat.ID
	data := callback.Data

	if data == "show_scan_options" {
		showScanTypeModal(bot, chatID)
	} else if data == "btn_trial_credits" {
		handleTrialSelection(bot, chatID, "credits_1000")
	} else if data == "btn_trial_time" {
		handleTrialSelection(bot, chatID, "trial_3day")
	} else if data == "back_to_menu" {
		showMainMenu(bot, chatID)
	} else if data == "scan_realtime" {
		startDevFinderImprovedWithType(bot, chatID, "realtime")
	} else if data == "scan_slow" {
		startDevFinderImprovedWithType(bot, chatID, "slow")
	} else if data == "dev_finder" {
		startDevFinder(bot, chatID)
	} else if data == "dev_finder_v2" {
		startDevFinderImproved(bot, chatID)
	} else if strings.HasPrefix(data, "cancel_search_") {
		handleCancelSearch(bot, callback)
		return
	} else if strings.HasPrefix(data, "confirm_cancel_") {
		handleConfirmCancel(bot, callback)
		return
	} else if strings.HasPrefix(data, "continue_search_") {
		handleContinueSearch(bot, callback)
		return
	} else if data == "check_balance" || data == "refresh_balance" {
		handleBalanceCommand(bot, chatID)
	} else if data == "manage_wallets" {
		handleWalletsCommand(bot, chatID)
	} else if data == "add_wallet" {
		// Show options: view-only or generate/import
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("👀 View-Only Wallet", "add_viewonly"),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🆕 Generate New Wallet", "generate_wallet"),
				tgbotapi.NewInlineKeyboardButtonData("📥 Import Wallet", "import_wallet"),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🔙 Back", "manage_wallets"),
			),
		)
		msg := tgbotapi.NewMessage(chatID, "👛 *Add Wallet*\n\nChoose how you'd like to add a wallet:")
		msg.ParseMode = "Markdown"
		msg.ReplyMarkup = keyboard
		bot.Send(msg)
	} else if data == "add_viewonly" {
		handleAddWalletStart(bot, chatID)
	} else if data == "generate_wallet" {
		handleGenerateWallet(bot, chatID)
	} else if data == "confirm_generate" {
		handleConfirmGenerate(bot, chatID)
	} else if data == "import_wallet" {
		handleImportWallet(bot, chatID)
	} else if data == "cancel_wallet" {
		sendError(bot, chatID, "Wallet operation cancelled")
		handleWalletsCommand(bot, chatID)
	} else if data == "remove_wallet" {
		handleRemoveWalletStart(bot, chatID)
	} else if strings.HasPrefix(data, "select_wallet:") {
		walletAddr := strings.TrimPrefix(data, "select_wallet:")
		handleSelectWallet(bot, chatID, walletAddr)
	} else if strings.HasPrefix(data, "confirm_remove:") {
		walletAddr := strings.TrimPrefix(data, "confirm_remove:")
		handleConfirmRemove(bot, chatID, walletAddr)
	} else if data == "start_buy" {
		handleStartBuy(bot, chatID)
	} else if data == "start_sell" {
		handleStartSell(bot, chatID)
	} else if data == "confirm_buy" {
		handleConfirmBuy(bot, chatID)
	} else if data == "cancel_buy" {
		sendError(bot, chatID, "Purchase cancelled")
		cleanupBuySession(chatID)
	} else if data == "open_settings" {
		handleSettings(bot, chatID)
	} else if data == "settings_slippage" {
		handleSettingsSlippage(bot, chatID)
	} else if data == "settings_jito" {
		handleSettingsJito(bot, chatID)
	} else if data == "settings_priority" {
		handleSettingsPriority(bot, chatID)
	} else if strings.HasPrefix(data, "set_slip_") {
		bps := parseSlippageCallback(data)
		handleSetSlippage(bot, chatID, bps)
	} else if strings.HasPrefix(data, "set_jito_") {
		lamports := parseJitoCallback(data)
		handleSetJito(bot, chatID, lamports)
	} else if strings.HasPrefix(data, "set_prio_") {
		lamports := parsePriorityCallback(data)
		handleSetPriority(bot, chatID, lamports)
	} else if data == "settings_copytrade" {
		handleSettingsCopyTrade(bot, chatID)
	} else if data == "toggle_copy_autobuy_on" {
		handleToggleCopyTradeAutoBuy(bot, chatID, true)
	} else if data == "toggle_copy_autobuy_off" {
		handleToggleCopyTradeAutoBuy(bot, chatID, false)
	} else if strings.HasPrefix(data, "sell_token:") {
		tokenMint := strings.TrimPrefix(data, "sell_token:")
		handleSellToken(bot, chatID, tokenMint)
	} else if strings.HasPrefix(data, "sell_pct:") {
		parts := strings.Split(data, ":")
		if len(parts) == 3 {
			tokenMint := parts[1]
			pct, _ := strconv.Atoi(parts[2])
			handleSellPercentage(bot, chatID, tokenMint, pct)
		}
	} else if data == "confirm_sell" {
		handleConfirmSell(bot, chatID)
	} else if data == "back_to_menu" {
		showMainMenu(bot, chatID)
	} else if strings.HasPrefix(data, "admin_") {
		handleAdminCallback(bot, callback)
	} else if data == "top_up_credits" {
		send(bot, chatID, "💎 *Top Up Credits*\n\nTo purchase more credits, please contact the admin:\n@AdminUser\n\nPackages:\n• 100 Credits: 0.1 SOL\n• 500 Credits: 0.4 SOL\n• 1000 Credits: 0.7 SOL")
	} else if data == "help" {
		send(bot, chatID, "📚 *Help & FAQ*\n\n*Credits*: 1 Credit is deducted for every wallet processed during a scan.\n*Dev Finder*: Scans Solana for profitable wallets based on your Win Rate and PnL filters.\n\nNeed more help? Contact support.")
	} else if data == "copytrade" {
		handleCopyTradeCommand(bot, chatID)
	} else if data == "copy_add_target" {
		handleAddCopyTargetStart(bot, chatID)
	} else if data == "copy_list_targets" {
		handleListCopyTargets(bot, chatID)
	} else if strings.HasPrefix(data, "stop_copy:") {
		target := strings.TrimPrefix(data, "stop_copy:")
		handleStopCopyTarget(bot, chatID, target)
	}
}

func showTrialOptions(bot *tgbotapi.BotAPI, chatID int64) {
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💎 Get 1000 Credits", "btn_trial_credits"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⏳ 3-Day Free Trial", "btn_trial_time"),
		),
	)

	text := "👋 *Welcome to Solana Wallet Scanner!*\n\n" +
		"To get started, please choose your *Free Trial* plan:\n\n" +
		"💎 *1000 Credits*\n" +
		"• 1 Credit = 1 Wallet Scan\n" +
		"• Use for Real-Time or Slow scans\n" +
		"• No time limit\n\n" +
		"⏳ *3-Day Free Trial*\n" +
		"• Unlimited Scans\n" +
		"• *Note:* Results delayed by 5-10 minutes\n" +
		"• Expires in 3 days\n\n" +
		"Select an option below:"

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}

func handleTrialSelection(bot *tgbotapi.BotAPI, chatID int64, planType string) {
	var text string
	if planType == "credits_1000" {
		err := scanner.db.SetUserPlan(chatID, "credits_1000", 1000, 0)
		if err != nil {
			log.Printf("Error setting plan: %v", err)
			sendError(bot, chatID, "Error setting plan. Please try again.")
			return
		}
		text = "✅ *Plan Activated: 1000 Credits*\n\n" +
			"You have 1000 credits. Each scan costs 1 credit.\n" +
			"You can use both Real-Time and Slow scans."
	} else {
		expiresAt := time.Now().Add(3 * 24 * time.Hour).Unix()
		err := scanner.db.SetUserPlan(chatID, "trial_3day", 0, expiresAt)
		if err != nil {
			log.Printf("Error setting plan: %v", err)
			sendError(bot, chatID, "Error setting plan. Please try again.")
			return
		}
		text = "✅ *Plan Activated: 3-Day Free Trial*\n\n" +
			"You have unlimited scans for 3 days.\n" +
			"⚠️ *Note:* All your scans will have a 5-10 minute delay."
	}

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	bot.Send(msg)

	// Show main menu after short delay
	time.Sleep(1 * time.Second)
	showMainMenu(bot, chatID)
}

func showMainMenu(bot *tgbotapi.BotAPI, chatID int64) {
	// Modern button layout
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔍 Dev Finder", "dev_finder"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📊 My Balance", "check_balance"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💹 Buy Token", "start_buy"),
			tgbotapi.NewInlineKeyboardButtonData("💸 Sell Token", "start_sell"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🤖 Copy Trading", "copytrade"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⚙️ Settings", "open_settings"),
			tgbotapi.NewInlineKeyboardButtonData("❓ Help", "help"),
		),
	)

	// Get User Plan for Header
	user, _ := scanner.db.GetUser(chatID)
	planBadge := ""
	if user != nil {
		if user.PlanType == "credits_1000" {
			planBadge = fmt.Sprintf("\n💎 *%d Credits Available*", user.Credits)
		} else if user.PlanType == "trial_3day" {
			timeLeft := time.Until(time.Unix(user.TrialExpiresAt, 0))
			days := int(timeLeft.Hours() / 24)
			hours := int(timeLeft.Hours()) % 24
			planBadge = fmt.Sprintf("\n⏰ *Free Trial: %dd %dh Left*", days, hours)
		}
	}

	text := "╔═══════════════════════╗\n"
	text += "    🚀 *SOLANA TRADING BOT*\n"
	text += "╚═══════════════════════╝"
	if planBadge != "" {
		text += planBadge
	}
	text += "\n\n📋 *Main Dashboard*\n"
	text += "━━━━━━━━━━━━━━━━━━━━\n"
	text += "Select an action from the menu below ⬇️"

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}

// handleAddWalletStart starts the add wallet flow
func handleAddWalletStart(bot *tgbotapi.BotAPI, chatID int64) {
	sessMu.Lock()
	sessions[chatID] = &UserSession{
		State:       "awaiting_wallet_address",
		RequestedAt: time.Now().Unix(),
	}
	sessMu.Unlock()

	send(bot, chatID, "👛 *Add Wallet*\n\nPlease send me a Solana wallet address (view-only):")
}

// handleRemoveWalletStart shows wallets to remove
func handleRemoveWalletStart(bot *tgbotapi.BotAPI, chatID int64) {
	wallets, err := scanner.db.GetUserWallets(chatID)
	if err != nil || len(wallets) == 0 {
		send(bot, chatID, "⚠️ No wallets to remove")
		return
	}

	var buttons [][]tgbotapi.InlineKeyboardButton
	for _, wallet := range wallets {
		name := wallet.WalletName
		if name == "" {
			name = "Unnamed Wallet"
		}

		buttons = append(buttons, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("🗑 %s", name),
				fmt.Sprintf("confirm_remove:%s", wallet.WalletAddress),
			),
		))
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(buttons...)
	msg := tgbotapi.NewMessage(chatID, "🗑 *Remove Wallet*\n\nSelect a wallet to remove:")
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}

// handleSelectWallet sets a wallet as active
func handleSelectWallet(bot *tgbotapi.BotAPI, chatID int64, walletAddr string) {
	err := scanner.db.SetActiveWallet(chatID, walletAddr)
	if err != nil {
		sendError(bot, chatID, "Error setting active wallet")
		return
	}

	send(bot, chatID, fmt.Sprintf("✅ Wallet activated!\n\n`%s`", walletAddr))
	handleWalletsCommand(bot, chatID)
}

// handleConfirmRemove removes a wallet
func handleConfirmRemove(bot *tgbotapi.BotAPI, chatID int64, walletAddr string) {
	err := scanner.db.RemoveUserWallet(chatID, walletAddr)
	if err != nil {
		sendError(bot, chatID, "Error removing wallet")
		return
	}

	send(bot, chatID, "✅ Wallet removed successfully")
	handleWalletsCommand(bot, chatID)
}

func showScanTypeModal(bot *tgbotapi.BotAPI, chatID int64) {
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⚡ Real-Time Scan", "scan_realtime"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🕐 Slow Scan", "scan_slow"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("« Back", "back_to_menu"),
		),
	)

	text := "🎯 *Choose Your Scan Type*\n\n" +
		"━━━━━━━━━━━━━━━━━━━━\n\n" +
		"⚡ *Real-Time Scan*\n" +
		"• Instant, high-priority scanning\n" +
		"• Results appear immediately as found\n" +
		"• Standard usage cost: *1x*\n" +
		"• Perfect for time-sensitive opportunities\n\n" +
		"━━━━━━━━━━━━━━━━━━━━\n\n" +
		"🕐 *Slow Scan*\n" +
		"• Lower priority, delayed results\n" +
		"• Results delivered in 5-60 minutes\n" +
		"• Reduced usage cost: *0.5x (50% discount)*\n" +
		"• Ideal for patient users saving credits\n\n" +
		"━━━━━━━━━━━━━━━━━━━━\n\n" +
		"💡 *Tip:* Use Real-Time for urgent scans, Slow for overnight or casual research.\n\n" +
		"Select your preferred scan type below:"

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}

func startDevFinder(bot *tgbotapi.BotAPI, chatID int64) {
	sessMu.Lock()
	scanner.mu.RLock()
	sessions[chatID] = &UserSession{
		State:       "awaiting_winrate",
		RequestedAt: time.Now().Unix(),
		StartCount:  len(scanner.walletsCache),
	}
	scanner.mu.RUnlock()
	sessMu.Unlock()

	text := "🎯 *Dev Finder*\n\n" +
		"Enter minimum *Win Rate* (25-100):"
	send(bot, chatID, text)
}

func handleWinrateInput(bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	chatID := msg.Chat.ID
	winrate, err := strconv.ParseFloat(msg.Text, 64)

	if err != nil || winrate < 25 || winrate > 100 {
		sendError(bot, chatID, "Invalid input. Enter 25-100")
		return
	}

	sessMu.Lock()
	sessions[chatID].State = "awaiting_pnl"
	sessions[chatID].Winrate = winrate
	sessMu.Unlock()

	send(bot, chatID, "✅ Enter minimum *PnL* (e.g. 100):")
}

func handlePnlInput(bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	chatID := msg.Chat.ID
	pnl, err := strconv.ParseFloat(msg.Text, 64)

	if err != nil || pnl < 25 {
		sendError(bot, chatID, "Invalid input.")
		return
	}

	sessMu.Lock()
	session := sessions[chatID]
	winrate := session.Winrate
	startCount := session.StartCount
	delete(sessions, chatID)
	sessMu.Unlock()

	searchAndRespond(bot, chatID, winrate, pnl, startCount)
}

func searchAndRespond(bot *tgbotapi.BotAPI, chatID int64, winrate, pnl float64, startCount int) {
	scanner.mu.RLock()
	var matches []*storage.WalletData
	for _, w := range scanner.walletsCache {
		if w.Winrate >= winrate && w.RealizedPnL >= pnl {
			matches = append(matches, w)
		}
	}
	totalScanned := len(scanner.walletsCache)
	isScanning := scanner.isScanning
	totalWallets := scanner.totalWallets
	scanner.mu.RUnlock()

	if len(matches) == 0 {
		msg := sendWaitingMessage(bot, chatID, winrate, pnl, startCount, totalScanned, totalWallets, isScanning)
		go updateProgress(bot, chatID, msg.MessageID, winrate, pnl, startCount)
		return
	}

	sendResults(bot, chatID, matches, winrate, pnl)
}

func updateProgress(bot *tgbotapi.BotAPI, chatID int64, messageID int, winrate, pnl float64, startCount int) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for i := 0; i < 30; i++ {
		<-ticker.C

		scanner.mu.RLock()
		var matches []*storage.WalletData
		for _, w := range scanner.walletsCache {
			if w.Winrate >= winrate && w.RealizedPnL >= pnl {
				matches = append(matches, w)
			}
		}
		totalScanned := len(scanner.walletsCache)
		isScanning := scanner.isScanning
		totalWallets := scanner.totalWallets
		scanner.mu.RUnlock()

		if len(matches) > 0 {
			sendResults(bot, chatID, matches, winrate, pnl)
			return
		}

		newScanned := totalScanned - startCount
		progress := 0.0
		if totalWallets > 0 {
			progress = float64(totalScanned) / float64(totalWallets) * 100
		}

		progressBar := createProgressBar(progress)
		text := fmt.Sprintf("⏳ *Scanning in Progress*\n\n"+
			"Filters: WR ≥ %.2f%%, PnL ≥ %.2f%%\n\n"+
			"%s\n"+
			"Progress: %.1f%%\n\n"+
			"📊 Total Wallets Scanned From Tokens: %d\n"+
			"⏱️ Status: %s",
			winrate, pnl, progressBar, progress, newScanned,
			map[bool]string{true: "Scanning...", false: "Waiting for next cycle"}[isScanning])

		edit := tgbotapi.NewEditMessageText(chatID, messageID, text)
		edit.ParseMode = "Markdown"
		bot.Send(edit)
	}
}

func sendWaitingMessage(bot *tgbotapi.BotAPI, chatID int64, winrate, pnl float64, startCount, totalScanned, totalWallets int, isScanning bool) tgbotapi.Message {
	newScanned := totalScanned - startCount
	progress := 0.0
	if totalWallets > 0 {
		progress = float64(totalScanned) / float64(totalWallets) * 100
	}

	progressBar := createProgressBar(progress)

	text := fmt.Sprintf("⏳ *Scanning in Progress*\n\n"+
		"Filters: WR ≥ %.2f%%, PnL ≥ %.2f%%\n\n"+
		"%s\n"+
		"Progress: %.1f%%\n\n"+
		"📊 Total Wallets Scanned From Tokens: %d\n"+
		"⏱️ Status: %s",
		winrate, pnl, progressBar, progress, newScanned,
		map[bool]string{true: "Scanning...", false: "Waiting for next cycle"}[isScanning])

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	sentMsg, _ := bot.Send(msg)
	return sentMsg
}

func sendResults(bot *tgbotapi.BotAPI, chatID int64, matches []*storage.WalletData, winrate, pnl float64) {
	text := fmt.Sprintf("✅ *Found %d Wallets!*\n\n"+
		"Filters: WR ≥ %.2f%%, PnL ≥ %.2f%%\n\n",
		len(matches), winrate, pnl)

	for i, w := range matches {
		if i >= 15 {
			text += fmt.Sprintf("\n_... and %d more_", len(matches)-15)
			break
		}
		text += fmt.Sprintf("`%s`\n💹 WR: %.2f%% | 💰 PnL: %.2f%%\n\n",
			w.Wallet, w.Winrate, w.RealizedPnL)
	}

	send(bot, chatID, text)
}

func createProgressBar(percent float64) string {
	filled := int(percent / 5)
	if filled > 20 {
		filled = 20
	}
	empty := 20 - filled

	bar := ""
	for i := 0; i < filled; i++ {
		bar += "█"
	}
	for i := 0; i < empty; i++ {
		bar += "░"
	}
	return bar
}

func sendStatus(bot *tgbotapi.BotAPI, chatID int64) {
	scanner.mu.RLock()
	count := len(scanner.walletsCache)
	scanned := scanner.scannedCount
	lastScan := scanner.lastScanStart
	isScanning := scanner.isScanning
	scanner.mu.RUnlock()

	elapsed := time.Since(time.Unix(lastScan, 0))
	status := map[bool]string{true: "🟢 Scanning", false: "🟡 Idle"}[isScanning]

	text := fmt.Sprintf("📊 *Scanner Status*\n\n"+
		"Status: %s\n"+
		"Total wallets: %d\n"+
		"Last scan: %d wallets\n"+
		"Time since scan: %s",
		status, count, scanned, elapsed.Round(time.Minute))
	send(bot, chatID, text)
}

func send(bot *tgbotapi.BotAPI, chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	bot.Send(msg)
}

func sendWithKeyboard(bot *tgbotapi.BotAPI, chatID int64, text string, keyboard tgbotapi.InlineKeyboardMarkup) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}

func sendError(bot *tgbotapi.BotAPI, chatID int64, text string) {
	send(bot, chatID, "❌ "+text)
}

func sendWarning(bot *tgbotapi.BotAPI, chatID int64, text string) {
	send(bot, chatID, "⚠️ "+text)
}

// publishScanProgress publishes scan progress to Redis for monitor TUI
func publishScanProgress(scanned, total int, isScanning bool, foundWallets int) {
	if redisClient == nil {
		return
	}

	scanner.mu.RLock()
	scanStartTime := scanner.lastScanStart
	scanner.mu.RUnlock()

	progress := map[string]interface{}{
		"is_scanning":     isScanning,
		"scanned_count":   scanned,
		"total_wallets":   total,
		"last_update":     time.Now().Unix(),
		"scan_start_time": scanStartTime,
		"found_wallets":   foundWallets,
	}

	data, err := json.Marshal(progress)
	if err != nil {
		return
	}

	// Publish to Redis with 1 hour expiry
	redisClient.Set(context.Background(), "scan:progress", string(data), 1*time.Hour)
}

func sendInfo(bot *tgbotapi.BotAPI, chatID int64, message string) {
	send(bot, chatID, fmt.Sprintf("ℹ️ *Info*\\n\\n%s", message))
}
