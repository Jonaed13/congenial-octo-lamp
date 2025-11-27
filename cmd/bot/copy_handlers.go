package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gagliardetto/solana-go"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// handleCopyTradeCommand shows the copy trade menu
func handleCopyTradeCommand(bot *tgbotapi.BotAPI, chatID int64) {
	// Check if user has encrypted wallet (required for trading)
	if !scanner.db.HasEncryptedWallet(chatID) {
		msg := "⚠️ *Wallet Required*\n\n"
		msg += "Copy trading requires an encrypted wallet for automatic execution.\n\n"
		msg += "📝 Use `/wallets` to create or import one."
		send(bot, chatID, msg)
		return
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✚ Add Target Wallet", "copy_add_target"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📋 View My Targets", "copy_list_targets"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📊 Trade History", "copy_history"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Back to Menu", "back_to_menu"),
		),
	)

	text := "╔═══════════════════════╗\n"
	text += "     🤖 *COPY TRADING*\n"
	text += "╚═══════════════════════╝\n\n"
	text += "💡 *What is Copy Trading?*\n"
	text += "Automatically mirror trades from successful wallets in real-time.\n\n"
	text += "━━━━━━━━━━━━━━━━━━━━\n"
	text += "✓ Monitor profitable wallets\n"
	text += "✓ Auto-copy their buy orders\n"
	text += "✓ Set custom SOL amounts\n"
	text += "━━━━━━━━━━━━━━━━━━━━"

	text += "━━━━━━━━━━━━━━━━━━━━\n"
	if fanoutEngine != nil && fanoutEngine.IsRunning() {
		text += "🟢 *Engine Status*: Active\n"
		text += fmt.Sprintf("📡 *Monitoring*: %d wallets\n", fanoutEngine.GetMonitoredCount())
	} else {
		text += "🔴 *Engine Status*: Offline\n"
	}

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}

// handleAddCopyTargetStart starts the flow to add a target
func handleAddCopyTargetStart(bot *tgbotapi.BotAPI, chatID int64) {
	sessMu.Lock()
	sessions[chatID] = &UserSession{
		State:       "awaiting_copy_target",
		RequestedAt: time.Now().Unix(),
	}
	sessMu.Unlock()

	text := "╔═══════════════════════╗\n"
	text += "     🎯 *ADD TARGET*\n"
	text += "╚═══════════════════════╝\n\n"
	text += "📝 Enter the Solana wallet address you want to copy:\n\n"
	text += "_Example: 7xKXtg2...BPUm_"
	send(bot, chatID, text)
}

// handleCopyTargetInput processes the target wallet address
func handleCopyTargetInput(bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	chatID := msg.Chat.ID
	targetAddr := strings.TrimSpace(msg.Text)

	// Validate address
	_, err := solana.PublicKeyFromBase58(targetAddr)
	if err != nil {
		send(bot, chatID, "❌ Invalid address. Please try again:")
		return
	}

	// Check if already copying
	targets, _ := scanner.db.GetCopyTargets(chatID)
	for _, t := range targets {
		if t.TargetWallet == targetAddr {
			send(bot, chatID, "⚠️ You are already copying this wallet!")
			return
		}
	}

	// Store target temporarily
	sessMu.Lock()
	sessions[chatID].State = "awaiting_copy_amount"
	// Use TempData to store target address
	if sessions[chatID].TempData == nil {
		sessions[chatID].TempData = make(map[string]interface{})
	}
	sessions[chatID].TempData["target_wallet"] = targetAddr
	sessMu.Unlock()

	send(bot, chatID, "💰 *Copy Amount*\n\nEnter the amount of SOL to buy per trade (e.g., 0.1):")
}

// handleCopyAmountInput processes the copy amount
func handleCopyAmountInput(bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	chatID := msg.Chat.ID
	amountStr := strings.TrimSpace(msg.Text)
	amount, err := strconv.ParseFloat(amountStr, 64)

	if err != nil || amount <= 0 {
		send(bot, chatID, "❌ Invalid amount. Please enter a number (e.g., 0.1):")
		return
	}

	sessMu.Lock()
	session := sessions[chatID]
	targetWallet, ok := session.TempData["target_wallet"].(string)
	delete(sessions, chatID) // Clear session
	sessMu.Unlock()

	if !ok {
		send(bot, chatID, "❌ Session error. Please start over.")
		return
	}

	// Save to DB
	err = scanner.db.AddCopyTarget(chatID, targetWallet, amount)
	if err != nil {
		send(bot, chatID, fmt.Sprintf("❌ Database error: %v", err))
		return
	}

	// Notify Fan-Out Engine to update Redis
	if fanoutEngine != nil {
		if err := fanoutEngine.SyncMonitoredWallets(); err != nil {
			log.Printf("Warning: Failed to sync wallets to Redis: %v", err)
		}
	}

	send(bot, chatID, fmt.Sprintf("✅ *Target Added Successfully!*\n\n━━━━━━━━━━━━━━━━━━━━\n🎯 *Wallet*\n`%s`\n\n💰 *Amount per Trade*\n`%.2f SOL`\n━━━━━━━━━━━━━━━━━━━━\n\n🔔 I'm now monitoring this wallet in real-time!", targetWallet, amount))
}

// handleListCopyTargets shows active targets
func handleListCopyTargets(bot *tgbotapi.BotAPI, chatID int64) {
	targets, err := scanner.db.GetCopyTargets(chatID)
	if err != nil {
		send(bot, chatID, "❌ Error fetching targets")
		return
	}

	if len(targets) == 0 {
		text := "📋 *Your Copy Targets*\n\n"
		text += "━━━━━━━━━━━━━━━━━━━━\n"
		text += "No active targets yet.\n\n"
		text += "💡 Add a wallet to start copy trading!"
		send(bot, chatID, text)
		return
	}

	msg := "╔═══════════════════════╗\n"
	msg += "    📋 *YOUR TARGETS*\n"
	msg += "╚═══════════════════════╝\n\n"
	var buttons [][]tgbotapi.InlineKeyboardButton

	for i, t := range targets {
		shortAddr := t.TargetWallet[:4] + "..." + t.TargetWallet[len(t.TargetWallet)-4:]
		msg += fmt.Sprintf("━━━━━━━━━━━━━━━━━━━━\n")
		msg += fmt.Sprintf("*Target #%d*\n", i+1)
		msg += fmt.Sprintf("▫️ Wallet: `%s`\n", t.TargetWallet)
		msg += fmt.Sprintf("▫️ Amount: `%.2f SOL`\n", t.CopyAmountSOL)

		buttons = append(buttons, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("🛑 Stop %s", shortAddr), fmt.Sprintf("stop_copy:%s", t.TargetWallet)),
		))
	}

	msg += "━━━━━━━━━━━━━━━━━━━━\n"

	buttons = append(buttons, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("➕ Add New Target", "copy_add_target"),
	))
	buttons = append(buttons, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔙 Back", "back_to_menu"),
	))

	keyboard := tgbotapi.NewInlineKeyboardMarkup(buttons...)
	reply := tgbotapi.NewMessage(chatID, msg)
	reply.ParseMode = "Markdown"
	reply.ReplyMarkup = keyboard
	bot.Send(reply)
}

// handleStopCopyTarget removes a target
func handleStopCopyTarget(bot *tgbotapi.BotAPI, chatID int64, targetWallet string) {
	err := scanner.db.RemoveCopyTarget(chatID, targetWallet)
	if err != nil {
		send(bot, chatID, "❌ Error removing target")
		return
	}

	if fanoutEngine != nil {
		if err := fanoutEngine.SyncMonitoredWallets(); err != nil {
			log.Printf("Warning: Failed to sync wallets to Redis: %v", err)
		}
	}

	send(bot, chatID, fmt.Sprintf("🛑 Stopped copying `%s`", targetWallet))
	send(bot, chatID, fmt.Sprintf("🛑 Stopped copying `%s`", targetWallet))
	handleListCopyTargets(bot, chatID) // Refresh list
}

// handleCopyTradeHistory shows recent copy trades
func handleCopyTradeHistory(bot *tgbotapi.BotAPI, chatID int64) {
	trades, err := scanner.db.GetRecentTrades(chatID, 10)
	if err != nil {
		send(bot, chatID, "❌ Error fetching trade history")
		return
	}

	if len(trades) == 0 {
		send(bot, chatID, "📊 No copy trades found yet.")
		return
	}

	msg := "📊 *Recent Copy Trades*\n\n"
	for i, t := range trades {
		statusIcon := "⏳"
		if t.Status == "confirmed" {
			statusIcon = "✅"
		} else if t.Status == "failed" {
			statusIcon = "❌"
		}

		msg += fmt.Sprintf("━━━━━━━━━━━━━━━━━━━━\n")
		msg += fmt.Sprintf("*Trade #%d*\n", i+1)
		msg += fmt.Sprintf("▫️ Type: %s\n", strings.ToUpper(t.TradeType))
		msg += fmt.Sprintf("▫️ Token: `%s`\n", t.TokenAddress)
		msg += fmt.Sprintf("▫️ Amount: %.2f SOL\n", t.SolAmount)
		msg += fmt.Sprintf("▫️ Status: %s %s\n", statusIcon, strings.Title(t.Status))
		if t.TxSignature != "" {
			msg += fmt.Sprintf("▫️ Signature: `%s`\n", t.TxSignature[:8]+"...")
		}
	}
	msg += "━━━━━━━━━━━━━━━━━━━━"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Back", "copytrade"),
		),
	)

	reply := tgbotapi.NewMessage(chatID, msg)
	reply.ParseMode = "Markdown"
	reply.ReplyMarkup = keyboard
	bot.Send(reply)
}
