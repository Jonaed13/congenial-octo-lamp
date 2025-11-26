# 🚀 Solana Token & Wallet Analysis Orchestrator

A beautiful, intelligent orchestrator system for analyzing Solana tokens and wallets.

## Features

- 🪙 **Multi-Source Token Fetching**: Supports both Birdeye and Moralis APIs
- 👥 **Holder Analysis**: Gets top holders for each token
- 🎭 **Concurrent Scanning**: Playwright multi-page wallet analysis
- 💎 **Smart Filtering**: Configurable win rate and PnL thresholds
- 🔁 **Auto-Loop Mode**: Continuous monitoring and analysis
- 📊 **Beautiful UI**: Color-coded terminal interface with progress bars
- 💾 **Resume Capability**: Pick up where you left off

## Quick Start

1. **Install Dependencies**:
   ```bash
   cd solana_orchestrator
   pip install -r requirements.txt
   playwright install chromium
   ```

2. **Configure API Keys**:
   Edit `config/config.json` and add your API keys:
   - `moralis_api_key`: Your Moralis API key
   - `birdeye_api_key`: Your Birdeye API key

3. **Run Interactive Mode**:
   ```bash
   python run.py
   ```

## Directory Structure

```
solana_orchestrator/
├── run.py                 # Main launcher script
├── README.md             # This file
├── requirements.txt      # Python dependencies
├── core/                 # Core application files
│   ├── orchestrator.py          # Main orchestrator logic
│   ├── playwright_multi_page_analyzer.py  # Concurrent wallet analyzer
│   └── playwright_scanner.py    # Single wallet scanner
├── utils/                # Utility modules
│   └── beautiful_logger.py     # Color-coded logging
├── config/               # Configuration files
│   └── config.json             # API keys and settings
├── logs/                 # Log files
│   └── orchestrator.log        # Application logs
└── data/                 # Data files (auto-generated)
    ├── tokens.json             # Fetched tokens
    ├── tokens.txt              # Token addresses
    ├── holders.json            # Token holders
    ├── holders.txt             # Holder addresses
    ├── owner_addresses.txt     # Compatible addresses
    ├── good_wallets.json       # Profitable wallets
    └── good_wallets.txt        # Profitable wallet addresses
```

## Usage Examples

### Interactive Mode (Recommended)
```bash
python run.py
```
Provides a beautiful menu to configure all settings.

### Command Line Mode
```bash
# Basic run with 5 pages, Birdeye tokens
python run.py --non-interactive --pages 5 --token-source birdeye

# Auto-loop every hour with specific filters
python run.py --non-interactive --loop 60 --min-winrate 80 --min-pnl 150

# Clean restart with custom settings
python run.py --clean --limit 50 --pages 3 --token-source moralis
```

### Resume from Specific Token
```bash
python run.py --resume 42
```

## Configuration

The `config/config.json` file contains:
- **API Keys**: Moralis and Birdeye API credentials
- **Analysis Filters**: Win rate and PnL thresholds
- **Scanning Limits**: Token and holder limits
- **Playwright Settings**: Browser and scanning configuration

## Data Flow

1. **Token Fetching**: Gets tokens from Birdeye or Moralis
2. **Holder Collection**: Retrieves top holders for each token
3. **Wallet Analysis**: Scans wallets using Playwright concurrently
4. **Filtering**: Applies win rate and PnL criteria
5. **Results**: Saves profitable wallets to files

## Output Files

- `data/tokens.json`: Complete token information
- `data/holders.json`: Detailed holder data
- `data/good_wallets.json`: Filtered profitable wallets
- `data/owner_addresses.txt`: Wallet addresses for external tools
- `logs/orchestrator.log`: Detailed application logs

## Auto-Loop Mode

Enable continuous monitoring:
- Fetches fresh tokens periodically
- Scans new wallets that haven't been processed
- Prevents duplicate scanning
- Configurable intervals (minutes to hours)

## Performance Tips

- **Concurrent Pages**: 3-5 for most systems, 7-10 for powerful PCs
- **Token Limits**: Start with 50-100 tokens for testing
- **Memory**: Each browser page uses ~150MB RAM
- **Network**: Rate limiting prevents API overload

## Troubleshooting

- **API Errors**: Check your API keys in `config/config.json`
- **Browser Issues**: Run `playwright install chromium`
- **Memory Issues**: Reduce concurrent pages
- **Clean Start**: Use `--clean` flag to delete all data

## API Sources

- **Birdeye** (`birdeye_api_key` required): Liquidity-based token selection
- **Moralis** (`moralis_api_key` required): PumpFun graduated tokens and holders

---

*Built with ❤️ for the Solana community*