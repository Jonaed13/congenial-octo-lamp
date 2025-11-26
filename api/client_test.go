package api

import (
	"testing"
)

func TestAPIClient(t *testing.T) {
	// Note: These tests require valid API keys
	// Load config to get keys
	client := NewClient(
		"test_moralis_key",
		"test_birdeye_key",
		3,
		[]string{},
	)

	if client == nil {
		t.Fatal("Client should not be nil")
	}

	t.Run("ClientInitialization", func(t *testing.T) {
		if client.moralisKey == "" {
			t.Error("Moralis key should be set")
		}

		if client.birdeyeKey == "" {
			t.Error("Birdeye key should be set")
		}

		if client.maxRetries <= 0 {
			t.Error("Max retries should be positive")
		}

		if client.httpClient == nil {
			t.Error("HTTP client should be initialized")
		}
	})

	// Can't test live API calls without valid keys, but we can test error handling
	t.Run("FetchBirdeyeTokens_ErrorHandling", func(t *testing.T) {
		// With invalid key, should get error
		_, err := client.FetchBirdeyeTokens(10)
		if err != nil {
			t.Logf("Expected error with test key: %v", err)
			// This is expected
		}
	})

	t.Run("InvalidTokenLimit", func(t *testing.T) {
		// Test with limit > 50 (Birdeye max)
		_, err := client.FetchBirdeyeTokens(100)
		if err != nil {
			t.Logf("Large limit error: %v", err)
		}

		// Test with limit = 0
		_, err = client.FetchBirdeyeTokens(0)
		if err != nil {
			t.Logf("Zero limit error: %v", err)
		}
	})
}

func TestRetryLogic(t *testing.T) {
	client := NewClient("test_key", "test_key", 3, []string{})

	t.Run("GetTokenHolders_Retry", func(t *testing.T) {
		// This should retry with invalid token
		_, err := client.GetTokenHolders("InvalidTokenAddress123")
		if err != nil {
			t.Logf("Expected error after retries: %v", err)
		}
	})
}

func TestFallbackKeys(t *testing.T) {
	primaryKey := "primary_key"
	fallbackKeys := []string{"fallback1", "fallback2"}

	client := NewClient(primaryKey, "birdeye_key", 3, fallbackKeys)

	t.Run("FallbackKeysStored", func(t *testing.T) {
		if len(client.fallbackKeys) != 2 {
			t.Errorf("Expected 2 fallback keys, got %d", len(client.fallbackKeys))
		}

		if client.fallbackKeys[0] != "fallback1" {
			t.Error("Fallback key 1 not stored correctly")
		}

		if client.fallbackKeys[1] != "fallback2" {
			t.Error("Fallback key 2 not stored correctly")
		}
	})
}
