# 🚀 Solana Orchestrator - Quick Start Guide

## Fresh VPS Setup (From Scratch)

### Step 1: Upload Files to VPS
```bash
# On your local machine, upload the entire folder to your VPS:
scp -r solana_orchestrator user@your-vps-ip:/home/user/
```

Or clone from git if you have it there:
```bash
git clone <your-repo-url>
cd solana_orchestrator
```

### Step 2: Run Setup Script
```bash
cd /home/user/solana_orchestrator
chmod +x SETUP_VPS.sh
./SETUP_VPS.sh
```

**The script will:**
- ✅ Update system packages
- ✅ Install Python 3.10+
- ✅ Install all system dependencies for Chromium
- ✅ Install Python packages (playwright, requests, etc.)
- ✅ Install Playwright's Chromium browser
- ✅ Create necessary directories
- ✅ Verify everything works

**Time:** ~5-10 minutes depending on VPS speed

### Step 3: Configure API Keys
```bash
nano config/config.json
```

Add your API keys:
```json
{
  "moralis_api_key": "YOUR_MORALIS_KEY_HERE",
  "birdeye_api_key": "YOUR_BIRDEYE_KEY_HERE",
  "api_settings": {
    "max_retries": 3,
    "token_limit": 100
  },
  "analysis_filters": {
    "min_winrate": 70.0,
    "min_realized_pnl": 100.0
  }
}
```

Save and exit (Ctrl+X, then Y, then Enter)

### Step 4: Run the Bot!

**Interactive Mode (Recommended for first time):**
```bash
python3 run.py
```

**Quick Start with CLI:**
```bash
# Start with 10 pages, relaxed filters
python3 run.py --pages 10 --min-winrate 50 --min-pnl 10 --limit 20
```

**Production Settings:**
```bash
# Strict filters, many pages, full batch
python3 run.py --pages 10 --min-winrate 70 --min-pnl 100 --limit 100 --token-source birdeye
```

## What You'll See

The bot will show beautiful output like this:

```
┌─ Page 0 ─────────────────────────────────────────┐
│ Wallet: 13H2M1C3...iGJK                    ✅ PASS │
│ Win Rate:  75.2% ✓                                │
│ PnL:       215.3% ✓                               │
└────────────────────────────────────────────────────┘
```

- **Green ✓** = Criterion met
- **Red ✗** = Criterion not met (shows what's needed)
- **✅ PASS** = Wallet saved to results
- **❌ FAIL** = Wallet skipped

## Results

Good wallets are saved in:
- `data/good_wallets.json` - Full data
- `data/good_wallets.txt` - Wallet addresses only

## Performance

With your VPS and our optimizations:
- **Per wallet:** ~40-45 seconds total (includes waiting)
- **With 10 pages:** Process 10 wallets concurrently
- **100 wallets:** ~6-8 minutes with 10 pages

## Troubleshooting

### "Playwright not found"
```bash
pip3 install playwright
python3 -m playwright install chromium
```

### "Permission denied"
```bash
chmod +x SETUP_VPS.sh
chmod +x run.py
```

### "Config not found"
```bash
cp config/config.json.example config/config.json
nano config/config.json
```

### "No wallets found"
Your filters might be too strict. Try:
```bash
python3 run.py --min-winrate 40 --min-pnl 10
```

## Command Line Options

```bash
python3 run.py [OPTIONS]

Options:
  --non-interactive          Skip menu, use CLI args
  --token-source             birdeye or moralis (default: birdeye)
  --limit NUM               Number of tokens to fetch (default: 100)
  --pages NUM               Concurrent browser pages (default: 3)
  --min-winrate NUM         Min win rate % (default: 70)
  --min-pnl NUM             Min PnL % (default: 100)
  --clean                   Delete all data files first
  --loop MINUTES            Auto-loop mode, run every N minutes
```

## Examples

**Test run (fast, relaxed):**
```bash
python3 run.py --limit 10 --pages 5 --min-winrate 40 --min-pnl 10
```

**Production run (strict):**
```bash
python3 run.py --limit 100 --pages 10 --min-winrate 70 --min-pnl 100
```

**Auto-loop (runs every hour):**
```bash
python3 run.py --loop 60 --pages 10
```

## Support

- Check `logs/orchestrator.log` for detailed logs
- Run with `--clean` to start fresh
- Each wallet takes ~40-45 seconds to fully analyze

---

**Ready to find profitable Solana wallets!** 🚀
