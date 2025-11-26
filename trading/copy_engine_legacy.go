//go:build ignore

package trading

// DEPRECATED: This file is replaced by engine/fanout.go
// The new Fan-Out architecture uses a single WebSocket connection
// and Redis for O(1) lookups, scaling to thousands of users.
// This file is kept for reference only.

import (
	"context"
	"fmt"
	"log"
	"solana-orchestrator/storage"
	"sync"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// CopyTradeEngine manages copy trading logic
type CopyTradeEngine struct {
	db          *storage.DB
	bot         *tgbotapi.BotAPI
	rpcClient   *rpc.Client
	wsClient    *WSClient
	activeSubs  map[string]context.CancelFunc // Map of TargetWallet -> CancelFunc
	mu          sync.RWMutex
	shyftApiKey string
}

// NewCopyTradeEngine creates a new engine
func NewCopyTradeEngine(db *storage.DB, bot *tgbotapi.BotAPI, shyftApiKey string) *CopyTradeEngine {
	rpcURL := fmt.Sprintf("https://rpc.shyft.to?api_key=%s", shyftApiKey)
	wsURL := fmt.Sprintf("wss://rpc.shyft.to?api_key=%s", shyftApiKey)

	return &CopyTradeEngine{
		db:          db,
		bot:         bot,
		rpcClient:   rpc.New(rpcURL),
		wsClient:    NewWSClient(wsURL),
		activeSubs:  make(map[string]context.CancelFunc),
		shyftApiKey: shyftApiKey,
	}
}

// Start starts the engine
func (e *CopyTradeEngine) Start() {
	log.Println("🚀 Starting Copy Trade Engine...")
	e.ReloadTargets()
}

// ReloadTargets refreshes subscriptions based on DB
func (e *CopyTradeEngine) ReloadTargets() {
	e.mu.Lock()
	defer e.mu.Unlock()

	targets, err := e.db.GetAllActiveCopyTargets()
	if err != nil {
		log.Printf("❌ Error fetching targets: %v", err)
		return
	}

	// Identify needed subscriptions
	needed := make(map[string]bool)
	for _, t := range targets {
		needed[t.TargetWallet] = true
	}

	// Remove old subscriptions
	for wallet, cancel := range e.activeSubs {
		if !needed[wallet] {
			log.Printf("🛑 Unsubscribing from %s", wallet)
			cancel()
			delete(e.activeSubs, wallet)
		}
	}

	// Add new subscriptions
	for wallet := range needed {
		if _, exists := e.activeSubs[wallet]; !exists {
			log.Printf("👀 Subscribing to %s", wallet)
			ctx, cancel := context.WithCancel(context.Background())
			e.activeSubs[wallet] = cancel
			go e.monitorWallet(ctx, wallet)
		}
	}
}

// monitorWallet listens for transactions on a target wallet
func (e *CopyTradeEngine) monitorWallet(ctx context.Context, targetWallet string) {
	// Subscribe to logs for the wallet
	// Note: Shyft/Solana WS logsSubscribe filters by mention
	sub, err := e.wsClient.SubscribeLogs(ctx, targetWallet)
	if err != nil {
		log.Printf("❌ Failed to subscribe to %s: %v", targetWallet, err)
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case logResult := <-sub:
			// Process log result
			// logResult is map[string]interface{} from WSClient
			data, ok := logResult.(map[string]interface{})
			if !ok {
				continue
			}

			// Navigate JSON structure: params -> result -> value -> signature
			// Structure depends on RPC provider, but typically:
			// { "method": "logsNotification", "params": { "result": { "value": { "signature": "..." } } } }

			params, ok := data["params"].(map[string]interface{})
			if !ok {
				continue
			}

			result, ok := params["result"].(map[string]interface{})
			if !ok {
				continue
			}

			value, ok := result["value"].(map[string]interface{})
			if !ok {
				continue
			}

			sig, ok := value["signature"].(string)
			if !ok {
				continue
			}

			go e.processTransaction(targetWallet, sig)
		}
	}
}

// processTransaction analyzes a transaction and executes copy trades
func (e *CopyTradeEngine) processTransaction(targetWallet, signature string) {
	// 1. Fetch Transaction
	// We need to wait a moment for it to be queryable via RPC
	time.Sleep(2 * time.Second)

	tx, err := e.rpcClient.GetTransaction(context.Background(), solana.MustSignatureFromBase58(signature), nil)
	if err != nil {
		log.Printf("❌ Error fetching tx %s: %v", signature, err)
		return
	}

	if tx == nil || tx.Meta == nil || tx.Meta.Err != nil {
		return // Failed transaction
	}

	// 2. Analyze for Swaps
	// This is complex: we need to look at pre/post token balances
	// If Target Wallet's Token Balance INCREASED -> BUY
	// If Target Wallet's Token Balance DECREASED -> SELL

	// Simplified logic: Look for Raydium/Jupiter program interaction
	// and balance changes.

	// ... (Transaction parsing logic would go here) ...
	// For this prototype, we'll log detection
	log.Printf("🔎 Detected transaction on %s: %s", targetWallet, signature)

	// Notify users copying this wallet
	targets, _ := e.db.GetAllActiveCopyTargets()
	for _, t := range targets {
		if t.TargetWallet == targetWallet {
			// Get user settings to check for auto-buy
			settings, err := e.db.GetUserSettings(t.UserID)
			if err != nil {
				log.Printf("❌ Failed to get settings for user %d: %v", t.UserID, err)
				continue
			}

			if settings.CopyTradeAutoBuy {
				// Auto-execute logic
				// 1. Get Wallet
				wallet, err := e.db.GetEncryptedWallet(t.UserID)
				if err != nil {
					log.Printf("❌ Failed to get wallet for user %d: %v", t.UserID, err)
					continue
				}

				// 2. Decrypt Private Key (We need a way to store/retrieve password or use a different mechanism)
				// CRITICAL: We cannot decrypt without the user's password which is not stored.
				// For now, we can only alert. Auto-buy requires a non-custodial solution or session-based password caching (which expires).
				// OR, we assume the user has a "trading password" stored securely or we use a hot wallet approach (less secure).

				// Given the current architecture, we can't auto-sign without the password.
				// We will send a "Click to Buy" button instead which prompts for password.

				keyboard := tgbotapi.NewInlineKeyboardMarkup(
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("🚀 Buy %.2f SOL", t.CopyAmountSOL), fmt.Sprintf("copy_buy:%s:%s", targetWallet, signature)),
					),
				)

				msg := tgbotapi.NewMessage(t.UserID, fmt.Sprintf("🤖 *Auto-Copy Triggered*\n\nTarget `%s` swapped!\nSignature: `%s`\n\n*Action Required:* Auto-buy requires authorization.", targetWallet, signature))
				msg.ParseMode = "Markdown"
				msg.ReplyMarkup = keyboard
				e.bot.Send(msg)

			} else {
				// Alert only
				msg := tgbotapi.NewMessage(t.UserID, fmt.Sprintf("🔔 *Copy Trade Alert*\n\nTarget `%s` executed a transaction!\nSignature: `%s`\n\n_Auto-buy is disabled. Enable it in Settings to copy automatically._", targetWallet, signature))
				msg.ParseMode = "Markdown"
				e.bot.Send(msg)
			}
		}
	}
}
