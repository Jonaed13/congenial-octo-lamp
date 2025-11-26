package analyzer

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/playwright-community/playwright-go"
)

const (
	DefaultPageTimeout      = 30000.0
	DefaultSelectorTimeout  = 10000.0
	DefaultLoadStateTimeout = 15000.0
	MaxWalletsPerScan       = 50 // Limit wallets per scan cycle
)

type WalletStats struct {
	Wallet      string  `json:"wallet"`
	Winrate     float64 `json:"winrate"`
	RealizedPnL float64 `json:"realized_pnl"`
}

type Analyzer struct {
	numPages       int
	minWinrate     float64
	minRealizedPnL float64
	scannedWallets sync.Map
}

func NewAnalyzer(numPages int, minWinrate, minRealizedPnL float64) *Analyzer {
	return &Analyzer{
		numPages:       numPages,
		minWinrate:     minWinrate,
		minRealizedPnL: minRealizedPnL,
	}
}

func (a *Analyzer) AnalyzeWallets(ctx context.Context, wallets []string, onResult func(*WalletStats)) ([]WalletStats, error) {
	// Limit wallets to process
	if len(wallets) > MaxWalletsPerScan {
		log.Printf("⚠️ Limiting scan to %d wallets (requested %d)", MaxWalletsPerScan, len(wallets))
		wallets = wallets[:MaxWalletsPerScan]
	}

	pw, err := playwright.Run()
	if err != nil {
		return nil, err
	}
	defer pw.Stop()

	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true),
	})
	if err != nil {
		return nil, err
	}
	defer browser.Close()

	var results []WalletStats
	var mu sync.Mutex
	var wg sync.WaitGroup

	walletChan := make(chan string, len(wallets))
	for _, w := range wallets {
		walletChan <- w
	}
	close(walletChan)

	for i := 0; i < a.numPages; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			// Create a reusable page for this worker
			page, err := browser.NewPage()
			if err != nil {
				log.Printf("❌ Worker %d: Failed to create page: %v", workerID, err)
				return
			}
			defer page.Close()

			for wallet := range walletChan {
				// Check context cancellation
				select {
				case <-ctx.Done():
					return
				default:
				}

				stats, err := a.analyzeSingleWallet(ctx, page, wallet)

				if err != nil {
					// If error is related to page crash, try to recreate page
					if strings.Contains(err.Error(), "Target closed") || strings.Contains(err.Error(), "Connection closed") {
						log.Printf("⚠️ Worker %d: Page crashed, recreating...", workerID)
						page.Close()
						page, err = browser.NewPage()
						if err != nil {
							log.Printf("❌ Worker %d: Failed to recreate page: %v", workerID, err)
							return
						}
					} else {
						log.Printf("❌ Worker %d: Error analyzing %s: %v", workerID, wallet, err)
					}
					continue
				}

				if stats != nil {
					mu.Lock()
					results = append(results, *stats)
					mu.Unlock()

					if onResult != nil {
						onResult(stats)
					}
					log.Printf("✅ Worker %d: %s - WR: %.2f%%, PnL: %.2f%%", workerID, wallet[:8], stats.Winrate, stats.RealizedPnL)
				}
			}
		}(i)
	}

	wg.Wait()
	return results, nil
}

// analyzeSingleWallet analyzes a single wallet using the provided page
func (a *Analyzer) analyzeSingleWallet(ctx context.Context, page playwright.Page, wallet string) (*WalletStats, error) {
	// Check context before starting
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Navigate to DexCheck
	url := fmt.Sprintf("https://dexcheck.ai/app/wallet-analyzer/%s", wallet)
	if _, err := page.Goto(url, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		Timeout:   playwright.Float(DefaultPageTimeout),
	}); err != nil {
		return nil, fmt.Errorf("failed to navigate: %w", err)
	}

	// Wait for key elements that indicate data is loaded
	_, err := page.WaitForSelector("div:has-text('Win Rate')", playwright.PageWaitForSelectorOptions{
		Timeout: playwright.Float(DefaultSelectorTimeout),
	})

	// Check context during wait
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	if err != nil {
		// Check if it's an invalid wallet or no data
		if content, _ := page.Content(); strings.Contains(content, "No data found") {
			return nil, fmt.Errorf("no data found")
		}
		return nil, fmt.Errorf("timeout waiting for data")
	}

	// Additional wait to ensure loading SVGs are replaced with actual data
	page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State:   playwright.LoadStateNetworkidle,
		Timeout: playwright.Float(DefaultLoadStateTimeout),
	})

	// Get the full HTML content
	html, err := page.Content()
	if err != nil {
		return nil, fmt.Errorf("failed to get page content: %w", err)
	}

	// Check if page still has loading indicators
	if strings.Contains(html, "Loading...</title>") {
		// Wait a bit more and retry
		page.WaitForTimeout(2000)
		html, err = page.Content()
		if err != nil {
			return nil, fmt.Errorf("failed to get page content after retry: %w", err)
		}
	}

	// Extract WR and PnL using the helper functions
	winrate := extractWinrate(html)
	realizedPnL := extractRealizedPnL(html)

	// Observability for 0 values
	if winrate == 0 && realizedPnL == 0 {
		log.Printf("⚠️ Worker: Zero metrics extracted for %s. HTML snippet: %s", wallet, html[:min(len(html), 200)])
	}

	// Check if wallet meets the minimum criteria
	if winrate < a.minWinrate {
		return nil, fmt.Errorf("winrate %.2f%% below minimum %.2f%%", winrate, a.minWinrate)
	}
	if realizedPnL < a.minRealizedPnL {
		return nil, fmt.Errorf("realized PnL %.2f%% below minimum %.2f%%", realizedPnL, a.minRealizedPnL)
	}

	return &WalletStats{
		Wallet:      wallet,
		Winrate:     winrate,
		RealizedPnL: realizedPnL,
	}, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func extractWinrate(html string) float64 {
	// Match: <h3...>Win Rate</h3><p class="...text-2xl...">XX.XX%</p>
	re := regexp.MustCompile(`(?i)Win Rate</h3><p[^>]*text-2xl[^>]*>([\d\.]+)%`)
	if matches := re.FindStringSubmatch(html); len(matches) > 1 {
		val, err := strconv.ParseFloat(matches[1], 64)
		if err != nil {
			log.Printf("⚠️ Failed to parse WR value '%s': %v", matches[1], err)
			return 0
		}
		return val
	}
	return 0
}

func extractRealizedPnL(html string) float64 {
	// Match: Realized</p><p...>$XXX <span...>(Y.YY%)</span> or (-Y.YY%)
	// Handles -$XXX and $XXX
	re := regexp.MustCompile(`(?i)Realized</p><p[^>]*>-?\$[\d,\.]+\s*<span[^>]*>\((-?[\d\.]+)%\)</span>`)
	if matches := re.FindStringSubmatch(html); len(matches) > 1 {
		val, err := strconv.ParseFloat(matches[1], 64)
		if err != nil {
			log.Printf("⚠️ Failed to parse PnL value '%s': %v", matches[1], err)
			return 0
		}
		return val
	}

	// Debug logging for missed PnL
	if len(html) > 500 {
		// Find "Realized" and print context
		reDebug := regexp.MustCompile(`(?i)Realized.{0,200}`)
		if match := reDebug.FindString(html); match != "" {
			fmt.Printf("⚠️ PnL Missed: %s\n", match)
		}
	}
	return 0
}
