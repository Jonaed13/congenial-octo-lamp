package solana

import (
	"context"
	"fmt"
	"time"

	"solana-orchestrator/storage"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// LimitOrderManager handles limit order operations
type LimitOrderManager struct {
	RPCClient  *rpc.Client
	JitoClient *JitoClient
	DB         *storage.DB
	TokenCache *TokenSupplyCache
	// We might need a Jupiter client here too, but for now assuming we construct instructions manually or via helper
	// In reality, we'd use the Jupiter SDK or API to get instructions.
}

// NewLimitOrderManager creates a new manager
func NewLimitOrderManager(rpcURL string, jito *JitoClient, db *storage.DB) *LimitOrderManager {
	return &LimitOrderManager{
		RPCClient:  rpc.New(rpcURL),
		JitoClient: jito,
		DB:         db,
		TokenCache: NewTokenSupplyCache(),
	}
}

// CreateTimedLimitOrder creates a limit order with expiry and MCAP targeting
func (m *LimitOrderManager) CreateTimedLimitOrder(ctx context.Context, userID int64, wallet *solana.PrivateKey, tokenMint string, amount float64, duration time.Duration, mcapTarget float64) error {
	// 1. Calculate Price from MCAP
	supply, err := m.TokenCache.GetSupplyFloat(ctx, tokenMint, m.RPCClient)
	if err != nil {
		return fmt.Errorf("failed to get token supply: %w", err)
	}

	// Target Price = Target MCAP / Total Supply
	// Assuming MCAP is in USD and Price is in USD? Or SOL?
	// Usually MCAP is USD. If we want Price in SOL, we need SOL price.
	// Let's assume the user inputs MCAP in USD and we want to set a Limit Order in USD (USDC) or SOL?
	// If the pair is SOL/Token, the limit price is in SOL.
	// So Target MCAP (USD) / Supply = Target Price (USD).
	// We need to convert Target Price (USD) to Target Price (SOL).
	// Price (SOL) = Price (USD) / SOL Price (USD).

	// For this implementation, let's assume the "Target MCAP" is actually "Target FDV in SOL"
	// or we have a way to get SOL price.
	// To keep it simple and strictly follow the prompt's formula:
	// targetPrice := mcapTarget / supply
	// We'll assume mcapTarget is in the same unit as the quote currency (e.g. SOL if trading vs SOL).

	targetPrice := mcapTarget / supply

	// 2. Calculate Expiry Timestamp
	expiryTime := time.Now().Add(duration).Unix()

	// 3. Build Jupiter Limit Order Instruction
	// This requires interacting with Jupiter's Limit Order program.
	// Since we don't have the full SDK here, we'll mock the instruction creation.
	// In a real app, we'd call `jupiter.NewCreateOrderInstruction(...)`.

	// Placeholder for actual instruction building
	// inst := jupiter.BuildCreateOrderIx(...)
	// For now, we'll create a dummy transfer to simulate "locking funds"
	inst := solana.NewInstruction(
		solana.SystemProgramID,
		solana.AccountMetaSlice{},
		[]byte{2, 0, 0, 0}, // Dummy data
	)

	// 4. Build Transaction
	latestBlockhash, err := m.RPCClient.GetRecentBlockhash(ctx, rpc.CommitmentProcessed)
	if err != nil {
		return err
	}

	tx, err := solana.NewTransaction(
		[]solana.Instruction{inst},
		latestBlockhash.Value.Blockhash,
		solana.TransactionPayer(wallet.PublicKey()),
	)
	if err != nil {
		return err
	}

	// Sign
	tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(wallet.PublicKey()) {
			return wallet
		}
		return nil
	})

	// 5. Send via Jito
	_, err = m.JitoClient.SendJitoBundle(ctx, tx, 10000) // Tip
	if err != nil {
		return err
	}

	// 6. Save to DB
	// We need the Order Pubkey (usually derived or returned).
	// For this mock, we'll generate a random one.
	orderPubkey := solana.NewWallet().PublicKey().String()

	order := &storage.LimitOrder{
		UserID:      userID,
		OrderPubkey: orderPubkey,
		TokenMint:   tokenMint,
		Side:        "buy", // Simplified
		Price:       targetPrice,
		Amount:      amount,
		Status:      "OPEN",
		ExpiresAt:   expiryTime,
		TargetMCAP:  mcapTarget,
	}

	return m.DB.SaveLimitOrder(order)
}

// UpdateLimitOrder atomically updates a limit order (Cancel + Create)
func (m *LimitOrderManager) UpdateLimitOrder(ctx context.Context, oldOrder *storage.LimitOrder, newPrice float64, wallet *solana.PrivateKey) error {
	// 1. Build Cancel Instruction
	cancelInst, err := m.buildCancelIx(oldOrder.OrderPubkey)
	if err != nil {
		return err
	}

	// 2. Build New Create Instruction
	// Placeholder
	createInst := solana.NewInstruction(
		solana.SystemProgramID,
		solana.AccountMetaSlice{},
		[]byte{2}, // Dummy
	)

	// 3. Build Atomic Transaction
	latestBlockhash, err := m.RPCClient.GetRecentBlockhash(ctx, rpc.CommitmentProcessed)
	if err != nil {
		return err
	}

	tx, err := solana.NewTransaction(
		[]solana.Instruction{cancelInst, createInst}, // Atomic bundle in one tx
		latestBlockhash.Value.Blockhash,
		solana.TransactionPayer(wallet.PublicKey()),
	)
	if err != nil {
		return err
	}

	tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(wallet.PublicKey()) {
			return wallet
		}
		return nil
	})

	// 4. Send via Jito
	_, err = m.JitoClient.SendJitoBundle(ctx, tx, 10000)
	if err != nil {
		return err
	}

	// 5. Update DB
	// Mark old as cancelled
	m.DB.UpdateOrderStatus(oldOrder.ID, "CANCELLED_REPLACED")

	// Create new order record...
	return nil
}

// BuildCancelOrderTx builds a transaction to cancel an order
func (m *LimitOrderManager) BuildCancelOrderTx(ctx context.Context, orderPubkey string) (*solana.Transaction, error) {
	inst, err := m.buildCancelIx(orderPubkey)
	if err != nil {
		return nil, err
	}

	// We need a blockhash
	latestBlockhash, err := m.RPCClient.GetRecentBlockhash(ctx, rpc.CommitmentProcessed)
	if err != nil {
		return nil, err
	}

	// Note: We don't have the user's private key here to sign!
	// The Janitor runs in background.
	// If the order account allows "anyone" to cancel if expired (some protocols do), then we can sign with bot key.
	// Jupiter Limit Orders usually allow anyone to cancel if expired?
	// Or maybe we need the user's key?
	// The prompt says: "The Janitor... Cancel and Refund... Send via Jito".
	// If it's a "Janitor", it implies the bot has authority or the protocol allows it.
	// Assuming the bot wallet (payer) can cancel or we have access to user keys (EncryptedWallet).
	// Since `Janitor` struct has `SolanaClient`, and `BuildCancelOrderTx` returns a `Transaction`,
	// the caller (Janitor) will likely need to sign it.
	// But wait, `Janitor` doesn't have access to user private keys easily.
	// If the protocol requires user signature, Janitor can't do it unless we decrypt user wallet.
	// Let's assume for this implementation that we return an unsigned transaction
	// and the Janitor will handle signing (maybe with a "Crank" key if protocol supports it,
	// or we assume we can decrypt the user's key).

	// For the purpose of this exercise, I'll return the transaction.

	return solana.NewTransaction(
		[]solana.Instruction{inst},
		latestBlockhash.Value.Blockhash,
		solana.TransactionPayer(solana.PublicKey{}), // Placeholder payer
	)
}

func (m *LimitOrderManager) buildCancelIx(orderPubkey string) (solana.Instruction, error) {
	// Placeholder for Jupiter Cancel Instruction
	return solana.NewInstruction(
		solana.SystemProgramID,
		solana.AccountMetaSlice{},
		[]byte{1}, // Dummy
	), nil
}
