# 🤖 Bot Quick Reference Card

## ✅ Current Status: RUNNING
- **Bot:** @Afnexbot
- **PID:** 51215
- **Database:** 1,070+ wallets

---

## 🚀 Quick Commands

### Check Status
```bash
cd /workspaces/persistent_user/sol/sol
./check-bot-status.sh
```

### View Logs
```bash
tail -f bot.log          # Live
tail -50 bot.log         # Last 50 lines
grep "error" bot.log     # Errors only
```

### Control Bot
```bash
# Stop
kill 51215

# Start
cd /workspaces/persistent_user/sol/sol
export TELEGRAM_BOT_TOKEN='8256426089:AAFHfyYFVDyxcbdr9wKKhhAcEGf-CEgiGCk'
nohup ./bin/telegram-bot > bot.log 2>&1 &

# Restart
kill 51215 && sleep 2 && \
export TELEGRAM_BOT_TOKEN='8256426089:AAFHfyYFVDyxcbdr9wKKhhAcEGf-CEgiGCk' && \
nohup ./bin/telegram-bot > bot.log 2>&1 &
```

### Database
```bash
# Count wallets
sqlite3 bot.db "SELECT COUNT(*) FROM wallets;"

# View recent
sqlite3 bot.db "SELECT wallet, winrate, realized_pnl FROM wallets ORDER BY scanned_at DESC LIMIT 10;"

# Database size
du -h bot.db
```

---

## 💬 Telegram Usage

1. Search: **@Afnexbot**
2. Send: `/start`
3. Search wallets: `50 100` (≥50% WR, ≥100% PnL)
4. Check status: `/status`
5. View balance: `/balance`

---

## ⚠️ Known Issues

1. **Returns 0% values** - Bug in analyzer (not fixed)
2. **Resource leaks** - May crash after days
3. **Can't rebuild** - Duplicate declarations in storage/

See `BOT_STATUS_FINAL.md` for details.

---

## 📁 Key Files

| File | Path |
|------|------|
| Binary | `bin/telegram-bot` |
| Logs | `bot.log` |
| Database | `bot.db` |
| Config | `config/config.json` |

---

## 🔧 Troubleshooting

**Bot not responding?**
```bash
ps aux | grep telegram-bot
tail -20 bot.log
```

**Playwright errors?**
```bash
ls ~/.cache/ms-playwright*/
```

**No wallets?**
```bash
sqlite3 bot.db "SELECT COUNT(*) FROM wallets;"
grep "Worker" bot.log | tail -20
```

---

## 📊 Expected Behavior

- Fetches 50 tokens from Moralis
- Collects ~3,500-4,000 wallet addresses
- Analyzes with 6 concurrent browsers
- Takes 25-45 minutes per cycle
- Repeats every 30 minutes
- Stores wallets for 5 hours

---

## 🎯 Current Config

- Min WR: 25%
- Min PnL: 25%
- Token Source: Moralis
- Workers: 6
- Retention: 5 hours

---

**Last Updated:** Nov 24, 2025
**Status:** ✅ Operational with known bugs