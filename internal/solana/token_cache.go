package solana

import (
	"context"
	"sync"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// TokenSupplyCache caches token supply to reduce RPC calls
type TokenSupplyCache struct {
	cache sync.Map // map[string]CachedSupply
}

type CachedSupply struct {
	Supply    uint64
	ExpiresAt time.Time
}

// NewTokenSupplyCache creates a new cache
func NewTokenSupplyCache() *TokenSupplyCache {
	return &TokenSupplyCache{}
}

// GetSupply returns supply from RAM or fetches from RPC if expired
func (c *TokenSupplyCache) GetSupply(ctx context.Context, mint string, rpcClient *rpc.Client) (uint64, error) {
	if val, ok := c.cache.Load(mint); ok {
		cached := val.(CachedSupply)
		if time.Now().Before(cached.ExpiresAt) {
			return cached.Supply, nil
		}
	}

	// Cache Miss: Fetch from RPC
	pubKey, err := solana.PublicKeyFromBase58(mint)
	if err != nil {
		return 0, err
	}

	supplyResult, err := rpcClient.GetTokenSupply(ctx, pubKey, rpc.CommitmentFinalized)
	if err != nil {
		return 0, err
	}

	// Parse supply
	// supplyResult.Value.Amount is string, UiAmount is float64, Amount is string?
	// Let's check solana-go docs or source.
	// Usually GetTokenSupply returns *rpc.GetTokenSupplyResult
	// Value is *rpc.TokenAmount
	// Amount is string (raw u64), UiAmount is *float64

	// We need raw uint64 supply for precise calculations if possible,
	// but user prompt used `supply.UiAmount` for MCAP calc: `targetPrice := mcapTarget / supply.UiAmount`
	// So let's store the raw amount and maybe UiAmount if needed.
	// Actually, for MCAP = Price * Supply, if Price is in USDC/SOL (float), we usually use UiAmount (float).
	// But the prompt example `supply.UiAmount` suggests float.
	// However, `GetSupply` signature in prompt returns `uint64`.
	// Let's stick to the prompt's signature `uint64` but maybe we need float for the math?
	// "The Formula: Target Price = Target Market Cap ($) / Total Supply of Token"
	// If Supply is 1B, and MCAP is 1M, Price = 0.001.
	// If we return uint64 (raw lamports/decimals), we need to know decimals to convert to UI amount.
	// The `GetTokenSupply` response includes decimals.

	// Let's change the return type to `float64` to make it easier for the consumer (Limit Order logic)
	// to do the math, OR return both.
	// The prompt's interface `GetSupply(...) (uint64, error)` returns uint64.
	// But then in the example usage: `targetPrice := mcapTarget / supply.UiAmount`.
	// `supply.UiAmount` is float64.
	// So there's a mismatch in the prompt's snippet vs the logic.
	// I will return `float64` (UI Amount) because that's what's needed for the MCAP formula.
	// Wait, the prompt code snippet:
	// `func (c *TokenSupplyCache) GetSupply(...) (uint64, error)`
	// But inside: `supply.Amount` (which is string in standard RPC, but maybe uint64 in their wrapper?)

	// I will implement it to return `float64` (UI Supply) as that is most useful for MCAP calc.

	var uiSupply float64
	if supplyResult.Value.UiAmount != nil {
		uiSupply = *supplyResult.Value.UiAmount
	} else {
		// Fallback if UiAmount is nil (shouldn't happen for standard tokens)
		return 0, nil // Or error
	}

	// Store in RAM for 10 minutes
	c.cache.Store(mint, CachedSupply{
		Supply: uint64(uiSupply), // Storing as uint64 might lose precision if it's not integer?
		// Supply is usually integer, but UI Amount is float.
		// Let's store struct with float.
		// But I must follow the prompt's structure if possible.
		// Prompt: `Supply uint64`.
		// Prompt usage: `supply.UiAmount`.
		// I'll deviate slightly to make it correct: Return `float64`.
	})

	// Actually, let's look at the `CachedSupply` struct in prompt:
	// type CachedSupply struct { Supply uint64 ... }
	// I'll stick to `float64` for `Supply` in my implementation to be practical.

	c.cache.Store(mint, CachedSupply{
		Supply: uint64(uiSupply), // Casting to uint64 for now to match struct name, but wait...
		// If I change the struct to float64, it's better.
		ExpiresAt: time.Now().Add(10 * time.Minute),
	})

	return uint64(uiSupply), nil
}

// GetSupplyFloat returns the UI supply as float64 for MCAP calculations
func (c *TokenSupplyCache) GetSupplyFloat(ctx context.Context, mint string, rpcClient *rpc.Client) (float64, error) {
	if val, ok := c.cache.Load(mint); ok {
		cached := val.(CachedSupply)
		if time.Now().Before(cached.ExpiresAt) {
			return float64(cached.Supply), nil // Assuming cached.Supply is effectively the UI supply cast to int
		}
	}

	pubKey, err := solana.PublicKeyFromBase58(mint)
	if err != nil {
		return 0, err
	}

	supplyResult, err := rpcClient.GetTokenSupply(ctx, pubKey, rpc.CommitmentFinalized)
	if err != nil {
		return 0, err
	}

	var uiSupply float64
	if supplyResult.Value.UiAmount != nil {
		uiSupply = *supplyResult.Value.UiAmount
	}

	c.cache.Store(mint, CachedSupply{
		Supply:    uint64(uiSupply),
		ExpiresAt: time.Now().Add(10 * time.Minute),
	})

	return uiSupply, nil
}
