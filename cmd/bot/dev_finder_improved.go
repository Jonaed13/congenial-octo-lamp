package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	"solana-orchestrator/storage"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// SearchSession tracks an active search with real-time results
type SearchSession struct {
	ChatID             int64
	MessageID          int
	Winrate            float64
	PnL                float64
	StartCount         int
	FoundWallets       []*storage.WalletData
	LastProcessedIndex int
	ProcessedCount     int
	LastUpdateTime     time.Time
	CancelRequested    bool
	Active             bool
	MaxCredits         int // Max credits to spend in this session
	CreditsSpent       int // Credits spent so far
	totalWallets       int
	isScanning         bool
	mu                 sync.RWMutex
}

var (
	activeSearches = make(map[int64]*SearchSession)
	searchMu       sync.RWMutex
)

const (
	ScanDelayTrialMin   = 300
	ScanDelayTrialMax   = 600
	ScanDelayNormalMin  = 300
	ScanDelayNormalMax  = 3600
	BatchSize           = 5
	TickerInterval      = 3 * time.Second
	MaxIterations       = 100
	MaxCreditsPerSearch = 200 // Cap credits per search
)

// ... (startDevFinderImproved and handlers remain mostly the same, just setting MaxCredits)

// startRealTimeSearch begins searching and shows results in real-time or queues for slow delivery
func startRealTimeSearch(bot *tgbotapi.BotAPI, chatID int64, winrate, pnl float64, startCount int, scanType string) {
	// Check user plan and credits
	user, err := scanner.db.GetUser(chatID)
	if err != nil {
		log.Printf("Error getting user: %v", err)
		sendError(bot, chatID, "Internal error occurred")
		return
	}

	if user == nil {
		sendWarning(bot, chatID, "Please send /start to register")
		return
	}

	// Enforce Plan Logic
	if user.PlanType == "trial_3day" {
		// 3-Day Trial: Force Slow Scan with 5-10 min delay
		if scanType == "realtime" {
			send(bot, chatID, "⚠️ *3-Day Trial Limitation*\n\nReal-Time scans are not available on the trial plan.\nSwitching to Slow Scan (5-10 min delay).")
			startRealTimeSearch(bot, chatID, winrate, pnl, startCount, "slow")
			return
		}
		// Check expiry
		if time.Now().Unix() > user.TrialExpiresAt {
			sendError(bot, chatID, "Trial Expired\n\nYour 3-Day Free Trial has ended.\nPlease upgrade to continue.")
			return
		}
	} else if user.PlanType == "credits_1000" {
		// Credit Plan: Check balance
		if user.Credits <= 0 {
			sendError(bot, chatID, "Insufficient Credits\n\nYou have 0 credits left.\nPlease purchase more credits to continue.")
			return
		}
		// Deduction happens per result found
		send(bot, chatID, fmt.Sprintf("💎 *Credit Balance*: %d\n1 Credit will be deducted for each wallet found.", user.Credits))
	}

	if scanType == "slow" {
		// ... (Slow scan logic)
		// Start background scan
		go runSlowScan(context.Background(), bot, chatID, winrate, pnl, startCount)
		return
	}

	// Original real-time behavior
	// Create initial progress message with Cancel button
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Cancel Search", fmt.Sprintf("cancel_search_%d", chatID)),
		),
	)

	text := fmt.Sprintf("🔍 *Searching for Wallets...*\n\n"+
		"Filters: WR ≥ %.2f%%, PnL ≥ %.2f%%\n\n"+
		"█░░░░░░░░░░░░░░░░░░░\n"+
		"Progress: 0.0%%\n\n"+
		"📊 Wallets Found: 0\n"+
		"⏱️ Status: Starting...",
		winrate, pnl)

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard
	sentMsg, _ := bot.Send(msg)

	// Create search session
	searchMu.Lock()
	// Create new search session
	search := &SearchSession{
		ChatID:             chatID,
		MessageID:          sentMsg.MessageID,
		Winrate:            winrate,
		PnL:                pnl,
		StartCount:         len(scanner.walletsCache),
		FoundWallets:       make([]*storage.WalletData, 0),
		LastProcessedIndex: 0, // Start from beginning to scan existing wallets
		ProcessedCount:     0,
		LastUpdateTime:     time.Now(),
		Active:             true,
		MaxCredits:         MaxCreditsPerSearch, // Set budget
		CreditsSpent:       0,
	}
	activeSearches[chatID] = search
	searchMu.Unlock()

	// Start the search goroutine
	go runRealTimeSearch(bot, chatID)
}

// runSlowScan performs scan in background and queues results for delayed delivery
func runSlowScan(ctx context.Context, bot *tgbotapi.BotAPI, chatID int64, winrate, pnl float64, startCount int) {
	// Poll for scan completion
	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

loop:
	for {
		select {
		case <-ticker.C:
			scanner.mu.RLock()
			scanning := scanner.isScanning
			scanner.mu.RUnlock()
			if !scanning {
				break loop
			}
		case <-timeout:
			break loop
		case <-ctx.Done():
			return
		}
	}

	// Collect matching wallets
	scanner.mu.RLock()
	var potentialMatches []*storage.WalletData
	for _, w := range scanner.walletsCache {
		if w.Winrate >= winrate && w.RealizedPnL >= pnl {
			potentialMatches = append(potentialMatches, w)
		}
	}
	scanner.mu.RUnlock()

	// Apply Credit Logic Atomically
	var confirmedMatches []*storage.WalletData
	user, _ := scanner.db.GetUser(chatID)

	if user != nil && user.PlanType == "credits_1000" {
		// Deduct 1 credit per wallet
		for _, w := range potentialMatches {
			if err := scanner.db.DecrementUserCredits(chatID, 1); err == nil {
				confirmedMatches = append(confirmedMatches, w)
			} else {
				// Stop if out of credits
				break
			}
		}
	} else {
		// Unlimited or Trial
		confirmedMatches = potentialMatches
	}

	// Generate random delay
	var delaySeconds int
	user, _ = scanner.db.GetUser(chatID) // Refresh user

	if user != nil && user.PlanType == "trial_3day" {
		// 5-10 minutes for trial
		delaySeconds = rand.Intn(ScanDelayTrialMax-ScanDelayTrialMin) + ScanDelayTrialMin
	} else {
		// 5-60 minutes for others
		delaySeconds = rand.Intn(ScanDelayNormalMax-ScanDelayNormalMin) + ScanDelayNormalMin
	}

	deliverAt := time.Now().Add(time.Duration(delaySeconds) * time.Second)

	// Store in pending queue - check for existing scan first
	pendingScansMu.Lock()
	if _, exists := pendingScans[chatID]; exists {
		pendingScansMu.Unlock()
		sendWarning(bot, chatID, "You already have a pending slow scan. This scan was cancelled.")
		return
	}
	pendingScans[chatID] = &PendingScan{
		UserID:      chatID,
		Results:     confirmedMatches,
		DeliverAt:   deliverAt,
		ScanType:    "slow",
		Winrate:     winrate,
		RealizedPnL: pnl,
	}
	pendingScansMu.Unlock()

	// Notify user about estimated wait time
	etaMinutes := delaySeconds / 60
	updateText := fmt.Sprintf("⏱️ *Scan Complete - Results Pending*\n\n"+
		"Found *%d wallets* matching your criteria.\n\n"+
		"Estimated delivery time: *~%d minutes*\n\n"+
		"You'll be notified when results are ready!",
		len(confirmedMatches), etaMinutes)
	send(bot, chatID, updateText)

	// Schedule delayed delivery
	deliverDelayedResults(ctx, bot, chatID, delaySeconds)
}

// runRealTimeSearch continuously searches and updates results
func runRealTimeSearch(bot *tgbotapi.BotAPI, chatID int64) {
	searchMu.RLock()
	search, exists := activeSearches[chatID]
	searchMu.RUnlock()

	if !exists {
		return
	}

	// Jitter to avoid synchronized bursts
	time.Sleep(time.Duration(rand.Intn(1000)) * time.Millisecond)

	ticker := time.NewTicker(TickerInterval)
	defer ticker.Stop()

	lastFoundCount := 0
	iterations := 0
	maxIterations := MaxIterations

	for iterations < maxIterations {
		<-ticker.C
		iterations++

		// Check for cancellation
		search.mu.RLock()
		if search.CancelRequested {
			search.mu.RUnlock()
			sendSearchSummary(bot, chatID, "cancelled")
			return
		}
		// Check budget
		if search.CreditsSpent >= search.MaxCredits {
			search.mu.RUnlock()
			sendSearchSummary(bot, chatID, "budget_limit")
			return
		}
		search.mu.RUnlock()

		// Scan for matching wallets
		scanner.mu.RLock()
		var newMatches []*storage.WalletData

		// Scalable Iteration: Only process wallets we haven't seen yet
		currentLen := len(scanner.walletsList)
		startIndex := search.LastProcessedIndex

		var walletsToProcess []*storage.WalletData
		if startIndex < currentLen {
			walletsToProcess = scanner.walletsList[startIndex:currentLen]
		}
		scanner.mu.RUnlock()

		// Process new wallets
		var validMatches []*storage.WalletData
		for _, w := range walletsToProcess {
			if w.Winrate >= search.Winrate && w.RealizedPnL >= search.PnL {
				validMatches = append(validMatches, w)
			}
		}

		// Batch Credit Deduction
		if len(validMatches) > 0 {
			// Calculate how many we can afford
			creditsNeeded := len(validMatches)

			// Check against session budget
			search.mu.RLock()
			remainingBudget := search.MaxCredits - search.CreditsSpent
			search.mu.RUnlock()

			if creditsNeeded > remainingBudget {
				creditsNeeded = remainingBudget
				validMatches = validMatches[:creditsNeeded]
			}

			if creditsNeeded > 0 {
				// Atomic batch deduction
				if err := scanner.db.DecrementUserCredits(chatID, creditsNeeded); err != nil {
					// Stop search immediately
					search.mu.Lock()
					search.Active = false
					search.mu.Unlock()

					send(bot, chatID, "⚠️ *Search Stopped: Insufficient Credits*\n\n"+
						"You have run out of credits. Please top up to continue searching.")
					return
				}

				// Update spent
				search.mu.Lock()
				search.CreditsSpent += creditsNeeded
				search.mu.Unlock()

				newMatches = validMatches
			}
		}

		search.mu.Lock()
		search.ProcessedCount += len(walletsToProcess)
		search.mu.Unlock()

		// Update Index
		search.mu.Lock()
		search.LastProcessedIndex = currentLen
		if len(newMatches) > 0 {
			search.FoundWallets = append(search.FoundWallets, newMatches...)
		}
		search.mu.Unlock()

		scanner.mu.RLock()
		isScanning := scanner.isScanning
		totalWallets := scanner.totalWallets
		scanner.mu.RUnlock()

		// Add new matches to search session
		if len(newMatches) > 0 {
			// Batch Process New Matches
			var batchMessage strings.Builder
			processedCount := 0

			if len(newMatches) > 0 {
				batchMessage.WriteString(fmt.Sprintf("✨ *New Wallets Found!* (%d)\n\n", len(newMatches)))

				for i, wallet := range newMatches {
					// Add to batch message
					batchMessage.WriteString(fmt.Sprintf("*%d.* `%s`\n", i+1, wallet.Wallet))
					batchMessage.WriteString(fmt.Sprintf("💹 WR: %.2f%% | 💰 PnL: %.2f%%\n\n", wallet.Winrate, wallet.RealizedPnL))
					processedCount++
				}

				// Send the batch message
				if processedCount > 0 {
					batchMessage.WriteString("🔍 Meets your criteria")
					send(bot, chatID, batchMessage.String())
				}
			}
		}

		// Update progress message
		search.mu.RLock()
		foundCount := len(search.FoundWallets)
		processedTotal := search.ProcessedCount
		search.mu.RUnlock()

		if foundCount != lastFoundCount || iterations%5 == 0 { // Update every 15 seconds or when new wallet found
			updateSearchProgress(bot, search, processedTotal, totalWallets, isScanning)
			lastFoundCount = foundCount
		}

		// Auto-Stop Logic
		if !isScanning && iterations > 20 && foundCount == 0 {
			break
		}
	}

	// Send final summary
	sendSearchSummary(bot, chatID, "exhausted")
}

// updateSearchProgress updates the progress message
// updateSearchProgress updates the progress message
func updateSearchProgress(bot *tgbotapi.BotAPI, search *SearchSession, processedTotal, totalWallets int, isScanning bool) {
	search.mu.RLock()
	defer search.mu.RUnlock()

	if !search.Active {
		return
	}

	progress := 0.0
	if totalWallets > 0 {
		// Estimate progress based on processed count vs total wallets
		// Since we don't know exactly where we are in the map iteration, this is an approximation
		// or we can just show processed count.
		// For now, let's just cap it at 100% if it goes over (which it shouldn't if totalWallets is correct)
		progress = float64(processedTotal) / float64(totalWallets) * 100
		if progress > 100 {
			progress = 100
		}
	}

	progressBar := createProgressBar(progress)
	foundCount := len(search.FoundWallets)

	text := fmt.Sprintf("🔍 *Searching for Wallets...*\n\n"+
		"Filters: WR ≥ %.2f%%, PnL ≥ %.2f%%\n\n"+
		"%s\n"+
		"Progress: %.1f%%\n\n"+
		"✅ Wallets Found: *%d*\n"+
		"📊 Wallets Processed: *%d*\n"+
		"⏱️ Status: %s",
		search.Winrate, search.PnL, progressBar, progress,
		foundCount, processedTotal,
		map[bool]string{true: "Scanning...", false: "Waiting"}[isScanning])

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Cancel Search", fmt.Sprintf("cancel_search_%d", search.ChatID)),
		),
	)

	edit := tgbotapi.NewEditMessageText(search.ChatID, search.MessageID, text)
	edit.ParseMode = "Markdown"
	edit.ReplyMarkup = &keyboard
	bot.Send(edit)
}

// handleCancelSearch handles the cancel button press
func handleCancelSearch(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {
	chatID := query.Message.Chat.ID

	// Send confirmation message
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Yes, Cancel", fmt.Sprintf("confirm_cancel_%d", chatID)),
			tgbotapi.NewInlineKeyboardButtonData("❌ No, Continue", fmt.Sprintf("continue_search_%d", chatID)),
		),
	)

	searchMu.RLock()
	search, exists := activeSearches[chatID]
	searchMu.RUnlock()

	if !exists {
		bot.Send(tgbotapi.NewCallback(query.ID, "Search not found"))
		return
	}

	search.mu.RLock()
	foundCount := len(search.FoundWallets)
	search.mu.RUnlock()

	text := fmt.Sprintf("⚠️ *Cancel Search?*\n\n"+
		"You have found *%d wallets* so far.\n\n"+
		"Do you want to cancel and receive these results?",
		foundCount)

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard
	bot.Send(msg)

	bot.Send(tgbotapi.NewCallback(query.ID, ""))
}

// handleConfirmCancel handles the confirmation of cancellation
func handleConfirmCancel(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {
	chatID := query.Message.Chat.ID

	searchMu.Lock()
	search, exists := activeSearches[chatID]
	if exists {
		search.mu.Lock()
		search.Active = false
		search.CancelRequested = true
		search.mu.Unlock()
	}
	searchMu.Unlock()

	if !exists {
		bot.Send(tgbotapi.NewCallback(query.ID, "Search not found"))
		return
	}

	// Delete confirmation message
	deleteMsg := tgbotapi.NewDeleteMessage(chatID, query.Message.MessageID)
	bot.Send(deleteMsg)

	// Send results
	sendSearchSummary(bot, chatID, "cancelled")

	bot.Send(tgbotapi.NewCallback(query.ID, "Search cancelled"))
}

// handleContinueSearch handles continuing the search
func handleContinueSearch(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {
	chatID := query.Message.Chat.ID

	// Delete confirmation message
	deleteMsg := tgbotapi.NewDeleteMessage(chatID, query.Message.MessageID)
	bot.Send(deleteMsg)

	bot.Send(tgbotapi.NewCallback(query.ID, "Continuing search..."))
}

// sendSearchSummary sends final summary of found wallets
func sendSearchSummary(bot *tgbotapi.BotAPI, chatID int64, stopReason string) {
	searchMu.Lock()
	search, exists := activeSearches[chatID]
	if exists {
		search.mu.Lock()
		search.Active = false
		search.mu.Unlock()
	}
	searchMu.Unlock()

	if !exists {
		return
	}

	search.mu.RLock()
	foundWallets := search.FoundWallets
	winrate := search.Winrate
	pnl := search.PnL
	search.mu.RUnlock()

	// Remove from active searches
	searchMu.Lock()
	delete(activeSearches, chatID)
	searchMu.Unlock()

	// Send summary
	statusIcon := "✅"
	statusText := "Search Complete"

	if stopReason == "cancelled" {
		statusIcon = "🛑"
		statusText = "Search Stopped by User"
	} else if stopReason == "exhausted" {
		statusIcon = "✅"
		statusText = "Search Complete (Source Exhausted)"
	} else if stopReason == "credits" {
		statusIcon = "⚠️"
		statusText = "Search Stopped (Insufficient Credits)"
	}

	if len(foundWallets) == 0 {
		text := fmt.Sprintf("%s *%s*\n\n"+
			"Filters: WR ≥ %.2f%%, PnL ≥ %.2f%%\n\n"+
			"❌ No wallets found matching your criteria.\n\n"+
			"Try lowering your filters or wait for the next scan cycle.",
			statusIcon, statusText, winrate, pnl)
		send(bot, chatID, text)
		return
	}

	// Send header
	headerText := fmt.Sprintf("%s *%s*\n\n"+
		"Filters: WR ≥ %.2f%%, PnL ≥ %.2f%%\n\n"+
		"✅ Found *%d wallets* matching your criteria!\n\n"+
		"━━━━━━━━━━━━━━━━━━━━",
		statusIcon, statusText, winrate, pnl, len(foundWallets))
	send(bot, chatID, headerText)

	// Send wallets in batches
	batchSize := BatchSize
	for i := 0; i < len(foundWallets); i += batchSize {
		end := i + batchSize
		if end > len(foundWallets) {
			end = len(foundWallets)
		}

		text := ""
		for j := i; j < end; j++ {
			w := foundWallets[j]
			text += fmt.Sprintf("*Wallet %d*\n"+
				"`%s`\n"+
				"💹 WR: %.2f%% | 💰 PnL: %.2f%%\n\n",
				j+1, w.Wallet, w.Winrate, w.RealizedPnL)
		}

		if i+batchSize >= len(foundWallets) {
			text += "━━━━━━━━━━━━━━━━━━━━\n" +
				"🎉 End of results"
		}

		send(bot, chatID, text)
		time.Sleep(500 * time.Millisecond) // Avoid rate limiting
	}
}

// Helper function to parse float with validation
func parseFloatV2(text string, min, max float64) (float64, error) {
	val, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0, err
	}
	if val < min || val > max {
		return 0, fmt.Errorf("value out of range")
	}
	return val, nil
}

// Wrapper functions to integrate with telegram-bot.go

func startDevFinderImproved(bot *tgbotapi.BotAPI, chatID int64) {
	showScanTypeModal(bot, chatID)
}

func startDevFinderImprovedWithType(bot *tgbotapi.BotAPI, chatID int64, scanType string) {
	sessMu.Lock()
	sessions[chatID] = &UserSession{
		State:       "awaiting_winrate_v2",
		RequestedAt: time.Now().Unix(),
		ScanType:    scanType,
	}
	sessMu.Unlock()

	send(bot, chatID, "🎯 *Dev Finder V2*\n\nEnter minimum *Win Rate* (25-100):")
}

func handleWinrateInputV2(bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	chatID := msg.Chat.ID
	winrate, err := strconv.ParseFloat(msg.Text, 64)

	if err != nil || winrate < 25 || winrate > 100 {
		sendError(bot, chatID, "Invalid input. Enter 25-100")
		return
	}

	sessMu.Lock()
	if sessions[chatID] == nil {
		sessions[chatID] = &UserSession{}
	}
	sessions[chatID].State = "awaiting_pnl_v2"
	sessions[chatID].Winrate = winrate
	sessMu.Unlock()

	send(bot, chatID, "✅ Enter minimum *PnL* (e.g. 100):")
}

func handlePnlInputV2(bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	chatID := msg.Chat.ID
	pnl, err := strconv.ParseFloat(msg.Text, 64)

	if err != nil || pnl < 25 {
		sendError(bot, chatID, "Invalid input.")
		return
	}

	sessMu.Lock()
	session := sessions[chatID]
	if session == nil {
		sessMu.Unlock()
		sendError(bot, chatID, "Session expired.")
		return
	}
	winrate := session.Winrate
	scanType := session.ScanType
	delete(sessions, chatID)
	sessMu.Unlock()

	startRealTimeSearch(bot, chatID, winrate, pnl, 0, scanType)
}

func deliverDelayedResults(ctx context.Context, bot *tgbotapi.BotAPI, chatID int64, delaySeconds int) {
	go func() {
		select {
		case <-time.After(time.Duration(delaySeconds) * time.Second):
			deliverPendingScanResults(bot, chatID)
		case <-ctx.Done():
			return
		}
	}()
}

func deliverPendingScanResults(bot *tgbotapi.BotAPI, chatID int64) {
	pendingScansMu.Lock()
	scan, exists := pendingScans[chatID]
	if !exists {
		pendingScansMu.Unlock()
		return
	}
	delete(pendingScans, chatID)
	pendingScansMu.Unlock()

	if len(scan.Results) == 0 {
		send(bot, chatID, "❌ Slow Scan Complete: No wallets found.")
		return
	}

	send(bot, chatID, fmt.Sprintf("✅ *Slow Scan Complete*\n\nFound %d wallets matching your criteria!", len(scan.Results)))

	// Send in batches
	batchSize := BatchSize
	for i := 0; i < len(scan.Results); i += batchSize {
		end := i + batchSize
		if end > len(scan.Results) {
			end = len(scan.Results)
		}

		text := ""
		for j := i; j < end; j++ {
			w := scan.Results[j]
			text += fmt.Sprintf("*Wallet %d*\n`%s`\n💹 WR: %.2f%% | 💰 PnL: %.2f%%\n\n", j+1, w.Wallet, w.Winrate, w.RealizedPnL)
		}
		send(bot, chatID, text)
	}
}
