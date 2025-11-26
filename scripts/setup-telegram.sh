#!/bin/bash
set -e

echo "🤖 Telegram Bot Setup"
echo "====================="

export GOPATH=$HOME/go

# Build telegram bot
echo "🔨 Building telegram bot..."
go build -o telegram-bot telegram-bot.go

chmod +x run-telegram.sh

echo ""
echo "✅ Setup complete!"
echo ""
echo "📝 Next steps:"
echo "1. Get bot token from @BotFather on Telegram"
echo "2. Run: export TELEGRAM_BOT_TOKEN='your_token_here'"
echo "3. Run: ./run-telegram.sh"
