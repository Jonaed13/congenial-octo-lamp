#!/bin/bash
# Quick start with optimal settings for your VPS

echo "🚀 Starting Solana Orchestrator with optimal settings..."
echo ""
echo "⚙️  Settings:"
echo "   - 10 concurrent pages (fast!)"
echo "   - Birdeye token source"
echo "   - 50 tokens"
echo "   - Min winrate: 70%"
echo "   - Min PnL: 100%"
echo ""

cd /home/jon/hi/solana_orchestrator

python3 run.py \
  --non-interactive \
  --token-source birdeye \
  --limit 50 \
  --pages 10 \
  --min-winrate 70 \
  --min-pnl 100

echo ""
echo "✅ Done! Check data/good_wallets.json for results"
