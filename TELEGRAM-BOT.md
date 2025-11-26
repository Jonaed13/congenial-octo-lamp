# Solana Wallet Scanner - Telegram Bot

24/7 continuous wallet scanner with Telegram interface for thousands of users.

## Features

- 🔄 Continuous scanning (runs 24/7)
- 👥 Supports thousands of concurrent users
- 🎯 Custom filters per user (winrate & PnL)
- ⚡ Instant results from database
- 📊 6 concurrent browser pages for fast scanning
- 💾 Persistent wallet database

## Setup

1. **Get Telegram Bot Token**
   - Message @BotFather on Telegram
   - Send `/newbot`
   - Follow instructions to create bot
   - Copy the token

2. **Build and Run**
   ```bash
   ./setup-telegram.sh
   export TELEGRAM_BOT_TOKEN='your_token_here'
   ./run-telegram.sh
   ```

## Usage

### For Users

1. Start bot: `/start`
2. Send filters: `winrate pnl`
   - Example: `50 100` (50% winrate, 100% PnL)
3. Get results instantly or wait message
4. Check status: `/status`

### Examples

```
50 100    → Find wallets with ≥50% WR and ≥100% PnL
25 25     → Find wallets with ≥25% WR and ≥25% PnL
70 200    → Find wallets with ≥70% WR and ≥200% PnL
```

## How It Works

1. **Scanner Loop** (background)
   - Fetches 100 tokens from Birdeye
   - Gets top holders for each token
   - Analyzes all wallets with 6 concurrent pages
   - Stores results in memory database
   - Repeats every 30 minutes

2. **User Requests** (instant)
   - User sends filters
   - Bot searches database
   - Returns matching wallets immediately
   - If no matches, shows scan progress

## Performance

- **Scan Speed**: ~1000 wallets per 30 minutes
- **User Response**: Instant (database lookup)
- **Concurrent Users**: Unlimited (async handling)
- **Memory**: ~500MB for 10k wallets

## Running 24/7

### Using systemd (Linux)

Create `/etc/systemd/system/solana-bot.service`:

```ini
[Unit]
Description=Solana Wallet Scanner Bot
After=network.target

[Service]
Type=simple
User=youruser
WorkingDirectory=/home/user/sol
Environment="TELEGRAM_BOT_TOKEN=your_token"
Environment="GOPATH=/home/user/go"
ExecStart=/home/user/sol/telegram-bot
Restart=always

[Install]
WantedBy=multi-user.target
```

Then:
```bash
sudo systemctl enable solana-bot
sudo systemctl start solana-bot
sudo systemctl status solana-bot
```

### Using screen (simple)

```bash
screen -S solana-bot
export TELEGRAM_BOT_TOKEN='your_token'
./run-telegram.sh
# Press Ctrl+A then D to detach
```

## Monitoring

Check logs:
```bash
# If using systemd
sudo journalctl -u solana-bot -f

# If using screen
screen -r solana-bot
```

## Configuration

Edit `config/config.json`:
- `token_limit`: Number of tokens to fetch (default: 100)
- `max_retries`: API retry attempts (default: 3)
- API keys are already configured

## Scaling

For high load:
1. Increase scan frequency (reduce sleep time)
2. Add more concurrent pages (change 6 to 10+)
3. Use Redis for distributed database
4. Run multiple scanner instances
