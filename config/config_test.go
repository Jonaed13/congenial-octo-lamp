package config

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Test loading valid config
	t.Run("LoadValidConfig", func(t *testing.T) {
		cfg, err := Load("config.json")
		if err != nil {
			t.Fatalf("Failed to load config: %v", err)
		}

		if cfg.MoralisAPIKey == "" {
			t.Error("Moralis API key is empty")
		}

		if cfg.BirdeyeAPIKey == "" {
			t.Error("Birdeye API key is empty")
		}

		if cfg.AnalysisFilters.MinWinrate < 0 {
			t.Error("MinWinrate should not be negative")
		}

		if cfg.APISettings.TokenLimit <= 0 {
			t.Error("TokenLimit should be positive")
		}

		t.Logf("Config loaded: TokenSource=%s, TokenLimit=%d",
			cfg.APISettings.TokenSource, cfg.APISettings.TokenLimit)
	})

	// Test loading non-existent config
	t.Run("LoadNonExistentConfig", func(t *testing.T) {
		_, err := Load("nonexistent.json")
		if err == nil {
			t.Error("Should fail when loading non-existent config")
		}
	})

	// Test invalid JSON
	t.Run("LoadInvalidJSON", func(t *testing.T) {
		// Create temp file with invalid JSON
		tmpfile, err := os.CreateTemp("", "invalid_*.json")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(tmpfile.Name())

		tmpfile.WriteString("{invalid json")
		tmpfile.Close()

		_, err = Load(tmpfile.Name())
		if err == nil {
			t.Error("Should fail when loading invalid JSON")
		}
	})
}

func TestConfigValidation(t *testing.T) {
	cfg, err := Load("config.json")
	if err != nil {
		t.Skip("Skipping validation test - config not found")
	}

	t.Run("TradingSettings", func(t *testing.T) {
		if cfg.TradingSettings.JitoTipLamports < 0 {
			t.Error("Jito tip should not be negative")
		}

		if cfg.TradingSettings.DefaultSlippageBps < 0 || cfg.TradingSettings.DefaultSlippageBps > 10000 {
			t.Errorf("Default slippage %d is out of valid range (0-10000 bps)", cfg.TradingSettings.DefaultSlippageBps)
		}

		if cfg.TradingSettings.MaxSlippageBps < cfg.TradingSettings.DefaultSlippageBps {
			t.Error("Max slippage should be >= default slippage")
		}
	})

	t.Run("WebSocketSettings", func(t *testing.T) {
		if cfg.WebSocketSettings.ShyftWSURL == "" {
			t.Error("WebSocket URL should not be empty")
		}

		if cfg.WebSocketSettings.ReconnectDelayMs <= 0 {
			t.Error("Reconnect delay should be positive")
		}
	})

	t.Run("RateLimits", func(t *testing.T) {
		if cfg.RateLimits.ShyftRPS <= 0 {
			t.Error("Shyft RPS should be positive")
		}

		if cfg.RateLimits.ShyftAPIPerSec <= 0 {
			t.Error("Shyft API per sec should be positive")
		}
	})
}
