# Solana Wallet Scanner Bot

A Telegram bot that automatically scans Solana wallets for profitable traders by analyzing their win rate and PnL (Profit and Loss). The bot fetches tokens from Moralis or Birdeye APIs, retrieves top holders and traders, and uses Playwright to scrape wallet analytics from DexCheck.

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [File Structure](#file-structure)
- [Detailed File Documentation](#detailed-file-documentation)
- [Configuration](#configuration)
- [Setup & Running](#setup--running)
- [Data Flow](#data-flow)

---

## 🚀 Features

### Wallet Scanner (Active)
- Automatically scans Solana wallets for profitable traders
- Fetches tokens from Moralis (graduated PumpFun) or Birdeye (liquidity-based)
- Analyzes wallet win rate and PnL using DexCheck
- Real-time progress updates
- SQLite persistence with 5-hour data retention
- Concurrent analysis with Playwright (6 pages)

### Trading Features (Foundation Complete) 🆕
The bot now includes foundational infrastructure for trading capabilities:

**Core Components:**
- ✅ **WebSocket Client** - Real-time updates via Shyft
  - Rate limiting (20 RPS, 1 API call/sec)
  - Auto-reconnection with exponential backoff
  - Ping/pong keepalive
  - Account balance subscriptions

- ✅ **Balance Manager** - Track wallet holdings
  - SOL balance checking
  - Token balance enumeration
  - Real-time balance updates via WebSocket
  - Multi-wallet support ready

- ✅ **Jito Integration** - MEV-protected trading
  - Bundle creation and submission
  - Dynamic tip calculation
  - Jito block engine integration
  - Transaction monitoring

**Coming Soon:**
- 💰 Live balance tracking
- 💱 Buy/sell tokens with Jito
- 📊 Position tracking
- 📈 Trade history
- 👛 Multi-wallet management

### API Integration
- **Moralis API** - Token & holder data with automatic fallback
- **Birdeye API** - Market data & top traders
- **Shyft WebSocket** - Real-time blockchain updates
- **DexCheck** - Wallet analytics scraping

### Latest Features (November 2025) 🆕

#### Fan-Out Engine Architecture (New) 🚀
- **Single WebSocket Connection**: Replaces per-wallet subscriptions with global program monitoring.
- **Redis Integration**: O(1) wallet lookups using Redis Sets.
- **Worker Pool**: 20 concurrent workers for high-throughput log processing.
- **Jito Bundles**: MEV-protected trade execution.
- **Scalability**: Capable of handling 2,000+ users on an 8GB VPS.

#### TUI Dashboard Monitor 🆕
- **Real-Time Monitoring**: Visual dashboard for bot health
- **Live Metrics**: CPU, RAM, Uptime, Active Users, Wallets Scanned
- **Log Streaming**: Live tail of bot logs within the dashboard
- **One-Command Launch**: Automatically starts with `./run.sh`

#### Copy Trading System
- **Real-Time Monitoring**: Track profitable wallets via Shyft WebSockets
- **Auto-Copy Trades**: Automatically mirror buy orders from target wallets
- **Configurable Settings**:
  - Set custom SOL amount per trade
  - Global Auto-Buy toggle in Settings
  - Add/remove targets dynamically
- **Safety Features**: Alert-only mode for testing

#### Shyft API Integration
- **Token Metadata**: Fetches name, symbol, and URI directly from chain
- **Supply Data**: Shows total supply with Metaplex PDA decoding
- **Fallback System**: Uses Shyft when DexScreener doesn't have token data
- **Buy Flow Enhancement**: Better support for newly launched tokens

#### Professional UI Redesign
- **Modern Design**: Unicode borders, section dividers, visual hierarchy
- **Cleaner Layout**: One primary action per row
- **Organized Sections**: Balance Overview, Account Status, etc.
- **Better Formatting**: Professional labels and consistent spacing
- **Enhanced Copy Trade UI**: Boxed headers and clear target display

#### User Experience Improvements
- **Balance Refresh**: Instant balance updates with refresh button
- **Top Up Button**: Quick access to credit purchase info
- **Settings Menu**: Copy Trade Auto-Buy toggle
- **Better Error Handling**: Graceful fallbacks for API failures

#### Infrastructure
- **Duplicate Prevention**: Enhanced run.sh with orphan process cleanup
- **WebSocket Logs**: logsSubscribe support for transaction monitoring
- **Database Updates**: Copy trade targets table, user settings enhancements

---

## Overview

The bot operates in a continuous loop:
1. Fetches tokens from Moralis (graduated PumpFun tokens) or Birdeye (liquidity-based)
2. Retrieves top holders for each token
3. Optionally fetches top traders for each token
4. Analyzes wallets using Playwright to scrape DexCheck
5. Saves profitable wallets (meeting min WR/PnL criteria) to SQLite database
6. Provides Telegram interface for users to search wallets with custom filters

**Key Features:**
- 24/7 continuous scanning
- Real-time wallet analysis with Playwright
- SQLite persistence with 5-hour data retention
- Multi-source token fetching (Moralis/Birdeye)
- Concurrent wallet analysis (6 pages)
- Telegram bot interface for queries

---

## Architecture

```
┌─────────────────┐
│  Telegram Bot   │ (User Interface)
└────────┬────────┘
         │
         ▼
┌─────────────────┐       ┌─────────────────┐
│ Fan-Out Engine  │◄──────│  Redis (Cache)  │
└────────┬────────┘       └─────────────────┘
         │
         ├──► Shyft WebSocket (Single Connection)
         │    └──► Subscribe to Program Logs (Jupiter/Raydium)
         │
         ├──► Worker Pool (20 Workers)
         │    └──► Process Logs & Match Wallets (O(1) Lookup)
         │
         └──► Executor
              └──► Execute Copy Trades (Jito Bundles)

┌─────────────────┐
│ Scanner Service │ (Background Analysis)
└────────┬────────┘
         │
         └──► Storage (SQLite)
```

---

## File Structure

```
/home/user/sol/
├── telegram-bot.go          # Main bot logic & Telegram handlers
├── bot_handlers.go          # General bot commands (balance, wallets)
├── wallet_handlers.go       # Wallet generation & encryption
├── buy_handlers.go          # Buy flow handlers
├── sell_handlers.go         # Sell flow handlers
├── settings_handlers.go     # User settings handlers
├── main.go                  # Standalone orchestrator (legacy)
├── run.sh                   # Build & run script
├── go.mod                   # Go dependencies
├── go.sum                   # Dependency checksums
├── bot.db                   # SQLite database
├── Makefile                 # Build automation
├── TELEGRAM-BOT.md          # Bot setup instructions
│
├── analyzer/
│   └── analyzer.go          # Playwright wallet analyzer
│
├── api/
│   └── client.go            # API client (Moralis/Birdeye)
│
├── config/
│   ├── config.go            # Configuration loader
│   └── config.json          # App configuration
│
├── storage/
│   ├── db.go                # SQLite database interface
│   ├── encrypted_wallet.go  # Secure wallet storage
│   ├── wallet_manager.go    # Wallet CRUD operations
│   └── settings.go          # User settings storage
│
├── trading/                 # Trading infrastructure
│   ├── balance.go           # Balance checking
│   ├── dexscreener.go       # Price data
│   ├── jito.go              # MEV protection
│   ├── jupiter.go           # Swap aggregation
│   └── websocket.go         # Real-time updates
│
├── data/                    # Output directory (JSON files)
│   ├── tokens.json
│   ├── holders.json
│   └── good_wallets.json
│
├── bin/                     # Compiled binaries
│   ├── telegram-bot
│   └── orchestrator
│
├── scripts/                 # Helper scripts
│   ├── setup.sh
│   ├── setup-telegram.sh
│   ├── start-bot.sh
│   └── run-telegram.sh
│
├── legacy_python/           # Original Python implementation
│   └── solana_orchestrator/
│
└── solana-trading-cli/      # External trading CLI (unused)
```

---

## Detailed File Documentation

### 1. `telegram-bot.go` (Main Application)

**Purpose:** Telegram bot server with continuous wallet scanning.

**Key Components:**

#### Structures
- `Scanner`: Manages scanning state, DB connection, and in-memory cache
  - `db`: SQLite database connection
  - `scannedCount`: Number of wallets analyzed in current cycle
  - `totalWallets`: Total wallets to analyze
  - `lastScanStart`: Unix timestamp of last scan
  - `isScanning`: Boolean flag for scan status
  - `walletsCache`: In-memory map for fast lookups

- `UserSession`: Tracks user interaction state
  - `State`: Current state (e.g., "awaiting_winrate", "awaiting_pnl")
  - `Winrate`: Temporary storage for user input
  - `RequestedAt`: Request timestamp
  - `StartCount`: Wallet count when user made request

#### Main Functions

**`main()`**
- Loads configuration from `config/config.json`
- Initializes SQLite database
- Loads existing wallets into memory cache
- Starts Telegram bot API connection
- Launches background goroutines:
  - `continuousScanner()`: Infinite scanning loop
  - `cleanupRoutine()`: Hourly cleanup of old data
- Listens for Telegram updates (messages/callbacks)

**`continuousScanner(cfg, bot)`**
- Infinite loop that:
  1. Fetches tokens (Moralis or Birdeye based on config)
  2. Retrieves holders for each token
  3. Optionally fetches top traders
  4. Builds unique wallet set
  5. Initializes Analyzer with config filters
  6. Analyzes wallets concurrently
  7. Saves results to DB via callback
  8. Sleeps 30 minutes between cycles

**`cleanupRoutine(db)`**
- Runs every hour
- Deletes wallet records older than 5 hours
- Maintains the 5-hour data retention policy

#### Telegram Handlers

**`handleMessage(bot, message)`**
- Routes commands:
  - `/start`: Show welcome screen with Dev Finder button
  - `/status`: Show scanner status (total wallets, last scan time)
- Handles user input for Dev Finder workflow:
  - `awaiting_winrate`: User entering min win rate
  - `awaiting_pnl`: User entering min PnL

**`handleCallback(bot, callback)`**
- Handles button clicks:
  - `dev_finder`: Initiates wallet search workflow

**`startDevFinder(bot, chatID)`**
- Creates user session
- Prompts for minimum Win Rate (25-100%)
- Records starting wallet count for progress tracking

**`handleWinrateInput(bot, msg)`**
- Validates win rate input (25-100)
- Transitions to PnL input state

**`handlePnlInput(bot, msg)`**
- Validates PnL input (≥25)
- Calls `searchAndRespond()` with filters

**`searchAndRespond(bot, chatID, winrate, pnl, startCount)`**
- Searches in-memory cache for matching wallets
- If found: Displays results immediately
- If not found: Shows progress message, starts polling updates

**`updateProgress(bot, chatID, messageID, winrate, pnl, startCount)`**
- Updates progress message every 10 seconds for up to 5 minutes
- Shows progress bar, wallet count, scan status
- Stops when matching wallets are found

**`sendResults(bot, chatID, matches, winrate, pnl)`**
- Formats and sends matching wallets (max 15 shown)
- Displays wallet address, Win Rate, PnL

**`sendStatus(bot, chatID)`**
- Shows scanner statistics:
  - Status (Scanning/Idle)
  - Total wallets in cache
  - Last scan count
  - Time since last scan

---

### 2. `main.go` (Standalone Orchestrator)

**Purpose:** Command-line orchestrator for one-time wallet analysis.

**Functionality:**
- Parses CLI flags (`-limit`, `-pages`, `-config`)
- Fetches tokens and holders
- Analyzes wallets using Playwright
- Saves results to JSON files in `data/`
- Used for testing or manual analysis

**Not used** when running the Telegram bot via `run.sh`.

---

### 3. `analyzer/analyzer.go`

**Purpose:** Wallet analysis engine using Playwright to scrape DexCheck.

#### Structure: `Analyzer`
- `numPages`: Number of concurrent Playwright browser pages (default: 6)
- `minWinrate`: Filter threshold (e.g., 25%)
- `minRealizedPnL`: Filter threshold (e.g., 25%)
- `scannedWallets`: Sync map to prevent duplicate scans

#### Key Functions

**`NewAnalyzer(numPages, minWinrate, minRealizedPnL)`**
- Constructor that creates analyzer with specified parameters

**`AnalyzeWallets(ctx, wallets, onResult)`**
- Main analysis function
- Launches Chromium browser (headless mode)
- Creates `numPages` concurrent workers
- Each worker:
  1. Takes wallet from channel
  2. Calls `analyzeWallet()`
  3. Filters by min WR/PnL
  4. Calls `onResult()` callback if passes
  5. Logs result with wallet stats

**`analyzeWallet(page, wallet)`**
- Navigates to `https://dexcheck.ai/app/wallet-analyzer/{wallet}`
- Waits for "Win Rate" text to appear (20s timeout)
- Retries up to 5 times if data is still loading (SVG skeleton detection)
- Extracts HTML content
- Parses Win Rate and Realized PnL using regex
- Returns `WalletStats` struct

**`extractWinrate(html)`**
- Regex: `Win Rate</h3><p...text-2xl...>XX.XX%</p>`
- Parses percentage as float64
- Returns 0 if not found

**`extractRealizedPnL(html)`**
- Regex: `Realized</p><p...>-?$XXX <span...>(-?XX.XX%)</span>`
- Handles both positive and negative dollar amounts
- Handles both positive and negative percentages
- Logs debug message if parsing fails (for troubleshooting)
- Returns 0 if not found

---

### 4. `api/client.go`

**Purpose:** HTTP client for Moralis and Birdeye APIs.

#### Structure: `Client`
- `moralisKey`: API key for Moralis
- `birdeyeKey`: API key for Birdeye
- `httpClient`: HTTP client with 30s timeout
- `maxRetries`: Retry count for failed requests

#### Functions

**`NewClient(moralisKey, birdeyeKey, maxRetries)`**
- Creates client with API keys and retry configuration

**`FetchBirdeyeTokens(limit)`**
- Endpoint: `https://public-api.birdeye.so/defi/tokenlist`
- Parameters:
  - `sort_by=liquidity&sort_type=desc`
  - `min_liquidity=100000&max_liquidity=500000`
  - `limit={limit}` (max 50)
- Returns list of token addresses
- Logs response on error for debugging

**`FetchGraduatedTokens(limit)`**
- Endpoint: `https://solana-gateway.moralis.io/token/mainnet/exchange/pumpfun/graduated`
- Parameters: `limit={limit}`
- Returns PumpFun tokens that have "graduated" to Raydium
- No retry logic (single attempt)

**`GetTokenHolders(tokenAddress)`**
- Endpoint: `https://solana-gateway.moralis.io/token/mainnet/{address}/top-holders`
- Parameters: `limit=100`
- Retry logic:
  - Retries on network errors
  - Handles 429 (rate limit) with exponential backoff
  - Sleeps 2^attempt seconds between retries
- Returns top 100 holders with balance and USD value

**`FetchTopTraders(tokenAddress)`**
- Endpoint: `https://public-api.birdeye.so/defi/v2/tokens/top_traders`
- Parameters:
  - `time_frame=24h`
  - `sort_by=volume&sort_type=desc`
  - `limit=100`
- Returns unique trader wallet addresses
- Deduplicates results using map

---

### 5. `config/config.go`

**Purpose:** Configuration management.

#### Structures

**`Config`**
- `MoralisAPIKey`: Moralis API key
- `BirdeyeAPIKey`: Birdeye API key
- `AnalysisFilters`: Filter thresholds
- `APISettings`: API configuration

**`AnalysisFilters`**
- `MinWinrate`: Minimum win rate % (default: 25)
- `MinRealizedPnL`: Minimum PnL % (default: 25)

**`APISettings`**
- `MaxRetries`: API retry count (default: 3)
- `TokenLimit`: Tokens to fetch per cycle (max 50 for Birdeye)
- `TokenSource`: "birdeye" or "moralis"
- `FetchTraders`: Boolean to enable top traders fetching

#### Function

**`Load(path)`**
- Reads JSON file from specified path
- Unmarshals into Config struct
- Returns error if file missing or invalid JSON

---

### 6. `config/config.json`

**Purpose:** Runtime configuration file.

**Current Settings:**
```json
{
  "moralis_api_key": "...",
  "birdeye_api_key": "...",
  "analysis_filters": {
    "min_winrate": 25,
    "min_realized_pnl": 25
  },
  "api_settings": {
    "max_retries": 3,
    "token_limit": 50,
    "token_source": "moralis",
    "fetch_traders": true
  }
}
```

**Notes:**
- `token_limit`: Max 50 for Birdeye API
- `token_source`: Switch between "birdeye" (liquidity) or "moralis" (graduated)
- `fetch_traders`: Adds top traders to wallet list (increases scan count)
- API keys required for operation

---

### 7. `storage/db.go`

**Purpose:** SQLite database interface for wallet persistence.

#### Structures

**`DB`**
- Embeds `*sql.DB` for database operations

**`WalletData`**
- `Wallet`: Solana wallet address
- `Winrate`: Win rate percentage
- `RealizedPnL`: PnL percentage
- `ScannedAt`: Unix timestamp

**`Alert`** (Unused - alerts feature removed)
- Legacy structure for alert system

#### Database Schema

**Table: `wallets`**
```sql
CREATE TABLE wallets (
    wallet TEXT PRIMARY KEY,
    winrate REAL,
    realized_pnl REAL,
    scanned_at INTEGER
)
```

**Table: `alerts`** (Unused)
- Legacy table from removed alerts feature

#### Functions

**`New(path)`**
- Opens SQLite database at specified path
- Creates `wallets` and `alerts` tables if not exist
- Returns DB instance

**`SaveWallet(w)`**
- Inserts wallet or updates if exists (UPSERT)
- Updates winrate, PnL, and timestamp on conflict
- Used in scanner callback for real-time saves

**`GetWallets()`**
- Queries wallets scanned in last 5 hours
- Orders by `realized_pnl DESC` (highest PnL first)
- Returns slice of WalletData pointers

**`CleanupOldData()`**
- Deletes wallets older than 5 hours
- Called by hourly cleanup routine
- Returns number of deleted records

**`CreateAlert()`, `GetMatchingAlerts()`**
- Legacy functions from removed alerts feature
- Not actively used

---

### 9. Trading & Wallet Management

#### Handlers

**`wallet_handlers.go`**
- **Purpose**: Manages wallet generation, import, and security.
- **Key Functions**:
  - `handleGenerateWallet`: Initiates secure wallet generation flow.
  - `handleConfirmGenerate`: Creates new keypair and mnemonic.
  - `handleWalletPassword`: Encrypts private key with user password using AES-256-GCM.
  - `handleImportWallet`: Supports importing via Private Key or Seed Phrase.

**`buy_handlers.go`**
- **Purpose**: Handles the token purchase workflow.
- **Key Functions**:
  - `handleStartBuy`: Initiates buy flow.
  - `handleBuyTokenInput`: Validates token address and fetches info from DexScreener.
  - `handleBuyAmountInput`: Checks SOL balance (via Shyft) and calculates estimated output.
  - `handleConfirmBuy`: Executes swap via Jupiter (placeholder for now).

**`sell_handlers.go`**
- **Purpose**: Handles token selling workflow.
- **Key Functions**:
  - `handleStartSell`: Fetches and displays user's token holdings with prices.
  - `handleSellToken`: Shows sell options (25%, 50%, 75%, 100%).
  - `handleSellPercentage`: Calculates sell amount and estimated return.

**`settings_handlers.go`**
- **Purpose**: Manages user trading preferences.
- **Key Functions**:
  - `handleSettings`: Displays current settings (Slippage, Jito Tip).
  - `handleSetSlippage`: Updates slippage tolerance (bps).
  - `handleSetJito`: Updates Jito tip amount for MEV protection.

#### Trading Package (`trading/`)

**`balance.go`**
- **Purpose**: Manages wallet balances.
- **Features**:
  - Fetches SOL balance via RPC.
  - Fetches SPL token balances via RPC.
  - `GetFullBalance`: Aggregates all assets.

**`dexscreener.go`**
- **Purpose**: Fetches real-time market data.
- **Features**:
  - `GetTokenInfo`: Retrieves price, liquidity, volume, and price changes from DexScreener API.

**`jito.go`**
- **Purpose**: MEV-protected transaction submission.
- **Features**:
  - `SubmitBundle`: Sends transactions to Jito Block Engine.
  - `CreateTipInstruction`: Adds tip to random Jito validator account.

**`jupiter.go`**
- **Purpose**: Aggregates swap routes.
- **Features**:
  - `GetBuyQuote` / `GetSellQuote`: Fetches best price quotes.
  - `GetSwapTransaction`: Generates unsigned swap transaction.

**`websocket.go`**
- **Purpose**: Real-time blockchain updates via Shyft.
- **Features**:
  - Robust connection management with auto-reconnect.
  - Rate limiting (20 RPS, 1 API/sec).
  - `SubscribeAccount`: Listens for balance changes.

---

### 8. `run.sh`

**Purpose:** Build and run script for Telegram bot.

**Functionality:**
1. Sets `GOPATH` environment variable
2. Sets `TELEGRAM_BOT_TOKEN` (hardcoded - should be env var)
3. Builds `telegram-bot.go` into `bin/telegram-bot`
4. Executes the binary

**Usage:**
```bash
./run.sh
```

**Security Note:** Token is hardcoded - should use environment variable instead.

---

## Configuration

### API Keys Required

1. **Moralis API Key**
   - Get from: https://moralis.io/
   - Used for: Graduated PumpFun tokens & holder data

2. **Birdeye API Key**
   - Get from: https://birdeye.so/
   - Used for: Liquidity-based tokens & top traders

3. **Telegram Bot Token**
   - Get from: @BotFather on Telegram
   - Set in: `run.sh` (should be environment variable)

### Configuration Options

Edit `config/config.json` to adjust:
- **Analysis Filters**: Min win rate & PnL thresholds
- **Token Source**: Choose between Moralis or Birdeye
- **Token Limit**: Number of tokens per scan cycle
- **Fetch Traders**: Enable/disable top trader fetching
- **Max Retries**: API failure retry count

---

## Setup & Running

### Prerequisites
- Go 1.21+
- Playwright (auto-installed on first run)
- SQLite3

### Installation
 
 1. **One-Command Setup:**
    ```bash
    ./run.sh install
    ```
    This will:
    - Check and install system dependencies (Go, Git, Curl, etc.)
    - Install Playwright browsers
    - Download Go modules

 2. **Configure:**
    - Edit `config/config.json` with your API keys
    - Edit `run.sh` with your Telegram bot token

 3. **Run:**
    ```bash
    ./run.sh
    ```
    This will build the bot, start it in the background, and launch the TUI dashboard.

### First Run
- Creates `bot.db` SQLite database
- Starts continuous scanning loop
- Bot becomes available on Telegram

---

## Data Flow

### Scanning Cycle (Every 30 Minutes)

```
1. Fetch Tokens
   ├─ Source: Moralis OR Birdeye (config.json)
   └─ Result: List of token addresses

2. Fetch Holders & Traders
   ├─ For each token:
   │  ├─ Get top 100 holders (Moralis)
   │  └─ Get top traders (Birdeye, if enabled)
   └─ Result: Unique wallet set (3000-4000 wallets)

3. Analyze Wallets (6 concurrent workers)
   ├─ For each wallet:
   │  ├─ Navigate to DexCheck
   │  ├─ Wait for data to load
   │  ├─ Extract Win Rate & PnL
   │  └─ Apply filters (min WR/PnL)
   └─ Result: Profitable wallets

4. Save to Database
   ├─ Save to SQLite (UPSERT)
   ├─ Update in-memory cache
   └─ Auto-cleanup after 5 hours

5. Sleep 30 minutes, repeat
```

### User Query Flow

```
1. User: /start
   └─ Bot: Show Dev Finder button

2. User: Click "Dev Finder"
   └─ Bot: "Enter minimum Win Rate (25-100):"

3. User: Enter winrate (e.g., 60)
   └─ Bot: "Enter minimum PnL (e.g., 100):"

4. User: Enter PnL (e.g., 50)
   └─ Bot: Search in-memory cache

5A. If Found:
    └─ Bot: Display matching wallets immediately

5B. If Not Found:
    ├─ Bot: Show progress message
    ├─ Update every 10 seconds
    └─ Wait for scanner to find matches
```

---

## Troubleshooting

### Common Issues

**1. API 401 Error (Unauthorized)**
- Check API keys in `config/config.json`
- Verify keys are valid and not expired

**2. No Wallets Found**
- Check `min_winrate` and `min_realized_pnl` values
- Lower thresholds to see more results
- Verify DexCheck website structure hasn't changed

**3. Playwright Installation Issues**
```bash
go run github.com/playwright-community/playwright-go/cmd/playwright@latest install --with-deps chromium
```

**4. Database Locked**
- Only one instance should run at a time
- Kill existing processes: `pkill telegram-bot`

---

## Development Notes

### Recent Changes
- ✅ Removed `/list` and `/alert` commands (simplified to Dev Finder only)
- ✅ Improved PnL regex to handle negative values
- ✅ Added loading state detection in Playwright
- ✅ Organized workspace (binaries in `bin/`, scripts in `scripts/`)
- ✅ Updated formatting to show 2 decimal places (matches DexCheck)
- ✅ Scanner now uses config filters (only saves wallets ≥25% WR/PnL)

### Future Enhancements
- [ ] Environment variable for Telegram token
- [ ] Configurable concurrent page count
- [ ] Export results to CSV
- [ ] Web dashboard for monitoring
- [ ] Multiple filter presets
