package crypto

import (
	"testing"
)

func TestGenerateWallet(t *testing.T) {
	wallet, err := GenerateWallet()
	if err != nil {
		t.Fatalf("Wallet generation failed: %v", err)
	}

	// Check public key
	if wallet.PublicKey == "" {
		t.Error("Public key is empty")
	}
	if len(wallet.PublicKey) < 32 {
		t.Errorf("Public key too short: %d chars", len(wallet.PublicKey))
	}

	// Check private key
	if wallet.PrivateKey == "" {
		t.Error("Private key is empty")
	}

	// Check mnemonic
	if wallet.Mnemonic == "" {
		t.Error("Mnemonic is empty")
	}

	// Mnemonic should be 12 words
	words := len(splitWords(wallet.Mnemonic))
	if words != 12 {
		t.Errorf("Mnemonic has %d words, expected 12", words)
	}

	t.Logf("✅ Wallet generated successfully")
	t.Logf("   Public Key: %s", wallet.PublicKey[:20]+"...")
	t.Logf("   Mnemonic: %d words", words)
}

func TestImportFromPrivateKey(t *testing.T) {
	// Generate a wallet first
	original, _ := GenerateWallet()

	// Import using the private key
	imported, err := ImportFromPrivateKey(original.PrivateKey)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	// Public key should match
	if imported.PublicKey != original.PublicKey {
		t.Error("Imported public key doesn't match original")
	}

	// Private key should match
	if imported.PrivateKey != original.PrivateKey {
		t.Error("Imported private key doesn't match original")
	}

	// Mnemonic should be empty (not generated during import)
	if imported.Mnemonic != "" {
		t.Error("Imported wallet should not have mnemonic")
	}

	t.Log("✅ Private key import working")
}

func TestImportFromMnemonic(t *testing.T) {
	// Generate a wallet first
	original, _ := GenerateWallet()

	// Import using the mnemonic
	imported, err := ImportFromMnemonic(original.Mnemonic)
	if err != nil {
		t.Fatalf("Mnemonic import failed: %v", err)
	}

	// Keys should match
	if imported.PublicKey != original.PublicKey {
		t.Error("Keys don't match after mnemonic import")
	}

	// Mnemonic should match
	if imported.Mnemonic != original.Mnemonic {
		t.Error("Mnemonic doesn't match")
	}

	t.Log("✅ Mnemonic import working")
}

func TestValidatePrivateKey(t *testing.T) {
	wallet, _ := GenerateWallet()

	// Valid key
	if !ValidatePrivateKey(wallet.PrivateKey) {
		t.Error("Valid private key marked as invalid")
	}

	// Invalid keys
	invalidKeys := []string{
		"",
		"invalid",
		"123",
		"shortkey",
	}

	for _, key := range invalidKeys {
		if ValidatePrivateKey(key) {
			t.Errorf("Invalid key '%s' marked as valid", key)
		}
	}

	t.Log("✅ Private key validation working")
}

func TestValidateMnemonic(t *testing.T) {
	wallet, _ := GenerateWallet()

	// Valid mnemonic
	if !ValidateMnemonic(wallet.Mnemonic) {
		t.Error("Valid mnemonic marked as invalid")
	}

	// Invalid mnemonics
	invalidMnemonics := []string{
		"",
		"invalid mnemonic phrase",
		"one two three",
		"abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon invalid",
	}

	for _, mnemonic := range invalidMnemonics {
		if ValidateMnemonic(mnemonic) {
			t.Errorf("Invalid mnemonic marked as valid: '%s'", mnemonic)
		}
	}

	t.Log("✅ Mnemonic validation working")
}

func TestFullWalletFlow(t *testing.T) {
	// 1. Generate wallet
	wallet, err := GenerateWallet()
	if err != nil {
		t.Fatalf("Generation failed: %v", err)
	}
	t.Log("✅ Step 1: Wallet generated")

	// 2. Encrypt private key
	password := "SecurePassword123!"
	encWallet, err := EncryptPrivateKey(wallet.PrivateKey, password)
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}
	t.Log("✅ Step 2: Private key encrypted")

	// 3. Decrypt private key
	decrypted, err := DecryptPrivateKey(encWallet, password)
	if err != nil {
		t.Fatalf("Decryption failed: %v", err)
	}
	t.Log("✅ Step 3: Private key decrypted")

	// 4. Verify it matches
	if decrypted != wallet.PrivateKey {
		t.Error("Decrypted key doesn't match original")
	}
	t.Log("✅ Step 4: Keys match")

	// 5. Re-import from mnemonic to verify backup works
	restored, err := ImportFromMnemonic(wallet.Mnemonic)
	if err != nil {
		t.Fatalf("Mnemonic restore failed: %v", err)
	}
	if restored.PublicKey != wallet.PublicKey {
		t.Error("Restored wallet doesn't match original")
	}
	t.Log("✅ Step 5: Wallet restored from mnemonic")

	t.Log("\n🎉 Full wallet flow test PASSED!")
}

// Helper to split mnemonic into words
func splitWords(mnemonic string) []string {
	words := []string{}
	current := ""
	for _, char := range mnemonic + " " {
		if char == ' ' {
			if current != "" {
				words = append(words, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}
	return words
}
