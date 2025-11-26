package engine

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/gagliardetto/solana-go"
	"golang.org/x/crypto/pbkdf2"
)

// DecryptWallet decrypts the private key using the user's password
func DecryptWallet(encryptedKey, salt, nonce string, password string) (*solana.PrivateKey, error) {
	// 1. Derive key from password and salt
	saltBytes, err := base64.StdEncoding.DecodeString(salt)
	if err != nil {
		return nil, fmt.Errorf("invalid salt: %w", err)
	}

	key := pbkdf2.Key([]byte(password), saltBytes, 4096, 32, sha256.New)

	// 2. Decode encrypted key and nonce
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedKey)
	if err != nil {
		return nil, fmt.Errorf("invalid encrypted key: %w", err)
	}

	nonceBytes, err := base64.StdEncoding.DecodeString(nonce)
	if err != nil {
		return nil, fmt.Errorf("invalid nonce: %w", err)
	}

	// 3. Decrypt using AES-GCM
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	plaintext, err := aesgcm.Open(nil, nonceBytes, ciphertext, nil)
	if err != nil {
		return nil, errors.New("decryption failed (wrong password?)")
	}

	// 4. Parse Private Key
	privKey, err := solana.PrivateKeyFromBase58(string(plaintext))
	if err != nil {
		return nil, fmt.Errorf("invalid private key format: %w", err)
	}

	return &privKey, nil
}
