package crypto

import (
	"testing"
)

func TestEncryptDecryptPrivateKey(t *testing.T) {
	privateKey := "5JKCvJNKqH7Xz8p9YQW3kRvP2mF8nL6sX9wT4vB7cD1eG2fH3aJ4iK5lM6nO7pQ8rS9tU0vW1xY2zA3bC4dE5f"
	password := "MySecurePassword123!"

	// Test encryption
	t.Run("Encrypt", func(t *testing.T) {
		encWallet, err := EncryptPrivateKey(privateKey, password)
		if err != nil {
			t.Fatalf("Encryption failed: %v", err)
		}

		if len(encWallet.EncryptedKey) == 0 {
			t.Error("Encrypted key is empty")
		}
		if len(encWallet.Salt) != 32 {
			t.Errorf("Salt length is %d, expected 32", len(encWallet.Salt))
		}
		if encWallet.PasswordHash == "" {
			t.Error("Password hash is empty")
		}

		t.Logf("Encryption successful - Key size: %d bytes", len(encWallet.EncryptedKey))
	})

	// Test decryption with correct password
	t.Run("DecryptCorrectPassword", func(t *testing.T) {
		encWallet, _ := EncryptPrivateKey(privateKey, password)

		decrypted, err := DecryptPrivateKey(encWallet, password)
		if err != nil {
			t.Fatalf("Decryption failed: %v", err)
		}

		if decrypted != privateKey {
			t.Error("Decrypted key doesn't match original")
		}

		t.Log("✅ Decryption with correct password successful")
	})

	// Test decryption with wrong password
	t.Run("DecryptWrongPassword", func(t *testing.T) {
		encWallet, _ := EncryptPrivateKey(privateKey, password)

		_, err := DecryptPrivateKey(encWallet, "WrongPassword")
		if err == nil {
			t.Error("Decryption should fail with wrong password")
		}

		t.Log("✅ Correctly rejected wrong password")
	})
}

func TestPasswordHashing(t *testing.T) {
	password := "TestPassword123"

	// Test hashing
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("Password hashing failed: %v", err)
	}

	if hash == "" {
		t.Error("Hash is empty")
	}

	// Test verification with correct password
	if !VerifyPassword(password, hash) {
		t.Error("Correct password verification failed")
	}

	// Test verification with wrong password
	if VerifyPassword("WrongPassword", hash) {
		t.Error("Wrong password should not verify")
	}

	t.Log("✅ Password hashing and verification working")
}

func TestKeyDerivation(t *testing.T) {
	password := "TestPassword"
	salt1, _ := GenerateSalt()
	salt2, _ := GenerateSalt()

	// Same password + same salt = same key
	key1a := DeriveKey(password, salt1)
	key1b := DeriveKey(password, salt1)

	if string(key1a) != string(key1b) {
		t.Error("Same password and salt should produce same key")
	}

	// Same password + different salt = different key
	key2 := DeriveKey(password, salt2)
	if string(key1a) == string(key2) {
		t.Error("Different salts should produce different keys")
	}

	t.Log("✅ Key derivation working correctly")
}

func TestMemoryZeroing(t *testing.T) {
	data := []byte("SensitiveData123")
	original := string(data)

	ZeroBytes(data)

	// Check all bytes are zero
	for i, b := range data {
		if b != 0 {
			t.Errorf("Byte at index %d is not zero: %d", i, b)
		}
	}

	if string(data) == original {
		t.Error("Data was not zeroed")
	}

	t.Log("✅ Memory zeroing working")
}

func TestBase64Encoding(t *testing.T) {
	data := []byte("Test data to encode")

	encoded := EncodeToBase64(data)
	if encoded == "" {
		t.Error("Encoding produced empty string")
	}

	decoded, err := DecodeFromBase64(encoded)
	if err != nil {
		t.Fatalf("Decoding failed: %v", err)
	}

	if string(decoded) != string(data) {
		t.Error("Decoded data doesn't match original")
	}

	t.Log("✅ Base64 encoding/decoding working")
}
