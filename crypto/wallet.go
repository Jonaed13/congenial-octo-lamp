package crypto

import (
	"crypto/ed25519"
	"errors"

	"github.com/gagliardetto/solana-go"
	"github.com/mr-tron/base58"
	"github.com/tyler-smith/go-bip39"
)

// WalletKeypair represents a Solana wallet with public and private keys
type WalletKeypair struct {
	PublicKey  string // Base58 encoded
	PrivateKey string // Base58 encoded
	Mnemonic   string // BIP39 seed phrase (optional)
}

// GenerateWallet creates a new Solana wallet with mnemonic
func GenerateWallet() (*WalletKeypair, error) {
	// Generate mnemonic (12 words)
	entropy, err := bip39.NewEntropy(128)
	if err != nil {
		return nil, err
	}

	mnemonic, err := bip39.NewMnemonic(entropy)
	if err != nil {
		return nil, err
	}

	// Derive seed from mnemonic
	seed := bip39.NewSeed(mnemonic, "")

	// Use first 32 bytes as private key
	privateKey := ed25519.NewKeyFromSeed(seed[:32])
	publicKey := privateKey.Public().(ed25519.PublicKey)

	return &WalletKeypair{
		PublicKey:  base58.Encode(publicKey),
		PrivateKey: base58.Encode(privateKey),
		Mnemonic:   mnemonic,
	}, nil
}

// ImportFromPrivateKey imports a wallet from a base58 private key
func ImportFromPrivateKey(privateKeyBase58 string) (*WalletKeypair, error) {
	// Decode private key
	privateKeyBytes, err := base58.Decode(privateKeyBase58)
	if err != nil {
		return nil, errors.New("invalid private key format")
	}

	if len(privateKeyBytes) != ed25519.PrivateKeySize {
		return nil, errors.New("invalid private key length")
	}

	privateKey := ed25519.PrivateKey(privateKeyBytes)
	publicKey := privateKey.Public().(ed25519.PublicKey)

	return &WalletKeypair{
		PublicKey:  base58.Encode(publicKey),
		PrivateKey: privateKeyBase58,
		Mnemonic:   "", // No mnemonic when importing from key
	}, nil
}

// ImportFromMnemonic imports a wallet from a BIP39 mnemonic
func ImportFromMnemonic(mnemonic string) (*WalletKeypair, error) {
	// Validate mnemonic
	if !bip39.IsMnemonicValid(mnemonic) {
		return nil, errors.New("invalid mnemonic phrase")
	}

	// Derive seed
	seed := bip39.NewSeed(mnemonic, "")

	// Generate keypair
	privateKey := ed25519.NewKeyFromSeed(seed[:32])
	publicKey := privateKey.Public().(ed25519.PublicKey)

	return &WalletKeypair{
		PublicKey:  base58.Encode(publicKey),
		PrivateKey: base58.Encode(privateKey),
		Mnemonic:   mnemonic,
	}, nil
}

// GetSolanaPrivateKey converts base58 private key to solana-go PrivateKey
func GetSolanaPrivateKey(privateKeyBase58 string) (solana.PrivateKey, error) {
	privateKeyBytes, err := base58.Decode(privateKeyBase58)
	if err != nil {
		return solana.PrivateKey{}, err
	}

	return solana.PrivateKey(privateKeyBytes), nil
}

// ValidatePrivateKey checks if a private key is valid
func ValidatePrivateKey(privateKeyBase58 string) bool {
	privateKeyBytes, err := base58.Decode(privateKeyBase58)
	if err != nil {
		return false
	}

	return len(privateKeyBytes) == ed25519.PrivateKeySize
}

// ValidateMnemonic checks if a mnemonic phrase is valid
func ValidateMnemonic(mnemonic string) bool {
	return bip39.IsMnemonicValid(mnemonic)
}
