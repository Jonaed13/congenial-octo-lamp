#!/bin/bash

# Bot Status Checker
# Usage: ./check-bot-status.sh

echo "🤖 Telegram Bot Status Check"
echo "=============================="
echo ""

# Check if bot process is running
BOT_PID=$(ps aux | grep './bin/telegram-bot' | grep -v grep | awk '{print $2}')

if [ -z "$BOT_PID" ]; then
    echo "❌ Bot is NOT running"
    echo ""
    echo "To start the bot, run:"
    echo "  cd /workspaces/persistent_user/sol/sol"
    echo "  export TELEGRAM_BOT_TOKEN='8256426089:AAFHfyYFVDyxcbdr9wKKhhAcEGf-CEgiGCk'"
    echo "  nohup ./bin/telegram-bot > bot.log 2>&1 &"
    exit 1
fi

echo "✅ Bot is RUNNING"
echo "   PID: $BOT_PID"
echo ""

# Show process info
echo "📊 Process Info:"
ps -p $BOT_PID -o pid,ppid,cmd,%cpu,%mem,etime --no-headers | awk '{printf "   PID: %s\n   CPU: %s%%\n   Memory: %s%%\n   Uptime: %s\n", $1, $4, $5, $6}'
echo ""

# Check log file
if [ -f "bot.log" ]; then
    echo "📝 Recent Logs (last 15 lines):"
    echo "─────────────────────────────────────────"
    tail -15 bot.log | sed 's/^/   /'
    echo "─────────────────────────────────────────"
    echo ""

    # Count log lines
    LOG_LINES=$(wc -l < bot.log)
    echo "   Total log lines: $LOG_LINES"
else
    echo "⚠️  No log file found (bot.log)"
fi
echo ""

# Check database
if [ -f "bot.db" ]; then
    DB_SIZE=$(du -h bot.db | cut -f1)
    echo "💾 Database: bot.db ($DB_SIZE)"

    # Try to count wallets if sqlite3 is available
    if command -v sqlite3 &> /dev/null; then
        WALLET_COUNT=$(sqlite3 bot.db "SELECT COUNT(*) FROM wallets;" 2>/dev/null || echo "N/A")
        if [ "$WALLET_COUNT" != "N/A" ]; then
            echo "   Wallets stored: $WALLET_COUNT"
        fi
    fi
else
    echo "⚠️  No database file found (bot.db)"
fi
echo ""

# Check config
if [ -f "config/config.json" ]; then
    echo "⚙️  Config: config/config.json ✓"

    # Extract key settings
    if command -v jq &> /dev/null; then
        MIN_WR=$(jq -r '.analysis_filters.min_winrate' config/config.json 2>/dev/null)
        MIN_PNL=$(jq -r '.analysis_filters.min_realized_pnl' config/config.json 2>/dev/null)
        TOKEN_SOURCE=$(jq -r '.api_settings.token_source' config/config.json 2>/dev/null)
        echo "   Min Winrate: ${MIN_WR}%"
        echo "   Min PnL: ${MIN_PNL}%"
        echo "   Token Source: $TOKEN_SOURCE"
    fi
else
    echo "❌ Config file not found!"
fi
echo ""

# Check bot token
if [ -z "$TELEGRAM_BOT_TOKEN" ]; then
    echo "⚠️  TELEGRAM_BOT_TOKEN not set in current shell"
    echo "   (Bot was started with token from environment)"
else
    echo "✅ TELEGRAM_BOT_TOKEN is set"
    echo "   Token: ${TELEGRAM_BOT_TOKEN:0:10}..."
fi
echo ""

# Provide useful commands
echo "📌 Useful Commands:"
echo "   View live logs:  tail -f bot.log"
echo "   Stop bot:        kill $BOT_PID"
echo "   Restart bot:     kill $BOT_PID && nohup ./bin/telegram-bot > bot.log 2>&1 &"
echo ""

# Check for known issues
echo "⚠️  Known Issues:"
echo "   1. Analyzer returns 0% winrate/PnL (bug in code)"
echo "   2. Resource leaks in API client (may crash after days)"
echo "   3. See bug report for full details"
echo ""

echo "✨ Bot is operational! Chat with @Afnexbot on Telegram"
