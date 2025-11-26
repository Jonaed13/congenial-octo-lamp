# 🚀 Enhanced run.sh Script - Usage Guide

**Version:** 2.0  
**Date:** November 24, 2025  
**Status:** Fully Functional

---

## 📋 Overview

The enhanced `run.sh` script is a complete bot management tool with build, start, stop, status, and logging capabilities.

---

## 🎯 Quick Start

```bash
# Build and start the bot
./run.sh rebuild

# Check if it's running
./run.sh status

# Watch live logs
./run.sh logs
```

---

## 📚 All Commands

### Basic Commands

#### `./run.sh build`
**Build the bot binary**

- Compiles from `cmd/bot/` directory
- Backs up old binary to `.backup`
- Shows build success/failure
- Displays binary size

**Example:**
```bash
$ ./run.sh build
╔═══════════════════════════════════════════╗
║     🤖 Telegram Bot Manager              ║
╔═══════════════════════════════════════════╗

🔨 Building telegram bot...
ℹ️  Old binary backed up to ./bin/telegram-bot.backup
✅ Build successful!
ℹ️  Binary size: 18M
```

---

#### `./run.sh start`
**Start the bot**

- Checks if already running
- Starts bot in background
- Saves PID to `bot.pid`
- Redirects output to `bot.log`
- Verifies startup success

**Example:**
```bash
$ ./run.sh start
╔═══════════════════════════════════════════╗
║     🤖 Telegram Bot Manager              ║
╔═══════════════════════════════════════════╗

🚀 Starting bot...
✅ Bot started successfully (PID: 12345)
ℹ️  Logs: tail -f bot.log
```

**What It Does:**
1. Checks if bot is already running
2. Verifies binary exists
3. Starts bot with `nohup`
4. Saves PID for management
5. Waits 2 seconds to verify
6. Confirms successful startup

---

#### `./run.sh stop`
**Stop the bot gracefully**

- Sends SIGTERM for graceful shutdown
- Waits 5 seconds for clean exit
- Force kills if necessary (SIGKILL)
- Removes PID file

**Example:**
```bash
$ ./run.sh stop
╔═══════════════════════════════════════════╗
║     🤖 Telegram Bot Manager              ║
╔═══════════════════════════════════════════╗

🛑 Stopping bot...
✅ Bot stopped (PID: 12345)
```

---

#### `./run.sh restart`
**Restart the bot**

- Stops the bot
- Waits 1 second
- Starts the bot

**Example:**
```bash
$ ./run.sh restart
╔═══════════════════════════════════════════╗
║     🤖 Telegram Bot Manager              ║
╔═══════════════════════════════════════════╗

🛑 Stopping bot...
✅ Bot stopped (PID: 12345)

🚀 Starting bot...
✅ Bot started successfully (PID: 12346)
ℹ️  Logs: tail -f bot.log
```

---

#### `./run.sh status`
**Show detailed bot status**

Shows:
- Running status (YES/NO)
- Process ID (PID)
- CPU usage (%)
- Memory usage (%)
- Runtime duration
- Database size
- Wallet count
- Last 5 log lines

**Example:**
```bash
$ ./run.sh status
╔═══════════════════════════════════════════╗
║     🤖 Telegram Bot Manager              ║
╔═══════════════════════════════════════════╗

📊 Bot Status
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
✅ Status: RUNNING
PID: 12345
CPU: 0.3%
Memory: 0.2%
Runtime: 01:23:45
Database: 2.5M
Wallets: 5448

📝 Recent Logs (last 5 lines):
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  2025/11/24 15:03:36 ✅ Worker 0: Abc12345 - WR: 65.50%, PnL: 245.80%
  2025/11/24 15:03:37 ✅ Worker 1: Def67890 - WR: 58.20%, PnL: 178.40%
  2025/11/24 15:03:38 ✅ Worker 2: Ghi11223 - WR: 72.10%, PnL: 312.50%
  2025/11/24 15:03:39 ✅ Worker 3: Jkl44556 - WR: 45.30%, PnL: 125.60%
  2025/11/24 15:03:40 ✅ Worker 4: Mno77889 - WR: 55.80%, PnL: 189.20%
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

---

#### `./run.sh logs`
**Show live logs (real-time)**

- Displays log file with `tail -f`
- Updates in real-time
- Press `Ctrl+C` to exit

**Example:**
```bash
$ ./run.sh logs
📋 Live Logs (Ctrl+C to exit)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
2025/11/24 15:03:36 ✅ Worker 0: Abc12345 - WR: 65.50%, PnL: 245.80%
2025/11/24 15:03:37 ✅ Worker 1: Def67890 - WR: 58.20%, PnL: 178.40%
2025/11/24 15:03:38 ✅ Worker 2: Ghi11223 - WR: 72.10%, PnL: 312.50%
...
^C (Press Ctrl+C to exit)
```

---

#### `./run.sh clean`
**Clean old log files**

- Backs up current logs with timestamp
- Removes empty log files
- Saves backup as `bot.log.YYYYMMDD_HHMMSS.bak`

**Example:**
```bash
$ ./run.sh clean
╔═══════════════════════════════════════════╗
║     🤖 Telegram Bot Manager              ║
╔═══════════════════════════════════════════╗

🧹 Cleaning logs...
✅ Old logs backed up to bot.log.20251124_150345.bak
```

---

#### `./run.sh rebuild`
**Full rebuild and restart**

Does everything in order:
1. Stops bot
2. Builds new binary
3. Starts bot

Perfect for deploying updates!

**Example:**
```bash
$ ./run.sh rebuild
╔═══════════════════════════════════════════╗
║     🤖 Telegram Bot Manager              ║
╔═══════════════════════════════════════════╗

🛑 Stopping bot...
✅ Bot stopped (PID: 12345)

🔨 Building telegram bot...
ℹ️  Old binary backed up to ./bin/telegram-bot.backup
✅ Build successful!
ℹ️  Binary size: 18M

🚀 Starting bot...
✅ Bot started successfully (PID: 12346)
ℹ️  Logs: tail -f bot.log
```

---

#### `./run.sh help`
**Show help message**

Displays all available commands and examples.

---

## 🎨 Color Output

The script uses colors for better readability:

- 🔴 **Red** - Errors
- 🟢 **Green** - Success messages
- 🟡 **Yellow** - Warnings
- 🔵 **Blue** - Info messages
- 🔷 **Cyan** - Headers

---

## 🔧 Technical Details

### Files Created/Used:

| File | Purpose |
|------|---------|
| `bot.pid` | Stores current bot process ID |
| `bot.log` | Main log file (stdout + stderr) |
| `bot.log.*.bak` | Backup log files |
| `bin/telegram-bot` | Compiled bot binary |
| `bin/telegram-bot.backup` | Previous binary backup |

### Environment Variables:

```bash
GOPATH=$HOME/go                                      # Go workspace
TELEGRAM_BOT_TOKEN='8256426089:AAFHfyYFV...'        # Bot token
```

### Process Management:

- **Start:** Uses `nohup` for background execution
- **Stop:** Graceful SIGTERM, then SIGKILL if needed
- **PID Tracking:** Stored in `bot.pid` file
- **Status Check:** Uses `ps` command

---

## 📊 Enhanced Logging

### New Log Format:

**Before:**
```
✅ Worker 0: Analyzed Abc123XyZ789def456...
```

**After (with WR/PnL):**
```
✅ Worker 0: Abc12345 - WR: 65.50%, PnL: 245.80%
```

### Log Information:

Each worker log line shows:
- ✅ Success indicator
- Worker ID (0-5)
- First 8 characters of wallet address
- Win Rate percentage
- Realized PnL percentage

**Example:**
```
2025/11/24 15:03:36 ✅ Worker 0: Abc12345 - WR: 65.50%, PnL: 245.80%
2025/11/24 15:03:37 ✅ Worker 1: Def67890 - WR: 58.20%, PnL: 178.40%
2025/11/24 15:03:38 ✅ Worker 2: Ghi11223 - WR: 72.10%, PnL: 312.50%
2025/11/24 15:03:39 ✅ Worker 3: Jkl44556 - WR: 45.30%, PnL: 125.60%
2025/11/24 15:03:40 ✅ Worker 4: Mno77889 - WR: 55.80%, PnL: 189.20%
2025/11/24 15:03:41 ✅ Worker 5: Pqr99001 - WR: 68.90%, PnL: 223.40%
```

---

## 💡 Common Use Cases

### Deploy Code Changes
```bash
# 1. Make your code changes
# 2. Rebuild and restart
./run.sh rebuild
```

### Check if Bot is Alive
```bash
./run.sh status
```

### Monitor Activity
```bash
./run.sh logs
```

### Troubleshoot Issues
```bash
# Check status
./run.sh status

# View recent errors
tail -50 bot.log | grep -i error

# View full logs
less bot.log
```

### Clean Disk Space
```bash
# Clean old logs
./run.sh clean

# Remove old backups
rm -f bot.log.*.bak
rm -f bin/telegram-bot.backup
```

---

## 🐛 Error Handling

### Build Fails
**Problem:** Compilation errors

**What Script Does:**
- Shows error message
- Restores previous binary
- Exits with error code

**Action:**
1. Check error output
2. Fix compilation issues
3. Try building again

### Bot Won't Start
**Problem:** Binary doesn't exist or crashes

**What Script Does:**
- Shows error message
- Displays last 20 log lines
- Exits with error code

**Action:**
1. Check logs for crash reason
2. Verify binary exists: `ls -lh bin/telegram-bot`
3. Check permissions: `chmod +x bin/telegram-bot`
4. Try rebuilding: `./run.sh rebuild`

### Bot Already Running
**Problem:** Trying to start when already running

**What Script Does:**
- Shows warning with PID
- Doesn't start duplicate

**Action:**
- Use `./run.sh restart` to restart
- Or `./run.sh stop` then `./run.sh start`

---

## 🔍 Status Checks

### Quick Status
```bash
./run.sh status
```

### Detailed Process Info
```bash
ps aux | grep telegram-bot | grep -v grep
```

### Memory Usage
```bash
ps -p $(cat bot.pid) -o %cpu,%mem,cmd
```

### Database Stats
```bash
sqlite3 bot.db "SELECT COUNT(*) FROM wallets;"
```

### Log Size
```bash
du -h bot.log
```

---

## 📝 Log Management

### View Last N Lines
```bash
tail -50 bot.log        # Last 50 lines
tail -100 bot.log       # Last 100 lines
```

### Search Logs
```bash
grep "Worker" bot.log                    # All worker logs
grep "WR:" bot.log                       # All with win rates
grep "Error" bot.log                     # All errors
grep "2025/11/24 15:" bot.log           # Specific time
```

### Filter by Win Rate
```bash
# Find wallets with WR > 70%
grep "WR: [7-9][0-9]\." bot.log

# Find wallets with PnL > 200%
grep "PnL: [2-9][0-9][0-9]\." bot.log
```

### Rotate Logs Manually
```bash
# Create backup
cp bot.log bot.log.$(date +%Y%m%d).bak

# Clear current log
> bot.log

# Restart to create new log
./run.sh restart
```

---

## ⚙️ Configuration

### Change Bot Token
Edit `run.sh` line 19:
```bash
export TELEGRAM_BOT_TOKEN='YOUR_NEW_TOKEN_HERE'
```

### Change Log File Name
Edit `run.sh` line 21:
```bash
LOG_FILE="custom_name.log"
```

### Change PID File Name
Edit `run.sh` line 22:
```bash
PID_FILE="custom.pid"
```

---

## 🚨 Safety Features

### Automatic Backup
- Old binary backed up before rebuild
- Can rollback if new build fails
- Backup file: `bin/telegram-bot.backup`

### Graceful Shutdown
- Sends SIGTERM first (allows cleanup)
- Waits 5 seconds
- Force kill only if necessary

### PID Tracking
- Accurate process management
- Prevents duplicate starts
- Clean process tracking

### Error Recovery
- Build failure restores backup
- Detailed error messages
- Safe exit codes

---

## 📊 Script Functions

### Internal Functions:

| Function | Purpose |
|----------|---------|
| `print_header()` | Display banner |
| `print_success()` | Green success message |
| `print_error()` | Red error message |
| `print_warning()` | Yellow warning |
| `print_info()` | Blue info message |
| `is_running()` | Check if bot is active |
| `get_pid()` | Get bot process ID |
| `stop_bot()` | Stop bot process |
| `build_bot()` | Compile binary |
| `start_bot()` | Launch bot |
| `restart_bot()` | Stop and start |
| `show_status()` | Display status |
| `show_logs()` | Tail logs |
| `clean_logs()` | Backup and clean |
| `show_help()` | Display help |

---

## 🎯 Best Practices

### Regular Operations
```bash
# Morning check
./run.sh status

# Monitor activity
./run.sh logs

# Weekly cleanup
./run.sh clean
```

### Deployment Workflow
```bash
# 1. Test changes locally
# 2. Commit changes
# 3. Deploy
./run.sh rebuild

# 4. Verify
./run.sh status

# 5. Monitor
./run.sh logs
```

### Troubleshooting Workflow
```bash
# 1. Check status
./run.sh status

# 2. View logs
./run.sh logs

# 3. If stuck, restart
./run.sh restart

# 4. If broken, rebuild
./run.sh rebuild
```

---

## 📌 Quick Reference

```bash
# Essential Commands
./run.sh start          # Start bot
./run.sh stop           # Stop bot
./run.sh restart        # Restart bot
./run.sh status         # Show status
./run.sh logs           # Watch logs

# Build Commands
./run.sh build          # Build only
./run.sh rebuild        # Build + restart

# Maintenance
./run.sh clean          # Clean logs
./run.sh help           # Show help
```

---

## ✅ Summary

The enhanced `run.sh` script provides:

- ✅ Easy bot management
- ✅ Color-coded output
- ✅ Detailed status information
- ✅ Graceful shutdown
- ✅ Automatic backups
- ✅ Error recovery
- ✅ Log management
- ✅ PID tracking
- ✅ Enhanced logging with WR/PnL

**One command to rule them all:**
```bash
./run.sh rebuild
```

---

**Version:** 2.0  
**Last Updated:** November 24, 2025  
**Status:** Production Ready  
**Author:** AI Assistant