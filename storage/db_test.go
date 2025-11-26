package storage

import (
	"os"
	"testing"
	"time"
)

func TestDatabaseOperations(t *testing.T) {
	// Create temporary database
	tmpfile, err := os.CreateTemp("", "test_*.db")
	if err != nil {
		t.Fatal(err)
	}
	dbPath := tmpfile.Name()
	tmpfile.Close()
	defer os.Remove(dbPath)

	// Initialize database
	db, err := New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	t.Run("SaveWallet", func(t *testing.T) {
		wallet := &WalletData{
			Wallet:      "TestWallet123",
			Winrate:     75.5,
			RealizedPnL: 125.75,
			ScannedAt:   time.Now().Unix(),
		}

		err := db.SaveWallet(wallet)
		if err != nil {
			t.Fatalf("Failed to save wallet: %v", err)
		}

		// Try saving same wallet again (should update)
		wallet.Winrate = 80.0
		err = db.SaveWallet(wallet)
		if err != nil {
			t.Fatalf("Failed to update wallet: %v", err)
		}
	})

	t.Run("GetWallets", func(t *testing.T) {
		wallets, err := db.GetWallets()
		if err != nil {
			t.Fatalf("Failed to get wallets: %v", err)
		}

		if len(wallets) == 0 {
			t.Error("Should have at least one wallet")
		}

		// Check if wallets are sorted by PnL descending
		for i := 1; i < len(wallets); i++ {
			if wallets[i-1].RealizedPnL < wallets[i].RealizedPnL {
				t.Error("Wallets should be sorted by PnL descending")
			}
		}

		t.Logf("Retrieved %d wallets", len(wallets))
	})

	t.Run("CleanupOldData", func(t *testing.T) {
		// Add old wallet
		oldWallet := &WalletData{
			Wallet:      "OldWallet",
			Winrate:     50.0,
			RealizedPnL: 30.0,
			ScannedAt:   time.Now().Add(-6 * time.Hour).Unix(),
		}
		db.SaveWallet(oldWallet)

		// Add recent wallet
		recentWallet := &WalletData{
			Wallet:      "RecentWallet",
			Winrate:     60.0,
			RealizedPnL: 40.0,
			ScannedAt:   time.Now().Unix(),
		}
		db.SaveWallet(recentWallet)

		// Cleanup
		deleted, err := db.CleanupOldData()
		if err != nil {
			t.Fatalf("Cleanup failed: %v", err)
		}

		if deleted == 0 {
			t.Error("Should have deleted at least one old wallet")
		}

		t.Logf("Deleted %d old wallets", deleted)

		// Verify old wallet is gone, recent wallet remains
		wallets, _ := db.GetWallets()
		for _, w := range wallets {
			if w.Wallet == "OldWallet" {
				t.Error("Old wallet should have been deleted")
			}
		}
	})

	t.Run("SaveWallet_InvalidData", func(t *testing.T) {
		// Test with empty wallet address
		wallet := &WalletData{
			Wallet:      "",
			Winrate:     75.5,
			RealizedPnL: 125.75,
			ScannedAt:   time.Now().Unix(),
		}

		err := db.SaveWallet(wallet)
		// Should this fail? Let's see what happens
		if err != nil {
			t.Logf("Empty wallet address error: %v", err)
		}
	})

	t.Run("ConcurrentWrites", func(t *testing.T) {
		// Test concurrent writes to check for race conditions
		done := make(chan bool)

		for i := 0; i < 10; i++ {
			go func(id int) {
				wallet := &WalletData{
					Wallet:      "ConcurrentWallet" + string(rune(id)),
					Winrate:     float64(50 + id),
					RealizedPnL: float64(100 + id),
					ScannedAt:   time.Now().Unix(),
				}
				db.SaveWallet(wallet)
				done <- true
			}(i)
		}

		// Wait for all goroutines
		for i := 0; i < 10; i++ {
			<-done
		}

		wallets, _ := db.GetWallets()
		t.Logf("After concurrent writes: %d wallets", len(wallets))
	})
}
