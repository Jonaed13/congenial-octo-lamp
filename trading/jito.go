package trading

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/mr-tron/base58"
)

// JitoClient handles Jito bundle submissions
type JitoClient struct {
	blockEngineURL string
	httpClient     *http.Client
	tipLamports    uint64
}

// NewJitoClient creates a new Jito client
func NewJitoClient(blockEngineURL string, tipLamports uint64) *JitoClient {
	return &JitoClient{
		blockEngineURL: blockEngineURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		tipLamports: tipLamports,
	}
}

// Bundle represents a Jito transaction bundle
type Bundle struct {
	Transactions []string `json:"transactions"`
}

// BundleResult represents the result of a bundle submission
type BundleResult struct {
	BundleID  string
	Signature string
	Status    string
}

// SubmitBundle submits a transaction bundle to Jito
func (jc *JitoClient) SubmitBundle(ctx context.Context, transactions []solana.Transaction) (*BundleResult, error) {
	// Serialize transactions to base58 (Jito expects this format)
	txStrings := make([]string, len(transactions))
	for i, tx := range transactions {
		txBytes, err := tx.MarshalBinary()
		if err != nil {
			return nil, fmt.Errorf("failed to marshal transaction %d: %w", i, err)
		}
		// Use base58 encoding for transaction bytes
		txStrings[i] = base58.Encode(txBytes)
	}

	bundle := Bundle{
		Transactions: txStrings,
	}

	// Create JSON-RPC request
	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "sendBundle",
		"params":  []interface{}{bundle.Transactions},
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Submit to Jito block engine
	httpReq, err := http.NewRequestWithContext(ctx, "POST", jc.blockEngineURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := jc.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send bundle: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bundle submission failed with status: %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Extract bundle ID
	bundleID := ""
	if res, ok := result["result"].(string); ok {
		bundleID = res
	}

	return &BundleResult{
		BundleID: bundleID,
		Status:   "submitted",
	}, nil
}

// CreateTipInstruction creates a Jito tip instruction
func (jc *JitoClient) CreateTipInstruction(feePayer solana.PublicKey) (solana.Instruction, error) {
	// Jito tip accounts (randomly select one)
	tipAccounts := []string{
		"96gYZGLnJYVFmbjzopPSU6QiEV5fGqZNyN9nmNhvrZU5",
		"HFqU5x63VTqvQss8hp11i4wVV8bD44PvwucfZ2bU7gRe",
		"Cw8CFyM9FkoMi7K7Crf6HNQqf4uEMzpKw6QNghXLvLkY",
		"ADaUMid9yfUytqMBgopwjb2DTLSokTSzL1zt6iGPaS49",
		"DfXygSm4jCyNCybVYYK6DwvWqjKee8pbDmJGcLWNDXjh",
		"ADuUkR4vqLUMWXxW9gh6D6L8pMSawimctcNZ5pGwDcEt",
		"DttWaMuVvTiduZRnguLF7jNxTgiMBZ1hyAumKUiL2KRL",
		"3AVi9Tg9Uo68tJfuvoKvqKNWKkC5wPdSSdeBnizKZ6jT",
	}

	tipAccount := solana.MustPublicKeyFromBase58(tipAccounts[0])

	// Create transfer instruction
	instruction := solana.NewInstruction(
		solana.SystemProgramID,
		solana.AccountMetaSlice{
			solana.Meta(feePayer).WRITE().SIGNER(),
			solana.Meta(tipAccount).WRITE(),
		},
		// Transfer instruction data (2 = Transfer, followed by amount)
		append([]byte{2, 0, 0, 0}, uint64ToBytes(jc.tipLamports)...),
	)

	return instruction, nil
}

// uint64ToBytes converts uint64 to little-endian bytes
func uint64ToBytes(num uint64) []byte {
	b := make([]byte, 8)
	for i := 0; i < 8; i++ {
		b[i] = byte(num >> (8 * i))
	}
	return b
}

// GetTipAmount returns the configured tip amount
func (jc *JitoClient) GetTipAmount() uint64 {
	return jc.tipLamports
}

// SetTipAmount updates the tip amount
func (jc *JitoClient) SetTipAmount(lamports uint64) {
	jc.tipLamports = lamports
}
