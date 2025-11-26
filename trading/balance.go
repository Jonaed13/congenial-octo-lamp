package trading

import (
	"context"
	"fmt"
	"solana-orchestrator/api"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// BalanceManager handles wallet balance queries
type BalanceManager struct {
	rpcClient *rpc.Client
	wsClient  *WSClient
	apiClient *api.Client
}

// NewBalanceManager creates a new balance manager
func NewBalanceManager(rpcURL string, wsClient *WSClient, apiClient *api.Client) *BalanceManager {
	return &BalanceManager{
		rpcClient: rpc.New(rpcURL),
		wsClient:  wsClient,
		apiClient: apiClient,
	}
}

// Balance represents wallet balances
type Balance struct {
	Wallet        solana.PublicKey
	SOLBalance    uint64 // in lamports
	TokenBalances []TokenBalance
}

// TokenBalance represents a single token holding
type TokenBalance struct {
	Mint     solana.PublicKey
	Amount   uint64
	Decimals uint8
	UIAmount float64
	Symbol   string
	Name     string
	Logo     string
}

// GetSOLBalance fetches SOL balance for a wallet
func (bm *BalanceManager) GetSOLBalance(ctx context.Context, wallet solana.PublicKey) (uint64, error) {
	balance, err := bm.rpcClient.GetBalance(ctx, wallet, rpc.CommitmentFinalized)
	if err != nil {
		return 0, fmt.Errorf("failed to get SOL balance: %w", err)
	}
	return balance.Value, nil
}

// GetTokenBalances fetches all token balances for a wallet using Moralis API
func (bm *BalanceManager) GetTokenBalances(ctx context.Context, wallet solana.PublicKey) ([]TokenBalance, error) {
	if bm.apiClient == nil {
		return nil, fmt.Errorf("API client not initialized")
	}

	tokens, err := bm.apiClient.GetWalletTokenBalances(ctx, wallet.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get token balances: %w", err)
	}

	tokenBalances := make([]TokenBalance, 0, len(tokens))
	for _, t := range tokens {
		// Parse balance string to uint64
		amount := parseUint64(t.Balance)

		// Calculate UI Amount
		uiAmount := float64(amount) / float64(pow10(t.Decimals))

		mint, _ := solana.PublicKeyFromBase58(t.TokenAddress)

		tokenBalances = append(tokenBalances, TokenBalance{
			Mint:     mint,
			Amount:   amount,
			Decimals: uint8(t.Decimals),
			UIAmount: uiAmount,
			Symbol:   t.Symbol,
			Name:     t.Name,
			Logo:     t.Logo,
		})
	}

	return tokenBalances, nil
}

// GetFullBalance retrieves both SOL and token balances
func (bm *BalanceManager) GetFullBalance(ctx context.Context, wallet solana.PublicKey) (*Balance, error) {
	solBalance, err := bm.GetSOLBalance(ctx, wallet)
	if err != nil {
		return nil, err
	}

	tokenBalances, err := bm.GetTokenBalances(ctx, wallet)
	if err != nil {
		// Don't fail completely if token balances fail
		fmt.Printf("Warning: failed to get token balances: %v\n", err)
		tokenBalances = []TokenBalance{}
	}

	return &Balance{
		Wallet:        wallet,
		SOLBalance:    solBalance,
		TokenBalances: tokenBalances,
	}, nil
}

// SubscribeToBalance subscribes to real-time balance updates via WebSocket
func (bm *BalanceManager) SubscribeToBalance(ctx context.Context, wallet solana.PublicKey) (<-chan interface{}, error) {
	return bm.wsClient.SubscribeAccount(ctx, wallet.String())
}

// parseUint64 safely parses string to uint64
func parseUint64(s string) uint64 {
	var result uint64
	fmt.Sscanf(s, "%d", &result)
	return result
}

// FormatSOL converts lamports to SOL
func FormatSOL(lamports uint64) float64 {
	return float64(lamports) / 1e9
}

func pow10(n int) uint64 {
	result := uint64(1)
	for i := 0; i < n; i++ {
		result *= 10
	}
	return result
}
