# ⚡ START HERE - 3 Simple Steps

## On Your NEW VPS:

### 1️⃣ Upload Files
```bash
# Upload solana_orchestrator folder to your VPS
scp -r solana_orchestrator user@your-vps:/home/user/

# Or if already there, just cd into it
cd /home/user/solana_orchestrator
```

### 2️⃣ Run Setup (One Command!)
```bash
./SETUP_VPS.sh
```

**This installs EVERYTHING:**
- Python, pip
- Playwright + Chromium (no snap!)
- All system dependencies
- Creates directories

**Takes:** 5-10 minutes

### 3️⃣ Add API Keys & Run
```bash
# Edit config
nano config/config.json

# Add your Moralis & Birdeye API keys, then save

# Run it!
python3 run.py
```

## That's It! 🎉

The bot will:
- Fetch tokens from Birdeye
- Get top holders
- Analyze wallets with beautiful display:

```
┌─ Page 0 ──────────────────────────────────┐
│ Wallet: 13H2M1C3...iGJK          ✅ PASS │
│ Win Rate:  75.2% ✓                        │
│ PnL:       215.3% ✓                       │
└────────────────────────────────────────────┘
```

Results saved in `data/good_wallets.json`

---

**For more details:** See `QUICKSTART.md`
