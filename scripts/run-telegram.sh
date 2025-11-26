#!/bin/bash
set -e

export GOPATH=$HOME/go

if [ -z "$TELEGRAM_BOT_TOKEN" ]; then
    echo "❌ TELEGRAM_BOT_TOKEN not set"
    echo ""
    echo "Set token and run:"
    echo "export TELEGRAM_BOT_TOKEN='your_token'"
    echo "./run-telegram.sh"
    exit 1
fi

echo "🔨 Building bot..."
go build -o telegram-bot telegram-bot.go

echo "🚀 Starting Telegram Bot..."
echo "Bot Token: ${TELEGRAM_BOT_TOKEN:0:10}..."
echo ""

./telegram-bot
