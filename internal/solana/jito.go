package solana

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/rpc"
)

// JitoBlockEngineURL is the endpoint for the Jito Block Engine
const JitoBlockEngineURL = "https://amsterdam.mainnet.block-engine.jito.wtf/api/v1/bundles"

// JitoTipAccount is a common Jito tip account
var JitoTipAccount = solana.MustPublicKeyFromBase58("96gYZGLnJYVFmbjzopPSU6QiEV5fGqZNyN9nmNhvrZU5")

// JitoClient handles communication with the Jito Block Engine
type JitoClient struct {
	rpcClient  *rpc.Client
	httpClient *http.Client
	privateKey solana.PrivateKey
}

// NewJitoClient creates a new Jito client
func NewJitoClient(rpcURL string, privateKey solana.PrivateKey) *JitoClient {
	return &JitoClient{
		rpcClient:  rpc.New(rpcURL),
		httpClient: &http.Client{Timeout: 10 * time.Second},
		privateKey: privateKey,
	}
}

// SendJitoBundle sends a transaction bundle with a tip to the Jito Block Engine
func (c *JitoClient) SendJitoBundle(ctx context.Context, tx *solana.Transaction, tipLamports uint64) (string, error) {
	// 1. Create Tip Transaction
	// We create a separate transaction for the tip to avoid modifying the original signed transaction.
	// Jito bundles execute atomically, so if the tip fails, the whole bundle fails.

	// Get fresh blockhash for the tip tx
	latestBlockhash, err := c.rpcClient.GetRecentBlockhash(ctx, rpc.CommitmentProcessed)
	if err != nil {
		return "", fmt.Errorf("failed to get blockhash: %w", err)
	}

	// Create tip instruction
	tipInst := system.NewTransferInstruction(
		tipLamports,
		c.privateKey.PublicKey(),
		JitoTipAccount,
	).Build()

	// Build tip transaction
	tipTx, err := solana.NewTransaction(
		[]solana.Instruction{tipInst},
		latestBlockhash.Value.Blockhash,
		solana.TransactionPayer(c.privateKey.PublicKey()),
	)
	if err != nil {
		return "", fmt.Errorf("failed to build tip tx: %w", err)
	}

	// Sign tip transaction
	_, err = tipTx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(c.privateKey.PublicKey()) {
			return &c.privateKey
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to sign tip tx: %w", err)
	}

	// 2. Serialize Transactions
	// Serialize original tx
	serializedTx, err := tx.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("failed to marshal original tx: %w", err)
	}
	base58Tx := solana.Base58(serializedTx).String() // Cast to string

	// Serialize tip tx
	serializedTipTx, err := tipTx.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("failed to marshal tip tx: %w", err)
	}
	base58TipTx := solana.Base58(serializedTipTx).String() // Cast to string

	// 3. Construct JSON-RPC Request for Jito
	// We send a bundle containing [OriginalTx, TipTx]
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "sendBundle",
		"params": []interface{}{
			[]string{base58Tx, base58TipTx}, // Array of transactions in the bundle
		},
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	// 4. Send Request
	req, err := http.NewRequestWithContext(ctx, "POST", JitoBlockEngineURL, bytes.NewReader(jsonPayload))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send bundle: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("jito error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response to get bundle ID
	var rpcResp struct {
		Result string `json:"result"`
		Error  struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		return "", fmt.Errorf("failed to parse jito response: %w", err)
	}

	if rpcResp.Error.Message != "" {
		return "", fmt.Errorf("jito rpc error: %s", rpcResp.Error.Message)
	}

	return rpcResp.Result, nil
}
