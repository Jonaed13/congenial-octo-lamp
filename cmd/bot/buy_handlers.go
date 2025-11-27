package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"runtime"
	"solana-orchestrator/api"
	"solana-orchestrator/config"
	"solana-orchestrator/crypto"
	"solana-orchestrator/storage"
	"solana-orchestrator/trading"
	"strconv"
	"strings"
	"time"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// handleStartBuy initiates the buy flow
func handleStartBuy(bot *tgbotapi.BotAPI, chatID int64) {
	// Check if user has encrypted wallet
	if !scanner.db.HasEncryptedWallet(chatID) {
		send(bot, chatID, "⚠️ No wallet found!\n\nUse /wallets to create or import a wallet first.")
		return
	}

	// Start buy flow
	sessMu.Lock()
	sessions[chatID] = &UserSession{
		State:       "awaiting_buy_token",
		RequestedAt: time.Now().Unix(),
	}
	sessMu.Unlock()

	msg := "✅ *Buy Token*\n\n" +
		"Enter the token address you want to buy:"

	send(bot, chatID, msg)
}

// handleBuyTokenInput processes token address for buying
func handleBuyTokenInput(bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	chatID := msg.Chat.ID
	tokenAddress := strings.TrimSpace(msg.Text)

	// Validate Solana address format
	_, err := solana.PublicKeyFromBase58(tokenAddress)
	if err != nil {
		sendError(bot, chatID, "Invalid token address format!\n\nPlease enter a valid Solana token address:")
		return
	}

	// Show loading
	loadingMsg := tgbotapi.NewMessage(chatID, "⏳ Looking up token...")
	loadingMsg.ParseMode = "Markdown"
	sentMsg, _ := bot.Send(loadingMsg)

	// Fetch token info from DexScreener
	// 1. Try DexScreener First
	tokenInfo, err := trading.GetTokenInfo(context.Background(), tokenAddress)
	if err != nil {
		log.Printf("DexScreener failed for %s: %v", tokenAddress, err)
		// Fallback: Create empty token info and rely on Shyft
		tokenInfo = &trading.TokenInfo{
			Address:   tokenAddress,
			Name:      "Unknown",
			Symbol:    "Unknown",
			PriceUSD:  "N/A",
			Liquidity: 0,
		}
	}

	// 2. Fetch Metadata & Supply from Shyft (Always try to augment/fix)
	rpcURL := "https://rpc.shyft.to?api_key=48KZbYxP-9e9SpqR" // Hardcoded for now as per user request, or load from config
	shyftMeta, errShyft := api.GetShyftMetadata(rpcURL, tokenAddress)
	if errShyft == nil {
		// Overwrite/Augment with Shyft Data
		tokenInfo.Name = shyftMeta.Name
		tokenInfo.Symbol = shyftMeta.Symbol
		tokenInfo.TotalSupply = shyftMeta.TotalSupply
	} else {
		fmt.Printf("⚠️ Shyft Metadata Error: %v\n", errShyft)
	}

	// If both failed (Name is still Unknown), then we can't proceed safely
	if tokenInfo.Name == "Unknown" && tokenInfo.Symbol == "Unknown" {
		editMessage(bot, chatID, sentMsg.MessageID, "❌ Token not found on DexScreener or Solana Chain. Please check the address.")
		return
	}

	// Store token info in temp storage
	tempBuyData[chatID] = &BuyData{
		TokenAddress: tokenAddress,
		TokenInfo:    tokenInfo,
	}

	// Show token info and ask for amount
	message := fmt.Sprintf("🪙 *%s (%s)*\n\n", tokenInfo.Name, tokenInfo.Symbol)
	message += fmt.Sprintf("💰 *Price:* $%s\n", tokenInfo.PriceUSD)
	message += fmt.Sprintf("📦 *Supply:* %s\n", tokenInfo.TotalSupply)
	message += fmt.Sprintf("📊 *24h Change:* %.2f%%\n", tokenInfo.Change24h)
	message += fmt.Sprintf("💧 *Liquidity:* $%.0f\n", tokenInfo.Liquidity)
	message += fmt.Sprintf("📈 *Volume 24h:* $%.0f\n\n", tokenInfo.Volume24h)
	message += fmt.Sprintf("🔥 *Buys (5m):* %d | *Sells:* %d\n\n", tokenInfo.Buys5m, tokenInfo.Sells5m)
	message += "💵 *Enter SOL amount to spend:*"

	// Update session state
	sessMu.Lock()
	sessions[chatID].State = "awaiting_buy_amount"
	sessMu.Unlock()

	editMessage(bot, chatID, sentMsg.MessageID, message)
}

// handleBuyAmountInput processes SOL amount for buying
func handleBuyAmountInput(bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	chatID := msg.Chat.ID

	// Parse SOL amount
	amountStr := strings.TrimSpace(msg.Text)
	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil || amount <= 0 {
		sendError(bot, chatID, "Invalid amount!\n\nPlease enter a valid SOL amount (e.g., 0.1):")
		return
	}

	// Get buy data
	buyData, ok := tempBuyData[chatID]
	if !ok {
		sendError(bot, chatID, "Session expired. Please start over with /buy")
		cleanupBuySession(chatID)
		return
	}

	buyData.SOLAmount = amount

	// Check balance
	wallet, err := scanner.db.GetEncryptedWallet(chatID)
	if err != nil || wallet == nil {
		sendError(bot, chatID, "Failed to load wallet")
		cleanupBuySession(chatID)
		return
	}

	// Get SOL balance via Shyft
	walletPubkey, _ := solana.PublicKeyFromBase58(wallet.PublicKey)
	rpcURL := "https://rpc.shyft.to?api_key=48KZbYxP-9e9SpqR"
	wsClient := trading.NewWSClient(getShyftWSURL())

	// Load config to get API keys
	cfg, err := config.Load("config/config.json")
	if err != nil {
		send(bot, chatID, "❌ Failed to load config")
		return
	}
	apiClient := api.NewClient(cfg.MoralisAPIKey, cfg.BirdeyeAPIKey, cfg.APISettings.MaxRetries, cfg.MoralisFallbackKeys)
	balanceMgr := trading.NewBalanceManager(rpcURL, wsClient, apiClient)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	solBalance, err := balanceMgr.GetSOLBalance(ctx, walletPubkey)
	if err != nil {
		send(bot, chatID, fmt.Sprintf("❌ Failed to check balance: %v", err))
		cleanupBuySession(chatID)
		return
	}

	solBalanceFloat := trading.FormatSOL(solBalance)

	// Check if enough balance (including fees)
	const estimatedFees = 0.001 // ~0.001 SOL for transaction fees
	if solBalanceFloat < amount+estimatedFees {
		message := fmt.Sprintf("❌ *Insufficient Balance!*\n\n")
		message += fmt.Sprintf("💰 Your Balance: %.6f SOL\n", solBalanceFloat)
		message += fmt.Sprintf("💸 Required: %.6f SOL\n", amount)
		message += fmt.Sprintf("⚡ Est. Fees: %.6f SOL\n", estimatedFees)
		message += fmt.Sprintf("📊 Total Needed: %.6f SOL\n\n", amount+estimatedFees)
		message += fmt.Sprintf("⚠️ You need %.6f more SOL", (amount+estimatedFees)-solBalanceFloat)

		send(bot, chatID, message)
		cleanupBuySession(chatID)
		return
	}

	// Get user settings for slippage
	settings, err := scanner.db.GetUserSettings(chatID)
	if err != nil {
		settings = &storage.UserSettings{SlippageBps: 500, JitoTipLamports: 10000} // defaults
	}

	// Calculate expected tokens (rough estimate)
	priceSOL, _ := strconv.ParseFloat(buyData.TokenInfo.PriceSOL, 64)
	var expectedTokens float64
	if priceSOL > 0 {
		expectedTokens = amount / priceSOL
	}

	// Show confirmation
	message := "⚠️ *Confirm Purchase*\n\n"
	message += fmt.Sprintf("🪙 *Token:* %s (%s)\n", buyData.TokenInfo.Name, buyData.TokenInfo.Symbol)
	message += fmt.Sprintf("💰 *Spend:* %.6f SOL\n", amount)
	if expectedTokens > 0 {
		message += fmt.Sprintf("📊 *Receive:* ~%.2f %s\n", expectedTokens, buyData.TokenInfo.Symbol)
	}
	message += fmt.Sprintf("⚙️ *Slippage:* %.1f%%\n", float64(settings.SlippageBps)/100)
	message += fmt.Sprintf("💎 *Jito Tip:* %.6f SOL\n\n", float64(settings.JitoTipLamports)/1e9)
	message += "⚠️ Slippage: Final amount may vary based on market\n\n"
	message += "Click Confirm to proceed:"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Confirm", "confirm_buy"),
			tgbotapi.NewInlineKeyboardButtonData("❌ Cancel", "cancel_buy"),
		),
	)

	msgConfig := tgbotapi.NewMessage(chatID, message)
	msgConfig.ParseMode = "Markdown"
	msgConfig.ReplyMarkup = keyboard
	bot.Send(msgConfig)

	// Update state
	sessMu.Lock()
	sessions[chatID].State = "awaiting_buy_confirm"
	sessMu.Unlock()
}

// handleConfirmBuy executes the buy after password
func handleConfirmBuy(bot *tgbotapi.BotAPI, chatID int64) {
	// Ask for password
	sessMu.Lock()
	sessions[chatID].State = "awaiting_buy_password"
	sessMu.Unlock()

	send(bot, chatID, "🔐 *Enter your wallet password:*\n\n⚠️ Message will be deleted for security")
}

// handleBuyPassword processes password and executes buy
func handleBuyPassword(bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	chatID := msg.Chat.ID
	password := msg.Text

	// Delete password message
	deleteMsg := tgbotapi.NewDeleteMessage(chatID, msg.MessageID)
	bot.Request(deleteMsg)

	// Get buy data
	buyData, ok := tempBuyData[chatID]
	if !ok {
		send(bot, chatID, "❌ Session expired")
		cleanupBuySession(chatID)
		return
	}

	send(bot, chatID, "⏳ Processing transaction...\n\nThis may take a few seconds")

	// 1. Decrypt private key
	encWallet, err := scanner.db.GetEncryptedWalletForDecryption(chatID)
	if err != nil {
		send(bot, chatID, "❌ Failed to load wallet")
		cleanupBuySession(chatID)
		return
	}

	// Convert storage.EncryptedWallet to crypto.EncryptedWallet
	cryptoWallet := &crypto.EncryptedWallet{
		EncryptedKey: []byte(encWallet.EncryptedPrivateKey),
		Salt:         []byte(encWallet.EncryptionSalt),
		Nonce:        []byte(encWallet.Nonce),
		PasswordHash: encWallet.PasswordHash,
	}

	privateKeyStr, err := crypto.DecryptPrivateKey(cryptoWallet, password)
	if err != nil {
		send(bot, chatID, "❌ Incorrect password!")
		// Don't cleanup session, let them try again?
		// For now, cleanup to be safe
		cleanupBuySession(chatID)
		return
	}

	// Parse private key
	privateKey, err := solana.PrivateKeyFromBase58(privateKeyStr)
	if err != nil {
		send(bot, chatID, "❌ Invalid private key in wallet")
		cleanupBuySession(chatID)
		return
	}

	// Clear private key string from memory (best effort)
	crypto.ZeroString(&privateKeyStr)

	// 2. Get User Settings
	settings, err := scanner.db.GetUserSettings(chatID)
	if err != nil {
		settings = &storage.UserSettings{SlippageBps: 500, JitoTipLamports: 10000}
	}

	// 3. Get Jupiter Quote
	solAmountLamports := uint64(buyData.SOLAmount * 1e9)
	quote, err := trading.GetBuyQuote(context.Background(), buyData.TokenAddress, solAmountLamports, settings.SlippageBps)
	if err != nil {
		send(bot, chatID, fmt.Sprintf("❌ Failed to get quote: %v", err))
		cleanupBuySession(chatID)
		return
	}

	// 4. Get Swap Transaction
	// Use Jito tip as priority fee if < 0.001 SOL, otherwise use standard
	priorityFee := int64(10000) // Default low priority fee
	swapResp, err := trading.GetSwapTransaction(context.Background(), quote, privateKey.PublicKey().String(), priorityFee)
	if err != nil {
		send(bot, chatID, fmt.Sprintf("❌ Failed to build transaction: %v", err))
		cleanupBuySession(chatID)
		return
	}

	// 5. Deserialize and Sign
	txBytes, err := base64.StdEncoding.DecodeString(swapResp.SwapTransaction)
	if err != nil {
		send(bot, chatID, "❌ Failed to decode transaction")
		cleanupBuySession(chatID)
		return
	}

	tx, err := solana.TransactionFromDecoder(bin.NewBinDecoder(txBytes))
	if err != nil {
		// Try versioned transaction if standard fails
		// For now, assume standard or handle error
		send(bot, chatID, fmt.Sprintf("❌ Failed to deserialize transaction: %v", err))
		cleanupBuySession(chatID)
		return
	}

	// Sign transaction
	_, err = tx.Sign(
		func(key solana.PublicKey) *solana.PrivateKey {
			if key.Equals(privateKey.PublicKey()) {
				return &privateKey
			}
			return nil
		},
	)
	if err != nil {
		send(bot, chatID, fmt.Sprintf("❌ Failed to sign transaction: %v", err))
		cleanupBuySession(chatID)
		return
	}

	// 6. Submit via Jito (if tip > 0) or RPC
	// For now, we'll use Jito if tip is configured, otherwise RPC
	// But we need a Jito client.
	// We'll assume Jito is preferred for reliability.

	// Initialize Jito Client (using Amsterdam endpoint as default)
	jitoClient := trading.NewJitoClient("https://amsterdam.mainnet.block-engine.jito.wtf/api/v1/bundles", uint64(settings.JitoTipLamports))

	// Create Tip Instruction if using Jito
	if settings.JitoTipLamports > 0 {
		tipInst, err := jitoClient.CreateTipInstruction(privateKey.PublicKey())
		if err == nil {
			// Add tip instruction to transaction
			// Note: Modifying a signed transaction invalidates signature.
			// We should have added the tip BEFORE signing.
			// But Jupiter returns a constructed transaction.
			// Jito bundles allow multiple transactions.
			// So we create a separate tip transaction and bundle them.

			// Create separate tip transaction
			recentBlockhash := tx.Message.RecentBlockhash
			tipTx, err := solana.NewTransaction(
				[]solana.Instruction{tipInst},
				recentBlockhash,
				solana.TransactionPayer(privateKey.PublicKey()),
			)
			if err == nil {
				tipTx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
					if key.Equals(privateKey.PublicKey()) {
						return &privateKey
					}
					return nil
				})

				// Submit Bundle
				bundleRes, err := jitoClient.SubmitBundle(context.Background(), []solana.Transaction{*tx, *tipTx})
				if err != nil {
					send(bot, chatID, fmt.Sprintf("❌ Jito submission failed: %v", err))
					cleanupBuySession(chatID)
					return
				}

				send(bot, chatID, fmt.Sprintf("✅ *Bundle Submitted!*\n\nBundle ID: `%s`\n\nWaiting for confirmation...", bundleRes.BundleID))
				cleanupBuySession(chatID)
				return
			}
		}
	}

	// Fallback to standard RPC if Jito fails or no tip
	rpcURL := "https://rpc.shyft.to?api_key=48KZbYxP-9e9SpqR"
	rpcClient := rpc.New(rpcURL)

	sig, err := rpcClient.SendTransaction(context.Background(), tx)
	if err != nil {
		send(bot, chatID, fmt.Sprintf("❌ Transaction failed: %v", err))
		cleanupBuySession(chatID)
		return
	}

	message := "✅ *Transaction Submitted!*\n\n"
	message += fmt.Sprintf("🪙 Token: %s\n", buyData.TokenInfo.Symbol)
	message += fmt.Sprintf("💰 Amount: %.6f SOL\n\n", buyData.SOLAmount)
	message += fmt.Sprintf("🔗 Signature: `%s`\n", sig.String())
	message += "⏳ Waiting for confirmation..."

	send(bot, chatID, message)
	cleanupBuySession(chatID)
}

// Temporary storage for buy flow
type BuyData struct {
	TokenAddress string
	TokenInfo    *trading.TokenInfo
	SOLAmount    float64
}

var tempBuyData = make(map[int64]*BuyData)

// cleanupBuySession cleans up buy session data
func cleanupBuySession(chatID int64) {
	sessMu.Lock()
	delete(sessions, chatID)
	sessMu.Unlock()

	delete(tempBuyData, chatID)
	runtime.GC()
}

// Helper to get Shyft WS URL
func getShyftWSURL() string {
	return "wss://rpc.shyft.to?api_key=48KZbYxP-9e9SpqR"
}
