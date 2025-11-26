# 🚀 VPS Deployment Guide

## ✅ Portability Status

**YES**, the bot will work on any Linux VPS! The code is fully portable and requires minimal configuration.

---

## 📋 Prerequisites

### Required on VPS:
- **Go 1.21+** 
- **Git**
- **SQLite3** (usually pre-installed)
- **Screen** or **tmux** (recommended for keeping bot running)

### Optional:
- **Playwright** (only if using Dev Finder feature)

---

## 🔧 Setup Steps

### 1. Install Go (if not installed)
```bash
# Download and install Go 1.21+
wget https://go.dev/dl/go1.21.6.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.6.linux-amd64.tar.gz

# Add to PATH
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
echo 'export GOPATH=$HOME/go' >> ~/.bashrc
source ~/.bashrc

# Verify
go version
```

### 2. Clone/Upload Your Project
```bash
# Option A: Via Git
git clone <your-repo-url>
cd sol/sol

# Option B: Via SCP
scp -r ./sol your-vps-user@your-vps-ip:/home/user/
```

### 3. **IMPORTANT**: Update Configuration

#### A. Update Telegram Bot Token in `run.sh`
**Current** (line 20):
```bash
export TELEGRAM_BOT_TOKEN='8256426089:AAFHfyYFVDyxcbdr9wKKhhAcEGf-CEgiGCk'
```

**Better** (use environment variable):
```bash
# Add to run.sh line 20:
export TELEGRAM_BOT_TOKEN="${TELEGRAM_BOT_TOKEN:-}"

# Then set in your VPS shell:
echo 'export TELEGRAM_BOT_TOKEN="your-bot-token-here"' >> ~/.bashrc
source ~/.bashrc
```

#### B. Update `config/config.json`
```bash
nano config/config.json
```
Update API keys for your VPS environment.

### 4. Install Dependencies
```bash
cd sol/sol
go mod download
```

### 5. Build the Bot
```bash
chmod +x run.sh
./run.sh build
```

### 6. Run the Bot

#### Option A: Foreground (testing)
```bash
./run.sh start
./run.sh logs  # Watch logs
```

#### Option B: Background (production)
```bash
# Using screen
screen -S telegram-bot
./run.sh start
# Press Ctrl+A, then D to detach

# Reattach later
screen -r telegram-bot

# OR using systemd (recommended for production)
# See "Systemd Service" section below
```

---

## 🛡️ Production Setup (Systemd)

### Create Service File

Create `/etc/systemd/system/solana-bot.service`:

```ini
[Unit]
Description=Solana Trading Telegram Bot
After=network.target

[Service]
Type=simple
User=your-username
WorkingDirectory=/home/your-username/sol/sol
Environment="TELEGRAM_BOT_TOKEN=your-bot-token-here"
Environment="GOPATH=/home/your-username/go"
ExecStart=/home/your-username/sol/sol/bin/telegram-bot
Restart=always
RestartSec=10
StandardOutput=append:/home/your-username/sol/sol/bot.log
StandardError=append:/home/your-username/sol/sol/bot.log

[Install]
WantedBy=multi-user.target
```

### Enable and Start
```bash
sudo systemctl daemon-reload
sudo systemctl enable solana-bot
sudo systemctl start solana-bot

# Check status
sudo systemctl status solana-bot

# View logs
tail -f ~/sol/sol/bot.log
```

---

## 🔍 Potential Issues & Solutions

### 1. **Port Issues**
- Bot uses outbound HTTPS (443) - usually no issues
- No inbound ports needed (Telegram polling)

### 2. **Firewall**
```bash
# Usually not needed, but if issues:
sudo ufw allow out 443/tcp
```

### 3. **File Permissions**
```bash
chmod +x run.sh
chmod 755 bin/
```

### 4. **Database**
- SQLite `bot.db` created automatically
- Stored in working directory: `/home/user/sol/sol/bot.db`
- Persists across restarts ✅

### 5. **Memory**
- Minimum: **512MB RAM**
- Recommended: **1GB+ RAM**
- Bot is lightweight (~50MB memory usage)

---

## 📦 What's Portable (No Changes Needed)

✅ **Go binaries** - Statically compiled  
✅ **SQLite database** - File-based, portable  
✅ **Configuration** - JSON files  
✅ **Scripts** - Pure bash  
✅ **Dependencies** - Go modules handle everything  

---

## ⚠️ What Needs Updating

🔧 **Telegram Bot Token** (line 20 in `run.sh`)  
🔧 **API Keys** (in `config/config.json`)  
🔧 **GOPATH** (automatically set to `$HOME/go`)  

---

## 🧪 Quick Test Deployment

```bash
# SSH into your VPS
ssh user@your-vps-ip

# Quick test script
cd sol/sol
export TELEGRAM_BOT_TOKEN="your-token"
./run.sh rebuild
./run.sh status

# Test Telegram bot
# Send /start to your bot
```

---

## 🔄 Updating on VPS

```bash
# Stop bot
./run.sh stop

# Update code
git pull  # or re-upload files

# Rebuild and restart
./run.sh rebuild

# Check logs
./run.sh logs
```

---

## 📊 Monitoring

### Check Bot Status
```bash
./run.sh status
```

### View Logs
```bash
./run.sh logs          # Live logs
tail -100 bot.log      # Last 100 lines
```

### Check Resources
```bash
top -p $(cat bot.pid)  # CPU/Memory usage
```

---

## 💡 Pro Tips

1. **Use Environment Variables** for sensitive data
2. **Setup Systemd** for auto-restart on crashes
3. **Monitor Logs** regularly
4. **Backup `bot.db`** periodically
5. **Use `screen` or `tmux`** for easy management

---

## ✅ Compatibility

- ✅ **Ubuntu 20.04+**
- ✅ **Debian 10+**
- ✅ **CentOS 7+**
- ✅ **Any Linux with Go 1.21+**
- ⚠️ **Windows** - Requires WSL or minor script changes
- ❌ **MacOS** - Works but `run.sh` needs minor tweaks

---

## 🆘 Troubleshooting

### Bot Won't Start
```bash
# Check Go installation
go version

# Check dependencies
cd sol/sol && go mod download

# Check permissions
ls -la bin/telegram-bot

# Check logs
cat bot.log
```

### Bot Crashes
```bash
# Enable systemd auto-restart
sudo systemctl enable solana-bot

# Or use run.sh in a loop
while true; do ./run.sh start; sleep 5; done
```

---

**Need help?** Check `bot.log` for detailed error messages!
