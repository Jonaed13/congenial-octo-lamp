package main

import (
	"context"
	"fmt"
	"log"
	"solana-orchestrator/api"
	"solana-orchestrator/config"
	"solana-orchestrator/trading"
	"strings"
	"time"

	"github.com/gagliardetto/solana-go"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// handleBalanceCommand checks and displays wallet balance
func handleBalanceCommand(bot *tgbotapi.BotAPI, chatID int64) {
	// Get active wallet
	activeWallet, err := scanner.db.GetActiveWallet(chatID)
	if err != nil {
		sendError(bot, chatID, "Error retrieving active wallet")
		log.Printf("Error getting active wallet: %v", err)
		return
	}

	if activeWallet == nil {
		sendWarning(bot, chatID, "No active wallet set!\n\nUse /wallets to add a wallet first.")
		return
	}

	// Get User Plan
	user, err := scanner.db.GetUser(chatID)
	if err != nil {
		log.Printf("Error getting user: %v", err)
	}

	// Show loading message
	loadingMsgConfig := tgbotapi.NewMessage(chatID, "⏳ Checking balance...")
	loadingMsgConfig.ParseMode = "Markdown"
	loadingMsg, _ := bot.Send(loadingMsgConfig)

	// Load config for RPC URL
	cfg, err := config.Load("config/config.json")
	if err != nil {
		editMessage(bot, chatID, loadingMsg.MessageID, "❌ Configuration error")
		return
	}

	// Initialize API client
	apiClient := api.NewClient(cfg.MoralisAPIKey, cfg.BirdeyeAPIKey, cfg.APISettings.MaxRetries, cfg.MoralisFallbackKeys)

	// Initialize balance manager
	balanceMgr := trading.NewBalanceManager(
		"https://rpc.shyft.to?api_key="+extractAPIKey(cfg.WebSocketSettings.ShyftWSURL),
		nil, // WS client not needed for one-off check
		apiClient,
	)

	// Get wallet public key
	walletPubkey, err := solana.PublicKeyFromBase58(activeWallet.WalletAddress)
	if err != nil {
		editMessage(bot, chatID, loadingMsg.MessageID, "❌ Invalid wallet address")
		return
	}

	// Get balance
	ctx := context.Background()
	fullBalance, err := balanceMgr.GetFullBalance(ctx, walletPubkey)
	if err != nil {
		editMessage(bot, chatID, loadingMsg.MessageID, fmt.Sprintf("❌ Error fetching balance: %v", err))
		return
	}

	// Format output with professional design
	solBalance := trading.FormatSOL(fullBalance.SOLBalance)

	message := "╔═══════════════════════╗\n"
	message += "       📊 *WALLET  BALANCE*\n"
	message += "╚═══════════════════════╝\n\n"

	message += fmt.Sprintf("💼 *Wallet Name*\n`%s`\n\n", activeWallet.WalletName)
	message += fmt.Sprintf("🏛 *Address*\n`%s`\n\n", activeWallet.WalletAddress)

	message += "━━━━━━━━━━━━━━━━━━━━\n"
	message += "💎 *BALANCE OVERVIEW*\n"
	message += "━━━━━━━━━━━━━━━━━━━━\n\n"

	message += fmt.Sprintf("▫️ *SOL:* `%.6f SOL`\n", solBalance)

	if len(fullBalance.TokenBalances) > 0 {
		message += fmt.Sprintf("▫️ *Tokens:* `%d holdings`\n", len(fullBalance.TokenBalances))
	} else {
		message += "▫️ *Tokens:* `None`\n"
	}

	// Add Plan Details
	message += "\n━━━━━━━━━━━━━━━━━━━━\n"
	message += "🎯 *ACCOUNT STATUS*\n"
	message += "━━━━━━━━━━━━━━━━━━━━\n\n"
	if user != nil {
		if user.PlanType == "credits_1000" {
			message += fmt.Sprintf("▫️ *Plan:* Credits Plan\n")
			message += fmt.Sprintf("▫️ *Credits:* `%d remaining`\n", user.Credits)
		} else if user.PlanType == "trial_3day" {
			timeLeft := time.Until(time.Unix(user.TrialExpiresAt, 0))
			days := int(timeLeft.Hours() / 24)
			hours := int(timeLeft.Hours()) % 24
			message += "▫️ *Plan:* Free Trial\n"
			message += fmt.Sprintf("▫️ *Time Left:* `%dd %dh`\n", days, hours)
		} else {
			message += "▫️ *Plan:* No active plan\n"
		}
	} else {
		message += "▫️ *Plan:* Unknown\n"
	}

	// Add Top Up Button
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💎 Top Up Credits", "top_up_credits"),
			tgbotapi.NewInlineKeyboardButtonData("🔄 Refresh", "refresh_balance"),
		),
	)

	// Use Send instead of Edit for the final message to attach keyboard if needed,
	// or edit the existing message and add markup.
	// Since loadingMsg was sent, we should edit it.

	edit := tgbotapi.NewEditMessageText(chatID, loadingMsg.MessageID, message)
	edit.ParseMode = "Markdown"
	edit.ReplyMarkup = &keyboard
	bot.Send(edit)
}

// handleWalletsCommand manages user wallets
func handleWalletsCommand(bot *tgbotapi.BotAPI, chatID int64) {
	// Self-healing: Check for orphaned encrypted wallet
	if scanner.db.HasEncryptedWallet(chatID) {
		encWallet, err := scanner.db.GetEncryptedWallet(chatID)
		if err == nil && encWallet != nil {
			// Check if it exists in user_wallets
			wallets, _ := scanner.db.GetUserWallets(chatID)
			found := false
			for _, w := range wallets {
				if w.WalletAddress == encWallet.PublicKey {
					found = true
					break
				}
			}

			// If not found, restore it
			if !found {
				log.Printf("🔧 Restoring orphaned wallet for user %d", chatID)
				scanner.db.AddUserWallet(chatID, encWallet.PublicKey, "Restored Wallet")
				scanner.db.SetActiveWallet(chatID, encWallet.PublicKey)
			}
		}
	}

	wallets, err := scanner.db.GetUserWallets(chatID)
	if err != nil {
		sendError(bot, chatID, "Error retrieving wallets")
		log.Printf("Error getting wallets: %v", err)
		return
	}

	if len(wallets) == 0 {
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("➕ Add Wallet", "add_wallet"),
			),
		)

		msg := tgbotapi.NewMessage(chatID, "👛 *Wallet Manager*\n\nYou have no wallets yet.\n\nClick below to add your first wallet!")
		msg.ParseMode = "Markdown"
		msg.ReplyMarkup = keyboard
		bot.Send(msg)
		return
	}

	// Build wallet list
	message := "👛 *Your Wallets*\n\n"

	var buttons [][]tgbotapi.InlineKeyboardButton
	for i, wallet := range wallets {
		status := ""
		if wallet.IsActive {
			status = "✅ "
		}

		// Check if this wallet has encrypted private key (trading capability)
		hasEncrypted := scanner.db.HasEncryptedWallet(chatID)
		encWallet, _ := scanner.db.GetEncryptedWallet(chatID)
		tradingIcon := ""
		if hasEncrypted && encWallet != nil && encWallet.PublicKey == wallet.WalletAddress {
			tradingIcon = "🔑 "
		}

		name := wallet.WalletName
		if name == "" {
			name = fmt.Sprintf("Wallet %d", i+1)
		}

		shortAddr := wallet.WalletAddress[:4] + "..." + wallet.WalletAddress[len(wallet.WalletAddress)-4:]
		message += fmt.Sprintf("%s%s*%s* `%s`\n", status, tradingIcon, name, shortAddr)

		// Add button for this wallet
		buttons = append(buttons, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("%s%s%s", status, tradingIcon, name),
				fmt.Sprintf("select_wallet:%s", wallet.WalletAddress),
			),
		))
	}

	message += "\n_🔑 = Trading enabled (encrypted wallet)_\n"

	// Add action buttons
	buttons = append(buttons, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("➕ Add Wallet", "add_wallet"),
		tgbotapi.NewInlineKeyboardButtonData("🗑 Remove", "remove_wallet"),
	))

	keyboard := tgbotapi.NewInlineKeyboardMarkup(buttons...)

	msg := tgbotapi.NewMessage(chatID, message)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}

// extractAPIKey extracts API key from WebSocket URL
func extractAPIKey(wsURL string) string {
	parts := strings.Split(wsURL, "api_key=")
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}

// editMessage edits an existing message
func editMessage(bot *tgbotapi.BotAPI, chatID int64, messageID int, text string) {
	edit := tgbotapi.NewEditMessageText(chatID, messageID, text)
	edit.ParseMode = "Markdown"
	bot.Send(edit)
}
