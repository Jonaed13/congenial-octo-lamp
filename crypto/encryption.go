package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/pbkdf2"
)

// EncryptedWallet represents an encrypted private key
type EncryptedWallet struct {
	EncryptedKey []byte
	Salt         []byte
	Nonce        []byte
	PasswordHash string
}

// GenerateSalt creates a random salt
func GenerateSalt() ([]byte, error) {
	salt := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}
	return salt, nil
}

// DeriveKey derives an encryption key from password and salt using PBKDF2
func DeriveKey(password string, salt []byte) []byte {
	return pbkdf2.Key([]byte(password), salt, 100000, 32, sha256.New)
}

// HashPassword hashes a password using bcrypt
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// VerifyPassword checks if password matches the hash
func VerifyPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// EncryptPrivateKey encrypts a private key with AES-256-GCM
func EncryptPrivateKey(privateKey string, password string) (*EncryptedWallet, error) {
	// Generate salt
	salt, err := GenerateSalt()
	if err != nil {
		return nil, err
	}

	// Derive encryption key from password
	key := DeriveKey(password, salt)

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// Create GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// Generate nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	// Encrypt
	ciphertext := gcm.Seal(nil, nonce, []byte(privateKey), nil)

	// Hash password for verification
	passwordHash, err := HashPassword(password)
	if err != nil {
		return nil, err
	}

	return &EncryptedWallet{
		EncryptedKey: ciphertext,
		Salt:         salt,
		Nonce:        nonce,
		PasswordHash: passwordHash,
	}, nil
}

// DecryptPrivateKey decrypts an encrypted private key
func DecryptPrivateKey(encWallet *EncryptedWallet, password string) (string, error) {
	// Verify password first
	if !VerifyPassword(password, encWallet.PasswordHash) {
		return "", errors.New("invalid password")
	}

	// Derive the same encryption key
	key := DeriveKey(password, encWallet.Salt)

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	// Create GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// Decrypt
	plaintext, err := gcm.Open(nil, encWallet.Nonce, encWallet.EncryptedKey, nil)
	if err != nil {
		return "", errors.New("decryption failed")
	}

	return string(plaintext), nil
}

// EncodeToBase64 encodes bytes to base64 string
func EncodeToBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// DecodeFromBase64 decodes base64 string to bytes
func DecodeFromBase64(encoded string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(encoded)
}

// ZeroBytes securely zeros out a byte slice
func ZeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// ZeroString securely zeros out a string's underlying bytes
// Note: This only works if we have access to the underlying byte array
func ZeroString(s *string) {
	// Convert to []byte and zero
	if s != nil && len(*s) > 0 {
		// In Go, strings are immutable, so we can't truly zero them
		// Best practice is to not reuse the variable and let GC handle it
		*s = ""
	}
}
