#!/bin/bash
set -e

echo "🚀 Solana Orchestrator Setup"
echo "=============================="

# Set GOPATH
export GOPATH=$HOME/go
mkdir -p $GOPATH

# Copy API keys from Python config if not already configured
if [ ! -f "config/config.json" ] || grep -q "YOUR_" config/config.json; then
    echo "📝 Configuring API keys..."
    if [ -f "solana_orchestrator/config/config.json" ]; then
        MORALIS_KEY=$(grep -o '"moralis_api_key": "[^"]*"' solana_orchestrator/config/config.json | cut -d'"' -f4)
        BIRDEYE_KEY=$(grep -o '"birdeye_api_key": "[^"]*"' solana_orchestrator/config/config.json | cut -d'"' -f4)
        
        cat > config/config.json <<EOF
{
  "moralis_api_key": "$MORALIS_KEY",
  "birdeye_api_key": "$BIRDEYE_KEY",
  "analysis_filters": {
    "min_winrate": 25,
    "min_realized_pnl": 25
  },
  "api_settings": {
    "max_retries": 3,
    "token_limit": 100
  }
}
EOF
        echo "✅ API keys configured"
    fi
fi

# Install dependencies
echo "📦 Installing Go dependencies..."
go mod tidy

# Install Playwright
if [ ! -d "$HOME/.cache/ms-playwright/chromium-1105" ]; then
    echo "🎭 Installing Playwright browsers..."
    go run github.com/playwright-community/playwright-go/cmd/playwright@v0.4201.1 install chromium
fi

# Build
echo "🔨 Building orchestrator..."
go build -o orchestrator main.go

echo "✅ Setup complete!"
echo ""
echo "Run with: ./orchestrator -limit 10 -pages 2"
