package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"runtime"
	"solana-orchestrator/api"
	"solana-orchestrator/crypto"
	"solana-orchestrator/storage"
	"solana-orchestrator/trading"
	"time"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// handleStartSell initiates sell flow
func handleStartSell(bot *tgbotapi.BotAPI, chatID int64) {
	// Check if user has encrypted wallet
	if !scanner.db.HasEncryptedWallet(chatID) {
		sendWarning(bot, chatID, "No wallet found!\n\nUse /wallets to create or import a wallet first.")
		return
	}

	// Get wallet
	wallet, err := scanner.db.GetEncryptedWallet(chatID)
	if err != nil || wallet == nil {
		sendError(bot, chatID, "Failed to load wallet")
		return
	}

	send(bot, chatID, "⏳ Loading your token holdings...")

	// Get token balances
	walletPubkey, _ := solana.PublicKeyFromBase58(wallet.PublicKey)
	rpcURL := "https://rpc.shyft.to?api_key=48KZbYxP-9e9SpqR"
	wsClient := trading.NewWSClient(getShyftWSURL())
	apiClient := api.NewClient(globalCfg.MoralisAPIKey, globalCfg.BirdeyeAPIKey, globalCfg.APISettings.MaxRetries, globalCfg.MoralisFallbackKeys)
	balanceMgr := trading.NewBalanceManager(rpcURL, wsClient, apiClient)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	tokenBalances, err := balanceMgr.GetTokenBalances(ctx, walletPubkey)
	if err != nil {
		sendError(bot, chatID, fmt.Sprintf("Failed to fetch token balances: %v", err))
		return
	}

	if len(tokenBalances) == 0 {
		sendWarning(bot, chatID, "No tokens found in your wallet\n\nBuy some tokens first!")
		return
	}

	// Show tokens with prices
	message := "📊 *Your Token Holdings*\n\n"
	message += "Select a token to sell:\n\n"

	var keyboard [][]tgbotapi.InlineKeyboardButton

	for i, token := range tokenBalances {
		if i >= 10 { // Limit to 10 tokens
			message += "\n_...and more. Showing first 10 tokens_"
			break
		}

		tokenMintStr := token.Mint.String()

		// Try to get price from DexScreener
		var priceInfo string
		tokenInfo, err := trading.GetTokenInfo(context.Background(), tokenMintStr)
		if err == nil {
			priceInfo = fmt.Sprintf(" - $%s", tokenInfo.PriceUSD)
			if tokenInfo.Change24h != 0 {
				changeEmoji := "📈"
				if tokenInfo.Change24h < 0 {
					changeEmoji = "📉"
				}
				priceInfo += fmt.Sprintf(" %s %.1f%%", changeEmoji, tokenInfo.Change24h)
			}
		}

		// Format token display
		tokenDisplay := fmt.Sprintf("%d. `%s...%s`\n   Amount: %.2f%s",
			i+1,
			tokenMintStr[:4],
			tokenMintStr[len(tokenMintStr)-4:],
			float64(token.Amount)/1e9, // Convert uint64 to tokenswith decimals
			priceInfo,
		)
		message += tokenDisplay + "\n\n"

		// Add button
		buttonText := fmt.Sprintf("%d. Sell", i+1)
		callbackData := fmt.Sprintf("sell_token:%s", tokenMintStr)
		keyboard = append(keyboard, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(buttonText, callbackData),
		))
	}

	keyboard = append(keyboard, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔙 Back", "back_to_menu"),
	))

	msgConfig := tgbotapi.NewMessage(chatID, message)
	msgConfig.ParseMode = "Markdown"
	msgConfig.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(keyboard...)
	bot.Send(msgConfig)
}

// handleSellToken shows sell options for a specific token
func handleSellToken(bot *tgbotapi.BotAPI, chatID int64, tokenMint string) {
	// Get token info
	tokenInfo, err := trading.GetTokenInfo(context.Background(), tokenMint)
	if err != nil {
		sendError(bot, chatID, fmt.Sprintf("Error loading token: %v\n\nTry again", err))
		return
	}

	// Get token balance
	wallet, _ := scanner.db.GetEncryptedWallet(chatID)
	walletPubkey, _ := solana.PublicKeyFromBase58(wallet.PublicKey)

	rpcURL := "https://rpc.shyft.to?api_key=48KZbYxP-9e9SpqR"
	wsClient := trading.NewWSClient(getShyftWSURL())
	apiClient := api.NewClient(globalCfg.MoralisAPIKey, globalCfg.BirdeyeAPIKey, globalCfg.APISettings.MaxRetries, globalCfg.MoralisFallbackKeys)
	balanceMgr := trading.NewBalanceManager(rpcURL, wsClient, apiClient)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tokenBalances, _ := balanceMgr.GetTokenBalances(ctx, walletPubkey)

	var tokenBalance float64
	for _, tb := range tokenBalances {
		if tb.Mint.String() == tokenMint {
			tokenBalance = float64(tb.Amount) / 1e9
			break
		}
	}

	if tokenBalance == 0 {
		send(bot, chatID, "❌ You don't own this token anymore")
		return
	}

	// Show sell options
	message := fmt.Sprintf("❌ *Sell %s*\n\n", tokenInfo.Symbol)
	message += fmt.Sprintf("💰 *Balance:* %.2f tokens\n", tokenBalance)
	message += fmt.Sprintf("💵 *Price:* $%s\n", tokenInfo.PriceUSD)
	message += fmt.Sprintf("📊 *24h:* %.2f%%\n\n", tokenInfo.Change24h)
	message += "*Select amount to sell:*"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("25%", fmt.Sprintf("sell_pct:%s:25", tokenMint)),
			tgbotapi.NewInlineKeyboardButtonData("50%", fmt.Sprintf("sell_pct:%s:50", tokenMint)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("75%", fmt.Sprintf("sell_pct:%s:75", tokenMint)),
			tgbotapi.NewInlineKeyboardButtonData("100%", fmt.Sprintf("sell_pct:%s:100", tokenMint)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Back", "start_sell"),
		),
	)

	msgConfig := tgbotapi.NewMessage(chatID, message)
	msgConfig.ParseMode = "Markdown"
	msgConfig.ReplyMarkup = keyboard
	bot.Send(msgConfig)

	// Store in temp
	tempSellData[chatID] = &SellData{
		TokenMint:    tokenMint,
		TokenInfo:    tokenInfo,
		TokenBalance: tokenBalance,
	}
}

// handleSellPercentage confirms sell with percentage
func handleSellPercentage(bot *tgbotapi.BotAPI, chatID int64, tokenMint string, percentage int) {
	sellData, ok := tempSellData[chatID]
	if !ok || sellData.TokenMint != tokenMint {
		send(bot, chatID, "❌ Session expired. Please start over.")
		return
	}

	sellAmount := sellData.TokenBalance * float64(percentage) / 100.0
	sellData.SellAmount = sellAmount
	sellData.Percentage = percentage

	// Show confirmation
	message := "⚠️ *Confirm Sale*\n\n"
	message += fmt.Sprintf("🪙 *Token:* %s\n", sellData.TokenInfo.Symbol)
	message += fmt.Sprintf("💰 *Sell:* %.2f tokens (%d%%)\n", sellAmount, percentage)
	message += fmt.Sprintf("💵 *Est. Receive:* ~%.6f SOL\n\n", sellAmount*parseFloat(sellData.TokenInfo.PriceSOL))
	message += "⚠️ Final amount depends on market slippage\n\n"
	message += "Click Confirm to proceed:"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Confirm", "confirm_sell"),
			tgbotapi.NewInlineKeyboardButtonData("❌ Cancel", "start_sell"),
		),
	)

	msgConfig := tgbotapi.NewMessage(chatID, message)
	msgConfig.ParseMode = "Markdown"
	msgConfig.ReplyMarkup = keyboard
	bot.Send(msgConfig)
}

// handleConfirmSell asks for password
func handleConfirmSell(bot *tgbotapi.BotAPI, chatID int64) {
	// Update state
	sessMu.Lock()
	sessions[chatID].State = "awaiting_sell_password"
	sessMu.Unlock()

	send(bot, chatID, "🔐 *Enter your wallet password:*\n\n⚠️ Message will be deleted for security")
}

// handleSellPassword processes password and executes sell
func handleSellPassword(bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	chatID := msg.Chat.ID
	password := msg.Text

	// Delete password message
	deleteMsg := tgbotapi.NewDeleteMessage(chatID, msg.MessageID)
	bot.Request(deleteMsg)

	// Get sell data
	sellData, ok := tempSellData[chatID]
	if !ok {
		send(bot, chatID, "❌ Session expired")
		cleanupSellSession(chatID)
		return
	}

	send(bot, chatID, "⏳ Processing transaction...\n\nThis may take a few seconds")

	// 1. Decrypt private key
	encWallet, err := scanner.db.GetEncryptedWalletForDecryption(chatID)
	if err != nil {
		send(bot, chatID, "❌ Failed to load wallet")
		cleanupSellSession(chatID)
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
		cleanupSellSession(chatID)
		return
	}

	// Parse private key
	privateKey, err := solana.PrivateKeyFromBase58(privateKeyStr)
	if err != nil {
		send(bot, chatID, "❌ Invalid private key in wallet")
		cleanupSellSession(chatID)
		return
	}

	// Clear private key string
	crypto.ZeroString(&privateKeyStr)

	// 2. Get User Settings
	settings, err := scanner.db.GetUserSettings(chatID)
	if err != nil {
		settings = &storage.UserSettings{SlippageBps: 500, JitoTipLamports: 10000}
	}

	// 3. Get Jupiter Quote
	// Fetch decimals from RPC to convert token amount to raw amount
	rpcURL := "https://rpc.shyft.to?api_key=48KZbYxP-9e9SpqR"
	rpcClient := rpc.New(rpcURL)

	mintPubkey := solana.MustPublicKeyFromBase58(sellData.TokenMint)
	supply, err := rpcClient.GetTokenSupply(context.Background(), mintPubkey, rpc.CommitmentFinalized)
	var decimals uint8
	if err == nil {
		decimals = uint8(supply.Value.Decimals)
	} else {
		sendError(bot, chatID, "Failed to get token decimals")
		cleanupSellSession(chatID)
		return
	}

	tokenAmountRaw := uint64(sellData.SellAmount * float64(pow10(int(decimals))))

	quote, err := trading.GetSellQuote(context.Background(), sellData.TokenMint, tokenAmountRaw, settings.SlippageBps)
	if err != nil {
		send(bot, chatID, fmt.Sprintf("❌ Failed to get quote: %v", err))
		cleanupSellSession(chatID)
		return
	}

	// 4. Get Swap Transaction
	priorityFee := int64(10000)
	swapResp, err := trading.GetSwapTransaction(context.Background(), quote, privateKey.PublicKey().String(), priorityFee)
	if err != nil {
		send(bot, chatID, fmt.Sprintf("❌ Failed to build transaction: %v", err))
		cleanupSellSession(chatID)
		return
	}

	// 5. Deserialize and Sign
	txBytes, err := base64.StdEncoding.DecodeString(swapResp.SwapTransaction)
	if err != nil {
		send(bot, chatID, "❌ Failed to decode transaction")
		cleanupSellSession(chatID)
		return
	}

	tx, err := solana.TransactionFromDecoder(bin.NewBinDecoder(txBytes))
	if err != nil {
		send(bot, chatID, fmt.Sprintf("❌ Failed to deserialize transaction: %v", err))
		cleanupSellSession(chatID)
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
		cleanupSellSession(chatID)
		return
	}

	// 6. Submit via Jito (if tip > 0) or RPC
	// Initialize Jito Client
	jitoClient := trading.NewJitoClient("https://amsterdam.mainnet.block-engine.jito.wtf/api/v1/bundles", uint64(settings.JitoTipLamports))

	if settings.JitoTipLamports > 0 {
		tipInst, err := jitoClient.CreateTipInstruction(privateKey.PublicKey())
		if err == nil {
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

				bundleRes, err := jitoClient.SubmitBundle(context.Background(), []solana.Transaction{*tx, *tipTx})
				if err != nil {
					send(bot, chatID, fmt.Sprintf("❌ Jito submission failed: %v", err))
					cleanupSellSession(chatID)
					return
				}

				send(bot, chatID, fmt.Sprintf("✅ *Bundle Submitted!*\n\nBundle ID: `%s`\n\nWaiting for confirmation...", bundleRes.BundleID))
				cleanupSellSession(chatID)
				return
			}
		}
	}

	// Fallback to RPC
	sig, err := rpcClient.SendTransaction(context.Background(), tx)
	if err != nil {
		send(bot, chatID, fmt.Sprintf("❌ Transaction failed: %v", err))
		cleanupSellSession(chatID)
		return
	}

	message := "✅ *Transaction Submitted!*\n\n"
	message += fmt.Sprintf("🪙 Token: %s\n", sellData.TokenInfo.Symbol)
	message += fmt.Sprintf("💰 Sold: %.2f tokens\n\n", sellData.SellAmount)
	message += fmt.Sprintf("🔗 Signature: `%s`\n", sig.String())
	message += "⏳ Waiting for confirmation..."

	send(bot, chatID, message)
	cleanupSellSession(chatID)
}

// Temporary storage for sell flow
type SellData struct {
	TokenMint    string
	TokenInfo    *trading.TokenInfo
	TokenBalance float64
	SellAmount   float64
	Percentage   int
}

var tempSellData = make(map[int64]*SellData)

// Helper functions
func parseFloat(s string) float64 {
	var f float64
	fmt.Sscanf(s, "%f", &f)
	return f
}

func pow10(n int) uint64 {
	result := uint64(1)
	for i := 0; i < n; i++ {
		result *= 10
	}
	return result
}

func cleanupSellSession(chatID int64) {
	delete(tempSellData, chatID)
	runtime.GC()
}
