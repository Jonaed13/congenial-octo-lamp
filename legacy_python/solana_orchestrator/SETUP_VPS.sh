#!/bin/bash
#
# 🚀 SOLANA ORCHESTRATOR - VPS SETUP FROM SCRATCH
# ================================================
# This script sets up everything you need on a fresh Ubuntu VPS
#

set -e  # Exit on any error

echo "🚀 Solana Orchestrator - VPS Setup"
echo "====================================="
echo ""

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Step 1: Update system
echo -e "${BLUE}📦 Step 1: Updating system packages...${NC}"
sudo apt-get update
sudo apt-get upgrade -y
echo -e "${GREEN}✅ System updated${NC}"
echo ""

# Step 2: Install Python 3.10+ if not present
echo -e "${BLUE}🐍 Step 2: Checking Python installation...${NC}"
if ! command -v python3 &> /dev/null; then
    echo "Installing Python 3..."
    sudo apt-get install -y python3 python3-pip python3-venv
else
    PYTHON_VERSION=$(python3 --version)
    echo "✅ Python already installed: $PYTHON_VERSION"
fi
echo ""

# Step 3: Install system dependencies for Playwright/Chromium
echo -e "${BLUE}📚 Step 3: Installing system dependencies for Chromium...${NC}"
sudo apt-get install -y \
    libnss3 \
    libnspr4 \
    libatk1.0-0 \
    libatk-bridge2.0-0 \
    libcups2 \
    libdrm2 \
    libxkbcommon0 \
    libxcomposite1 \
    libxdamage1 \
    libxfixes3 \
    libxrandr2 \
    libgbm1 \
    libpango-1.0-0 \
    libcairo2 \
    libasound2 \
    libatspi2.0-0 \
    wget \
    ca-certificates \
    fonts-liberation \
    libappindicator3-1 \
    libu2f-udev \
    xdg-utils

echo -e "${GREEN}✅ System dependencies installed${NC}"
echo ""

# Step 4: Install Python packages
echo -e "${BLUE}📦 Step 4: Installing Python packages...${NC}"
pip3 install --upgrade pip
pip3 install playwright requests asyncio
echo -e "${GREEN}✅ Python packages installed${NC}"
echo ""

# Step 5: Install Playwright browsers (Chromium)
echo -e "${BLUE}🌐 Step 5: Installing Playwright Chromium browser...${NC}"
echo "This may take a few minutes..."
python3 -m playwright install chromium
python3 -m playwright install-deps chromium
echo -e "${GREEN}✅ Playwright Chromium installed${NC}"
echo ""

# Step 6: Verify installation
echo -e "${BLUE}🔍 Step 6: Verifying installation...${NC}"
echo "Python version:"
python3 --version
echo ""
echo "Pip version:"
pip3 --version
echo ""
echo "Playwright version:"
python3 -c "from playwright.sync_api import sync_playwright; print('Playwright installed successfully')"
echo ""

# Step 7: Check if orchestrator files exist
echo -e "${BLUE}📁 Step 7: Checking orchestrator files...${NC}"
if [ -f "run.py" ]; then
    echo -e "${GREEN}✅ Orchestrator files found${NC}"
else
    echo -e "${RED}❌ Orchestrator files not found in current directory${NC}"
    echo "Please make sure you're in the solana_orchestrator directory"
    exit 1
fi
echo ""

# Step 8: Create necessary directories
echo -e "${BLUE}📂 Step 8: Creating necessary directories...${NC}"
mkdir -p data logs config
echo -e "${GREEN}✅ Directories created${NC}"
echo ""

# Step 9: Check config
echo -e "${BLUE}⚙️  Step 9: Checking configuration...${NC}"
if [ -f "config/config.json" ]; then
    echo -e "${GREEN}✅ Config file found${NC}"
else
    echo -e "${RED}⚠️  Config file not found${NC}"
    echo "Creating sample config..."
    cat > config/config.json << 'EOF'
{
  "moralis_api_key": "YOUR_MORALIS_API_KEY_HERE",
  "birdeye_api_key": "YOUR_BIRDEYE_API_KEY_HERE",
  "api_settings": {
    "max_retries": 3,
    "token_limit": 100
  },
  "analysis_filters": {
    "min_winrate": 70.0,
    "min_realized_pnl": 100.0
  }
}
EOF
    echo -e "${BLUE}📝 Please edit config/config.json with your API keys${NC}"
fi
echo ""

# Final summary
echo ""
echo -e "${GREEN}═══════════════════════════════════════════════════${NC}"
echo -e "${GREEN}✅ SETUP COMPLETE!${NC}"
echo -e "${GREEN}═══════════════════════════════════════════════════${NC}"
echo ""
echo "📝 Next steps:"
echo ""
echo "1. Edit your config file:"
echo "   nano config/config.json"
echo ""
echo "2. Add your API keys:"
echo "   - Moralis API key"
echo "   - Birdeye API key (optional)"
echo ""
echo "3. Run the orchestrator:"
echo "   python3 run.py"
echo ""
echo "4. Or run with specific settings:"
echo "   python3 run.py --pages 10 --min-winrate 70 --min-pnl 100"
echo ""
echo -e "${BLUE}📚 For more info, see README.md${NC}"
echo ""
