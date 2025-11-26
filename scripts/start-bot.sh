#!/bin/bash
cd /home/user/sol
export GOPATH=$HOME/go
export TELEGRAM_BOT_TOKEN='8256426089:AAFHfyYFVDyxcbdr9wKKhhAcEGf-CEgiGCk'

echo "🔨 Building..."
go build -o telegram-bot telegram-bot.go

echo "🚀 Starting bot..."
./telegram-bot
