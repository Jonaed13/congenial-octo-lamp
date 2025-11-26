# Feature Status & Roadmap

This document outlines features that are currently implemented in the codebase but may not be fully reflected in the main `README.md`, as well as planned improvements and future roadmap items.

## ✅ Implemented Features (Undocumented)

The following features are fully implemented in the code and available for use, despite being marked as "Coming Soon" or "Placeholder" in the main documentation.

### 1. Trading Engine with Jito Integration
- **Status**: **Active**
- **Files**: `buy_handlers.go`, `sell_handlers.go`, `trading/jito.go`
- **Description**: 
  - Complete Buy/Sell workflow is implemented.
  - **MEV Protection**: Transactions can be submitted via Jito Bundles to protect against sandwich attacks.
  - **Dynamic Tipping**: Users can configure Jito tips (bribes) for faster inclusion.
  - **Jupiter Aggregation**: Swaps are routed through Jupiter for best price execution.

### 2. Wallet Security & Management
- **Status**: **Active**
- **Files**: `wallet_handlers.go`, `storage/encrypted_wallet.go`
- **Description**:
  - **Encryption**: Private keys are AES-256-GCM encrypted with a user-provided password.
  - **Import/Export**: Support for importing wallets via Private Key or Seed Phrase.
  - **Non-Custodial**: Keys are stored locally (encrypted) and never transmitted in plain text.

### 3. User Settings
- **Status**: **Active**
- **Files**: `settings_handlers.go`
- **Description**:
  - **Slippage Control**: Users can set custom slippage tolerance (default 5%).
  - **Jito Tips**: Configurable tip amounts for MEV protection.

### 4. Real-Time Balance Tracking
- **Status**: **Active**
- **Files**: `trading/balance.go`, `trading/websocket.go`
- **Description**:
  - **Shyft Integration**: Uses Shyft RPC and WebSocket for real-time balance updates.
  - **Portfolio View**: Aggregates SOL and SPL token balances.

---

## 🚀 Planned Features (Roadmap)

The following features are planned for future development, based on architectural analysis and improvement proposals.

### Short Term: Scalability & Performance
- **SQLite WAL Mode**: Enable Write-Ahead Logging for better concurrency.
- **Connection Pooling**: Optimize database connections to reduce overhead.
- **Rate Limiting**: Implement per-user rate limits to prevent abuse.
- **LRU Caching**: Replace simple maps with LRU caches to manage memory usage.

### Medium Term: Infrastructure
- **PostgreSQL Migration**: Move from SQLite to PostgreSQL for true concurrency and network access.
- **Redis Caching**: Centralized cache for sharing state across multiple bot instances.
- **Prometheus Metrics**: detailed monitoring of scanner performance and API latency.

### Long Term: Advanced Trading
- **Multi-Wallet UI**: Enable switching between multiple active wallets (backend support exists).
- **Advanced Strategies**: Integration with `solana-trading-cli` for:
  - **Copy Trading**: Mirroring successful wallets found by the scanner.
  - **Sniping**: Automated buying of new pool launches.
  - **Limit Orders**: Storing and executing orders at specific prices.
- **Microservices**: Splitting the bot into separate Scanner, API, and Execution services.
