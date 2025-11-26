package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"solana-orchestrator/analyzer"
	"solana-orchestrator/api"
	"solana-orchestrator/config"

	"github.com/fatih/color"
)

func main() {
	limit := flag.Int("limit", 100, "Number of tokens")
	pages := flag.Int("pages", 3, "Concurrent pages")
	configPath := flag.String("config", "config/config.json", "Config path")
	flag.Parse()

	cyan := color.New(color.FgCyan, color.Bold)
	green := color.New(color.FgGreen, color.Bold)
	yellow := color.New(color.FgYellow)

	cyan.Println("\n" + strings.Repeat("=", 80))
	cyan.Println("🚀 SOLANA ORCHESTRATOR (Go)")
	cyan.Println(strings.Repeat("=", 80) + "\n")

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Config error: %v", err)
	}

	client := api.NewClient(cfg.MoralisAPIKey, cfg.BirdeyeAPIKey, cfg.APISettings.MaxRetries, cfg.MoralisFallbackKeys)

	yellow.Println("📊 Fetching tokens...")
	tokens, err := client.FetchBirdeyeTokens(*limit)
	if err != nil {
		log.Fatalf("Token fetch failed: %v", err)
	}
	green.Printf("✅ %d tokens\n\n", len(tokens))

	holdersMap := make(map[string][]api.Holder)
	walletSet := make(map[string]bool)

	cyan.Println("👥 Collecting holders...")
	for i, token := range tokens {
		fmt.Printf("\r[%d/%d] %s", i+1, len(tokens), token.TokenAddress[:8])

		holders, err := client.GetTokenHolders(token.TokenAddress)
		if err != nil {
			fmt.Printf(" ❌ Error: %v\n", err)
			continue
		}

		if len(holders) == 0 {
			fmt.Printf(" ⚠️  No holders\n")
			continue
		}

		holdersMap[token.TokenAddress] = holders
		for _, h := range holders {
			walletSet[h.OwnerAddress] = true
		}

		time.Sleep(2 * time.Second)
	}
	fmt.Println()

	wallets := make([]string, 0, len(walletSet))
	for w := range walletSet {
		wallets = append(wallets, w)
	}

	green.Printf("✅ %d wallets\n\n", len(wallets))

	os.MkdirAll("data", 0755)
	saveJSON("data/tokens.json", tokens)
	saveJSON("data/holders.json", holdersMap)

	cyan.Println("🔍 Analyzing wallets...")
	a := analyzer.NewAnalyzer(*pages, cfg.AnalysisFilters.MinWinrate, cfg.AnalysisFilters.MinRealizedPnL)

	goodWallets, err := a.AnalyzeWallets(context.Background(), wallets, func(stats *analyzer.WalletStats) {
		// Callback for progress updates (optional)
		log.Printf("Analyzed wallet: %s", stats.Wallet)
	})
	if err != nil {
		log.Fatalf("Failed to analyze wallets: %v", err)
	}

	log.Printf("\nAnalysis complete: %d wallets analyzed", len(goodWallets))

	fmt.Println()
	green.Printf("✅ %d profitable wallets\n", len(goodWallets))

	saveJSON("data/good_wallets.json", goodWallets)

	cyan.Println("\n" + strings.Repeat("=", 80))
	green.Println("🎉 COMPLETE!")
	cyan.Println(strings.Repeat("=", 80) + "\n")
}

func saveJSON(path string, data interface{}) {
	file, _ := os.Create(path)
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	encoder.Encode(data)
}
