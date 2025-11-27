package config

import (
	"encoding/json"
	"os"
)

type Config struct {
	MoralisAPIKey       string             `json:"moralis_api_key"`
	MoralisFallbackKeys []string           `json:"moralis_fallback_keys"`
	BirdeyeAPIKey       string             `json:"birdeye_api_key"`
	AnalysisFilters     AnalysisFilters    `json:"analysis_filters"`
	APISettings         APISettings        `json:"api_settings"`
	TradingSettings     TradingSettings    `json:"trading_settings"`
	WebSocketSettings   WebSocketSettings  `json:"websocket_settings"`
	ShyftAPIKey         string             `json:"shyft_api_key"`
	FanOutEngine        FanOutEngineConfig `json:"fanout_engine"`
	Redis               RedisConfig        `json:"redis"`
	Programs            ProgramsConfig     `json:"programs"`
	Sniper              SniperConfig       `json:"sniper"`
	RateLimits          RateLimits         `json:"rate_limits"`
}

type AnalysisFilters struct {
	MinWinrate     float64 `json:"min_winrate"`
	MinRealizedPnL float64 `json:"min_realized_pnl"`
}

type APISettings struct {
	MaxRetries   int    `json:"max_retries"`
	TokenLimit   int    `json:"token_limit"`
	TokenSource  string `json:"token_source"` // "birdeye" or "moralis"
	FetchTraders bool   `json:"fetch_traders"`
}

type TradingSettings struct {
	JitoTipLamports    int64  `json:"jito_tip_lamports"`
	JitoBlockEngineURL string `json:"jito_block_engine_url"`
	JitoPrivateKey     string `json:"jito_private_key"`
	DefaultSlippageBps int    `json:"default_slippage_bps"`
	MaxSlippageBps     int    `json:"max_slippage_bps"`
}

type WebSocketSettings struct {
	ShyftWSURL       string `json:"shyft_ws_url"`
	ReconnectDelayMs int    `json:"reconnect_delay_ms"`
	PingIntervalMs   int    `json:"ping_interval_ms"`
}

type RateLimits struct {
	ShyftRPS       int `json:"shyft_rps"`
	ShyftAPIPerSec int `json:"shyft_api_per_sec"`
}

type FanOutEngineConfig struct {
	WorkerCount           int `json:"worker_count"`
	LogBufferSize         int `json:"log_buffer_size"`
	NotificationRateLimit int `json:"notification_rate_limit"`
	TelegramBatchSize     int `json:"telegram_batch_size"`
	ReconnectDelaySeconds int `json:"reconnect_delay_seconds"`
	MaxReconnectAttempts  int `json:"max_reconnect_attempts"`
}

type RedisConfig struct {
	Address      string `json:"address"`
	Password     string `json:"password"`
	DB           int    `json:"db"`
	PoolSize     int    `json:"pool_size"`
	MinIdleConns int    `json:"min_idle_conns"`
}

type ProgramsConfig struct {
	JupiterLimitOrder string `json:"jupiter_limit_order"`
	RaydiumAMMV4      string `json:"raydium_amm_v4"`
	RaydiumCLMM       string `json:"raydium_clmm"`
}

type SniperConfig struct {
	Enabled          bool     `json:"enabled"`
	MinLiquiditySOL  float64  `json:"min_liquidity_sol"`
	MaxLiquiditySOL  float64  `json:"max_liquidity_sol"`
	AutoBuyAmountSOL float64  `json:"auto_buy_amount_sol"`
	BlacklistTokens  []string `json:"blacklist_tokens"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// Set defaults if not specified
	if cfg.FanOutEngine.WorkerCount == 0 {
		cfg.FanOutEngine.WorkerCount = 20
	}
	if cfg.FanOutEngine.LogBufferSize == 0 {
		cfg.FanOutEngine.LogBufferSize = 50000
	}
	if cfg.Redis.Address == "" {
		cfg.Redis.Address = "localhost:6379"
	}
	if cfg.Redis.PoolSize == 0 {
		cfg.Redis.PoolSize = 50
	}

	return &cfg, nil
}
