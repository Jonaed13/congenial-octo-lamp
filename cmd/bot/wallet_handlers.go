package main

import (
	"fmt"
	"log"
	"runtime"
	"solana-orchestrator/crypto"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// handleGenerateWallet starts the wallet generation flow
func handleGenerateWallet(bot *tgbotapi.BotAPI, chatID int64) {
	// Check if user already has a wallet
	if scanner.db.HasEncryptedWallet(chatID) {
		send(bot, chatID, "⚠️ You already have a wallet! Use /wallets to view it.\n\nTo create a new wallet, you must first delete your existing one.")
		return
	}

	// Show security warning
	warning := "🔐 **SECURITY WARNING** 🔐\n\n" +
		"You're about to create a NEW wallet.\n\n" +
		"**IMPORTANT:**\n" +
		"• You will set a PASSWORD to encrypt your private key\n" +
		"• If you FORGET your password, your funds are LOST FOREVER\n" +
		"• We CANNOT recover your password\n" +
		"• Your private key is encrypted and stored securely\n" +
		"• Only use this for trading amounts you can afford to lose\n\n" +
		"**DO YOU UNDERSTAND AND ACCEPT THESE RISKS?**"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Yes, Create Wallet", "confirm_generate"),
			tgbotapi.NewInlineKeyboardButtonData("❌ Cancel", "cancel_wallet"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, warning)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}

// handleConfirmGenerate generates the wallet after user confirmation
func handleConfirmGenerate(bot *tgbotapi.BotAPI, chatID int64) {
	// Generate wallet
	send(bot, chatID, "⏳ Generating your wallet...")

	wallet, err := crypto.GenerateWallet()
	if err != nil {
		send(bot, chatID, fmt.Sprintf("❌ Error generating wallet: %v", err))
		log.Printf("Wallet generation error: %v", err)
		return
	}

	// Store wallet info in session for password step
	sessMu.Lock()
	sessions[chatID] = &UserSession{
		State:       "awaiting_wallet_password",
		RequestedAt: time.Now().Unix(),
	}
	sessMu.Unlock()

	// Temporarily store keypair (will be encrypted with password)
	tempWalletKeypair[chatID] = wallet

	message := "✅ **Wallet Generated Successfully!**\n\n" +
		fmt.Sprintf("📍 **Your Wallet Address:**\n`%s`\n\n", wallet.PublicKey) +
		"🔑 **Your Seed Phrase (BACKUP THIS!):**\n" +
		"```\n" + wallet.Mnemonic + "\n```\n\n" +
		"⚠️ **WRITE THIS DOWN NOW!** Store it somewhere safe!\n\n" +
		"Now, set a strong password to encrypt your private key:"

	send(bot, chatID, message)
}

// handleWalletPassword processes the password for wallet encryption
func handleWalletPassword(bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	chatID := msg.Chat.ID
	password := msg.Text

	// Delete the password message for security
	deleteMsg := tgbotapi.NewDeleteMessage(chatID, msg.MessageID)
	bot.Request(deleteMsg)

	// Validate password strength
	if len(password) < 8 {
		send(bot, chatID, "❌ Password too short! Minimum 8 characters.\n\nPlease try again:")
		return
	}

	// Get stored keypair
	wallet, ok := tempWalletKeypair[chatID]
	if !ok {
		send(bot, chatID, "❌ Session expired. Please start over with /wallets")
		sessMu.Lock()
		delete(sessions, chatID)
		sessMu.Unlock()
		return
	}

	// Encrypt private key
	encWallet, err := crypto.EncryptPrivateKey(wallet.PrivateKey, password)
	if err != nil {
		send(bot, chatID, fmt.Sprintf("❌ Encryption error: %v", err))
		cleanupWalletSession(chatID)
		return
	}

	// Encrypt mnemonic too
	encMnemonic, err := crypto.EncryptPrivateKey(wallet.Mnemonic, password)
	if err != nil {
		send(bot, chatID, fmt.Sprintf("❌ Mnemonic encryption error: %v", err))
		cleanupWalletSession(chatID)
		return
	}

	// Save encrypted wallet to database
	err = scanner.db.SaveEncryptedWallet(
		chatID,
		wallet.PublicKey,
		encWallet.EncryptedKey,
		encWallet.Salt,
		encWallet.Nonce,
		encWallet.PasswordHash,
		crypto.EncodeToBase64(encMnemonic.EncryptedKey),
	)
	if err != nil {
		send(bot, chatID, fmt.Sprintf("❌ Database error: %v", err))
		cleanupWalletSession(chatID)
		return
	}

	// Also add to user_wallets for display/tracking
	err = scanner.db.AddUserWallet(chatID, wallet.PublicKey, "Trading Wallet")
	if err != nil {
		send(bot, chatID, fmt.Sprintf("❌ Error adding wallet: %v", err))
		cleanupWalletSession(chatID)
		return
	}

	// Set as active wallet
	scanner.db.SetActiveWallet(chatID, wallet.PublicKey)

	// Cleanup temporary storage
	cleanupWalletSession(chatID)

	// Success message
	message := "✅ **Wallet Created Successfully!**\n\n" +
		fmt.Sprintf("📍 **Address:** `%s`\n\n", wallet.PublicKey) +
		"🔒 Your private key is now encrypted and stored securely.\n\n" +
		"**IMPORTANT REMINDERS:**\n" +
		"• Never share your seed phrase with anyone\n" +
		"• Keep your password safe\n" +
		"• If you forget your password, your funds are lost\n\n" +
		"✅ Wallet is now active and ready to use!\n\n" +
		"Use /balance to check your wallet balance!"

	send(bot, chatID, message)

	// Show wallet manager
	handleWalletsCommand(bot, chatID)
}

// handleImportWallet starts the wallet import flow
func handleImportWallet(bot *tgbotapi.BotAPI, chatID int64) {
	if scanner.db.HasEncryptedWallet(chatID) {
		send(bot, chatID, "⚠️ You already have a wallet!")
		return
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔑 Private Key", "import_privkey"),
			tgbotapi.NewInlineKeyboardButtonData("📝 Seed Phrase", "import_mnemonic"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Cancel", "cancel_wallet"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, "📥 **Import Wallet**\n\nHow would you like to import?")
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}

// cleanupWalletSession clears temporary wallet data
func cleanupWalletSession(chatID int64) {
	// Clear from session
	sessMu.Lock()
	delete(sessions, chatID)
	sessMu.Unlock()

	// Clear temporary keypair
	if wallet, ok := tempWalletKeypair[chatID]; ok {
		// Zero out sensitive data
		crypto.ZeroString(&wallet.PrivateKey)
		crypto.ZeroString(&wallet.Mnemonic)
		delete(tempWalletKeypair, chatID)
	}

	// Force garbage collection
	runtime.GC()
}

// Global temp storage for wallet generation
var tempWalletKeypair = make(map[int64]*crypto.WalletKeypair)
