package trading

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const JUPITER_QUOTE_API = "https://quote-api.jup.ag/v6/quote"
const JUPITER_SWAP_API = "https://lite-api.jup.ag/swap/v1/swap"

// SOL mint address
const SOL_MINT = "So11111111111111111111111111111111111111112"

// JupiterQuote represents a quote response from Jupiter
type JupiterQuote struct {
	InputMint            string                   `json:"inputMint"`
	InAmount             string                   `json:"inAmount"`
	OutputMint           string                   `json:"outputMint"`
	OutAmount            string                   `json:"outAmount"`
	OtherAmountThreshold string                   `json:"otherAmountThreshold"`
	SwapMode             string                   `json:"swapMode"`
	SlippageBps          int                      `json:"slippageBps"`
	PriceImpactPct       string                   `json:"priceImpactPct"` // Changed to string to match API response
	RoutePlan            []map[string]interface{} `json:"routePlan"`
}

// PrioritizationFee represents the fee structure
type PrioritizationFee struct {
	PriorityLevelWithMaxLamports *PriorityLevel `json:"priorityLevelWithMaxLamports,omitempty"`
}

// PriorityLevel defines the max lamports and priority level
type PriorityLevel struct {
	MaxLamports   int64  `json:"maxLamports"`
	PriorityLevel string `json:"priorityLevel"`
}

// JupiterSwapRequest represents a swap request
type JupiterSwapRequest struct {
	QuoteResponse             JupiterQuote `json:"quoteResponse"`
	UserPublicKey             string       `json:"userPublicKey"`
	WrapAndUnwrapSol          bool         `json:"wrapAndUnwrapSol"`
	PrioritizationFeeLamports interface{}  `json:"prioritizationFeeLamports"` // Can be int64 or object
	DynamicComputeUnitLimit   bool         `json:"dynamicComputeUnitLimit"`
}

// JupiterSwapResponse represents the swap transaction
type JupiterSwapResponse struct {
	SwapTransaction      string `json:"swapTransaction"`
	LastValidBlockHeight int64  `json:"lastValidBlockHeight"`
}

// GetBuyQuote gets a quote for buying a token with SOL
func GetBuyQuote(ctx context.Context, tokenMint string, solAmount uint64, slippageBps int) (*JupiterQuote, error) {
	url := fmt.Sprintf("%s?inputMint=%s&outputMint=%s&amount=%d&slippageBps=%d",
		JUPITER_QUOTE_API, SOL_MINT, tokenMint, solAmount, slippageBps)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := SharedClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get quote: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Jupiter API error %d: %s", resp.StatusCode, string(body))
	}

	var quote JupiterQuote
	if err := json.NewDecoder(resp.Body).Decode(&quote); err != nil {
		return nil, fmt.Errorf("failed to parse quote: %w", err)
	}

	return &quote, nil
}

// GetSellQuote gets a quote for selling a token for SOL
func GetSellQuote(ctx context.Context, tokenMint string, tokenAmount uint64, slippageBps int) (*JupiterQuote, error) {
	url := fmt.Sprintf("%s?inputMint=%s&outputMint=%s&amount=%d&slippageBps=%d",
		JUPITER_QUOTE_API, tokenMint, SOL_MINT, tokenAmount, slippageBps)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := SharedClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get quote: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Jupiter API error %d: %s", resp.StatusCode, string(body))
	}

	var quote JupiterQuote
	if err := json.NewDecoder(resp.Body).Decode(&quote); err != nil {
		return nil, fmt.Errorf("failed to parse quote: %w", err)
	}

	return &quote, nil
}

// GetSwapTransaction gets the swap transaction from Jupiter
func GetSwapTransaction(ctx context.Context, quote *JupiterQuote, userPublicKey string, priorityFee int64) (*JupiterSwapResponse, error) {
	// Construct prioritization fee object
	// Using "veryHigh" and the provided fee as max lamports
	feeObj := PrioritizationFee{
		PriorityLevelWithMaxLamports: &PriorityLevel{
			MaxLamports:   priorityFee,
			PriorityLevel: "veryHigh",
		},
	}

	reqBody := JupiterSwapRequest{
		QuoteResponse:             *quote,
		UserPublicKey:             userPublicKey,
		WrapAndUnwrapSol:          true,
		PrioritizationFeeLamports: feeObj,
		DynamicComputeUnitLimit:   true,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", JUPITER_SWAP_API, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := SharedClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get swap transaction: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Jupiter swap API error %d: %s", resp.StatusCode, string(body))
	}

	var swapResp JupiterSwapResponse
	if err := json.NewDecoder(resp.Body).Decode(&swapResp); err != nil {
		return nil, fmt.Errorf("failed to parse swap response: %w", err)
	}

	return &swapResp, nil
}
