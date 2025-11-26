package trading

import (
	"context"
	"testing"
	"time"

	"github.com/gagliardetto/solana-go"
)

// TestWebSocketClient tests the WebSocket client functionality
func TestWebSocketClient(t *testing.T) {
	wsURL := "wss://rpc.shyft.to?api_key=48KZbYxP-9e9SpqR"
	client := NewWSClient(wsURL)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Test connection
	t.Run("Connect", func(t *testing.T) {
		err := client.Connect(ctx)
		if err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}

		if !client.IsConnected() {
			t.Error("Client should be connected")
		}
	})

	// Test subscription
	t.Run("Subscribe", func(t *testing.T) {
		testWallet := "G4vTBDnAbBre4wqTpibXbLmwdVtFAbFCr2DM8t22UrmM"

		ch, err := client.SubscribeAccount(ctx, testWallet)
		if err != nil {
			t.Fatalf("Failed to subscribe: %v", err)
		}

		if ch == nil {
			t.Error("Subscription channel should not be nil")
		}
	})

	// Test cleanup
	t.Run("Close", func(t *testing.T) {
		err := client.Close()
		if err != nil {
			t.Errorf("Failed to close: %v", err)
		}
	})
}

// TestBalanceManager tests balance checking functionality
func TestBalanceManager(t *testing.T) {
	rpcURL := "https://rpc.shyft.to?api_key=48KZbYxP-9e9SpqR"
	wsClient := NewWSClient("wss://rpc.shyft.to?api_key=48KZbYxP-9e9SpqR")

	balanceMgr := NewBalanceManager(rpcURL, wsClient)

	testWallet := solana.MustPublicKeyFromBase58("G4vTBDnAbBre4wqTpibXbLmwdVtFAbFCr2DM8t22UrmM")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Run("GetSOLBalance", func(t *testing.T) {
		balance, err := balanceMgr.GetSOLBalance(ctx, testWallet)
		if err != nil {
			t.Fatalf("Failed to get SOL balance: %v", err)
		}

		t.Logf("SOL Balance: %d lamports (%.9f SOL)", balance, FormatSOL(balance))

		if balance < 0 {
			t.Error("Balance should not be negative")
		}
	})

	t.Run("GetTokenBalances", func(t *testing.T) {
		tokens, err := balanceMgr.GetTokenBalances(ctx, testWallet)
		if err != nil {
			t.Logf("Token balance fetch error (expected for free plan): %v", err)
			// Don't fail - Shyft free plan doesn't support this
			return
		}

		t.Logf("Found %d token accounts", len(tokens))
	})

	t.Run("GetFullBalance", func(t *testing.T) {
		fullBalance, err := balanceMgr.GetFullBalance(ctx, testWallet)
		if err != nil {
			t.Fatalf("Failed to get full balance: %v", err)
		}

		if fullBalance.Wallet != testWallet {
			t.Error("Wallet mismatch in balance result")
		}

		t.Logf("Full Balance - SOL: %.9f, Tokens: %d",
			FormatSOL(fullBalance.SOLBalance),
			len(fullBalance.TokenBalances))
	})
}

// TestJitoClient tests Jito integration
func TestJitoClient(t *testing.T) {
	jitoURL := "https://mainnet.block-engine.jito.wtf"
	tipAmount := uint64(10000)

	client := NewJitoClient(jitoURL, tipAmount)

	t.Run("GetTipAmount", func(t *testing.T) {
		tip := client.GetTipAmount()
		if tip != tipAmount {
			t.Errorf("Expected tip %d, got %d", tipAmount, tip)
		}
	})

	t.Run("SetTipAmount", func(t *testing.T) {
		newTip := uint64(20000)
		client.SetTipAmount(newTip)

		if client.GetTipAmount() != newTip {
			t.Error("Tip amount not updated correctly")
		}
	})

	t.Run("CreateTipInstruction", func(t *testing.T) {
		feePayer := solana.MustPublicKeyFromBase58("G4vTBDnAbBre4wqTpibXbLmwdVtFAbFCr2DM8t22UrmM")

		instruction, err := client.CreateTipInstruction(feePayer)
		if err != nil {
			t.Fatalf("Failed to create tip instruction: %v", err)
		}

		if instruction.ProgramID() != solana.SystemProgramID {
			t.Error("Tip instruction should use System Program")
		}

		if len(instruction.Accounts()) != 2 {
			t.Errorf("Expected 2 accounts, got %d", len(instruction.Accounts()))
		}
	})
}

// TestRateLimiting tests rate limiter functionality
func TestRateLimiting(t *testing.T) {
	wsClient := NewWSClient("wss://rpc.shyft.to?api_key=48KZbYxP-9e9SpqR")

	t.Run("RPS Limit", func(t *testing.T) {
		ctx := context.Background()
		start := time.Now()

		// Try to make 25 requests (should be limited to 20/sec)
		for i := 0; i < 25; i++ {
			err := wsClient.rpsLimiter.Wait(ctx)
			if err != nil {
				t.Fatalf("Rate limiter error: %v", err)
			}
		}

		elapsed := time.Since(start)

		// Should take at least 1 second for 25 requests at 20 RPS
		if elapsed < time.Second {
			t.Errorf("Rate limiting not working properly: completed in %v", elapsed)
		}

		t.Logf("25 requests completed in %v (expected >1s)", elapsed)
	})

	t.Run("API Limit", func(t *testing.T) {
		ctx := context.Background()
		start := time.Now()

		// Try to make 3 API calls (should be limited to 1/sec)
		for i := 0; i < 3; i++ {
			err := wsClient.apiLimiter.Wait(ctx)
			if err != nil {
				t.Fatalf("API limiter error: %v", err)
			}
		}

		elapsed := time.Since(start)

		// Should take at least 2 seconds for 3 calls at 1/sec
		if elapsed < 2*time.Second {
			t.Errorf("API rate limiting not working: completed in %v", elapsed)
		}

		t.Logf("3 API calls completed in %v (expected >2s)", elapsed)
	})
}

// TestFormatSOL tests SOL formatting
func TestFormatSOL(t *testing.T) {
	tests := []struct {
		lamports uint64
		expected float64
	}{
		{1000000000, 1.0},
		{500000000, 0.5},
		{0, 0.0},
		{123456789, 0.123456789},
	}

	for _, tt := range tests {
		result := FormatSOL(tt.lamports)
		if result != tt.expected {
			t.Errorf("FormatSOL(%d) = %f, expected %f", tt.lamports, result, tt.expected)
		}
	}
}
