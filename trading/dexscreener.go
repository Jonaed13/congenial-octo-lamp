package trading

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DexScreener API client
const DEXSCREENER_API = "https://api.dexscreener.com/latest/dex/tokens"

// SharedClient is a shared HTTP client for the trading package
var SharedClient = &http.Client{
	Timeout: 15 * time.Second,
}

// TokenInfo represents token data from DexScreener
type TokenInfo struct {
	Address     string
	Name        string
	Symbol      string
	PriceUSD    string
	PriceSOL    string
	Change24h   float64
	Change6h    float64
	Change1h    float64
	Change5m    float64
	Liquidity   float64
	Volume24h   float64
	Buys5m      int
	Sells5m     int
	Buys1h      int
	Sells1h     int
	PairAddress string
	DexID       string
	TotalSupply string // Added for Shyft integration
}

type dexScreenerResponse struct {
	SchemaVersion string    `json:"schemaVersion"`
	Pairs         []dexPair `json:"pairs"`
}

type dexPair struct {
	ChainID     string      `json:"chainId"`
	DexID       string      `json:"dexId"`
	URL         string      `json:"url"`
	PairAddress string      `json:"pairAddress"`
	BaseToken   baseToken   `json:"baseToken"`
	PriceNative string      `json:"priceNative"`
	PriceUSD    string      `json:"priceUsd"`
	Txns        txns        `json:"txns"`
	Volume      volume      `json:"volume"`
	PriceChange priceChange `json:"priceChange"`
	Liquidity   liquidity   `json:"liquidity"`
}

type baseToken struct {
	Address string `json:"address"`
	Name    string `json:"name"`
	Symbol  string `json:"symbol"`
}

type txns struct {
	M5 txnDetail `json:"m5"`
	H1 txnDetail `json:"h1"`
}

type txnDetail struct {
	Buys  int `json:"buys"`
	Sells int `json:"sells"`
}

type volume struct {
	H24 float64 `json:"h24"`
}

type priceChange struct {
	M5  float64 `json:"m5"`
	H1  float64 `json:"h1"`
	H6  float64 `json:"h6"`
	H24 float64 `json:"h24"`
}

type liquidity struct {
	USD float64 `json:"usd"`
}

// GetTokenInfo fetches token data from DexScreener
func GetTokenInfo(ctx context.Context, tokenAddress string) (*TokenInfo, error) {
	url := fmt.Sprintf("%s/%s", DEXSCREENER_API, tokenAddress)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := SharedClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch token info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("DexScreener API error: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var dexResp dexScreenerResponse
	if err := json.Unmarshal(body, &dexResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(dexResp.Pairs) == 0 {
		return nil, fmt.Errorf("token not found on DexScreener")
	}

	// Use the first (most liquid) pair
	pair := dexResp.Pairs[0]

	return &TokenInfo{
		Address:     pair.BaseToken.Address,
		Name:        pair.BaseToken.Name,
		Symbol:      pair.BaseToken.Symbol,
		PriceUSD:    pair.PriceUSD,
		PriceSOL:    pair.PriceNative,
		Change24h:   pair.PriceChange.H24,
		Change6h:    pair.PriceChange.H6,
		Change1h:    pair.PriceChange.H1,
		Change5m:    pair.PriceChange.M5,
		Liquidity:   pair.Liquidity.USD,
		Volume24h:   pair.Volume.H24,
		Buys5m:      pair.Txns.M5.Buys,
		Sells5m:     pair.Txns.M5.Sells,
		Buys1h:      pair.Txns.H1.Buys,
		Sells1h:     pair.Txns.H1.Sells,
		PairAddress: pair.PairAddress,
		DexID:       pair.DexID,
	}, nil
}
