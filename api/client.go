package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"time"
)

type Token struct {
	TokenAddress string `json:"tokenAddress"`
}

type Holder struct {
	OwnerAddress string `json:"ownerAddress"`
	Balance      string `json:"balance"`
	USDValue     string `json:"usdValue"`
}

type Client struct {
	moralisKey      string
	fallbackKeys    []string
	birdeyeKey      string
	httpClient      *http.Client
	maxRetries      int
	currentKeyIndex int
}

func NewClient(moralisKey, birdeyeKey string, maxRetries int, fallbackKeys []string) *Client {
	return &Client{
		moralisKey:      moralisKey,
		fallbackKeys:    fallbackKeys,
		birdeyeKey:      birdeyeKey,
		httpClient:      &http.Client{Timeout: 30 * time.Second},
		maxRetries:      maxRetries,
		currentKeyIndex: 0,
	}
}

// DoRequest performs an HTTP request with retries and context cancellation
func (c *Client) DoRequest(ctx context.Context, req *http.Request) ([]byte, error) {
	var lastErr error

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff with jitter
			backoff := time.Duration(1<<attempt) * time.Second
			jitter := time.Duration(rand.Intn(1000)) * time.Millisecond
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff + jitter):
			}
		}

		// Attach context to request
		req = req.WithContext(ctx)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return body, nil
		}

		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("API error: %d", resp.StatusCode)
			continue
		}

		return nil, fmt.Errorf("API error: %d - %s", resp.StatusCode, string(body))
	}

	return nil, fmt.Errorf("max retries exceeded: %v", lastErr)
}

func (c *Client) FetchBirdeyeTokens(ctx context.Context, limit int) ([]Token, error) {
	url := fmt.Sprintf("https://public-api.birdeye.so/defi/tokenlist?sort_by=liquidity&sort_type=desc&offset=0&limit=%d&min_liquidity=100000&max_liquidity=500000", limit)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("X-API-KEY", c.birdeyeKey)
	req.Header.Set("accept", "application/json")
	req.Header.Set("x-chain", "solana")

	body, err := c.DoRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	var result struct {
		Success bool `json:"success"`
		Data    struct {
			Tokens []struct {
				Address string `json:"address"`
			} `json:"tokens"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Printf("❌ JSON Unmarshal Error: %v\nBody: %s\n", err, string(body))
		return nil, err
	}

	if !result.Success {
		fmt.Printf("❌ Birdeye API reported failure: %s\n", string(body))
	}

	tokens := make([]Token, len(result.Data.Tokens))
	for i, t := range result.Data.Tokens {
		tokens[i] = Token{TokenAddress: t.Address}
	}

	return tokens, nil
}

func (c *Client) FetchGraduatedTokens(ctx context.Context, limit int) ([]Token, error) {
	url := fmt.Sprintf("https://solana-gateway.moralis.io/token/mainnet/exchange/pumpfun/graduated?limit=%d", limit)

	// Try primary key
	apiKey := c.moralisKey
	keyName := "primary"

	for attempt := 0; attempt <= len(c.fallbackKeys); attempt++ {
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("accept", "application/json")
		req.Header.Set("X-API-Key", apiKey)

		// Use DoRequest but handle 401 specifically for key rotation
		// Note: DoRequest handles retries for 429/5xx, but here we want to switch keys on 401
		// So we might need a custom loop or just use DoRequest and check error?
		// Let's stick to the custom loop here for key rotation logic which is specific to Moralis

		// Attach context
		req = req.WithContext(ctx)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, err
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == 401 && attempt < len(c.fallbackKeys) {
			// Try fallback key
			apiKey = c.fallbackKeys[attempt]
			keyName = fmt.Sprintf("fallback #%d", attempt+1)
			fmt.Printf("⚠️ Moralis %s key failed (401), trying %s...\n",
				map[int]string{0: "primary"}[attempt], keyName)
			continue
		}

		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("API error: %d (tried %s)", resp.StatusCode, keyName)
		}

		var result struct {
			Result []struct {
				TokenAddress string `json:"tokenAddress"`
			} `json:"result"`
		}

		if err := json.Unmarshal(body, &result); err != nil {
			return nil, err
		}

		// Update current key if we switched
		if attempt > 0 {
			c.moralisKey = apiKey
			fmt.Printf("✅ Switched to Moralis %s key\n", keyName)
		}

		tokens := make([]Token, len(result.Result))
		for i, t := range result.Result {
			tokens[i] = Token{TokenAddress: t.TokenAddress}
		}

		return tokens, nil
	}

	return nil, fmt.Errorf("all Moralis API keys failed")
}

func (c *Client) FetchTopTraders(ctx context.Context, tokenAddress string) ([]string, error) {
	url := fmt.Sprintf("https://public-api.birdeye.so/defi/v2/tokens/top_traders?address=%s&time_frame=24h&sort_by=volume&sort_type=desc&offset=0&limit=100", tokenAddress)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("X-API-KEY", c.birdeyeKey)
	req.Header.Set("accept", "application/json")
	req.Header.Set("x-chain", "solana")

	body, err := c.DoRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	var result struct {
		Success bool `json:"success"`
		Data    struct {
			Items []struct {
				Owner string `json:"owner"`
			} `json:"items"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	var traders []string
	seen := make(map[string]bool)
	for _, item := range result.Data.Items {
		if item.Owner != "" && !seen[item.Owner] {
			traders = append(traders, item.Owner)
			seen[item.Owner] = true
		}
	}

	return traders, nil
}

func (c *Client) GetTokenHolders(ctx context.Context, tokenAddress string) ([]Holder, error) {
	url := fmt.Sprintf("https://solana-gateway.moralis.io/token/mainnet/%s/top-holders?limit=100", tokenAddress)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("accept", "application/json")
	req.Header.Set("X-API-Key", c.moralisKey)

	body, err := c.DoRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	var result struct {
		Result []struct {
			OwnerAddress string `json:"ownerAddress"`
			Balance      string `json:"balance"`
			USDValue     string `json:"usdValue"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	holders := make([]Holder, len(result.Result))
	for i, h := range result.Result {
		holders[i] = Holder{
			OwnerAddress: h.OwnerAddress,
			Balance:      h.Balance,
			USDValue:     h.USDValue,
		}
	}

	return holders, nil
}

type WalletToken struct {
	TokenAddress string `json:"tokenAddress"`
	Name         string `json:"name"`
	Symbol       string `json:"symbol"`
	Logo         string `json:"logo"`
	Decimals     int    `json:"decimals"`
	Balance      string `json:"balance"`
	PossibleSpam bool   `json:"possibleSpam"`
}

func (c *Client) GetWalletTokenBalances(ctx context.Context, walletAddress string) ([]WalletToken, error) {
	url := fmt.Sprintf("https://solana-gateway.moralis.io/account/mainnet/%s/tokens", walletAddress)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("accept", "application/json")
	req.Header.Set("X-API-Key", c.moralisKey)

	body, err := c.DoRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	var result []WalletToken
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	// Filter out spam tokens if needed, but for now return all
	return result, nil
}
