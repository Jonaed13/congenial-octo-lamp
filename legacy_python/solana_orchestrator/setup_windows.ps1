# PowerShell Setup Script for Solana Orchestrator
Write-Host "================================================================================" -ForegroundColor Cyan
Write-Host "🚀 SOLANA ORCHESTRATOR - Windows PowerShell Setup 🚀" -ForegroundColor Cyan
Write-Host "================================================================================" -ForegroundColor Cyan
Write-Host ""

# Check if Python is installed
try {
    $pythonVersion = python --version 2>$null
    Write-Host "✅ Python found: $pythonVersion" -ForegroundColor Green
} catch {
    Write-Host "❌ Python is not installed or not in PATH!" -ForegroundColor Red
    Write-Host "Please install Python from https://www.python.org/downloads/windows/" -ForegroundColor Yellow
    Write-Host "Make sure to check 'Add Python to PATH' during installation" -ForegroundColor Yellow
    Read-Host "Press Enter to exit"
    exit 1
}

# Check if pip is available
try {
    $pipVersion = pip --version 2>$null
    Write-Host "✅ pip found: $pipVersion" -ForegroundColor Green
    Write-Host ""
} catch {
    Write-Host "❌ pip is not available!" -ForegroundColor Red
    Read-Host "Press Enter to exit"
    exit 1
}

# Install Python packages
Write-Host "📦 Installing Python packages..." -ForegroundColor Yellow
try {
    pip install -r requirements.txt
    Write-Host "✅ Python packages installed successfully!" -ForegroundColor Green
} catch {
    Write-Host "❌ Failed to install Python packages!" -ForegroundColor Red
    Read-Host "Press Enter to exit"
    exit 1
}
Write-Host ""

# Install Playwright browser
Write-Host "🌐 Installing Playwright browser..." -ForegroundColor Yellow
try {
    playwright install chromium
    Write-Host "✅ Playwright browser installed successfully!" -ForegroundColor Green
} catch {
    Write-Host "❌ Failed to install Playwright browser!" -ForegroundColor Red
    Write-Host "Trying alternative installation..." -ForegroundColor Yellow
    try {
        playwright install chromium --with-deps
        Write-Host "✅ Playwright browser installed with dependencies!" -ForegroundColor Green
    } catch {
        Write-Host "❌ Alternative installation also failed!" -ForegroundColor Red
        Read-Host "Press Enter to exit"
        exit 1
    }
}
Write-Host ""

Write-Host "================================================================================" -ForegroundColor Cyan
Write-Host "✅ Setup Complete!" -ForegroundColor Green
Write-Host "================================================================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "Next steps:" -ForegroundColor Yellow
Write-Host "1. Edit config\config.json with your API keys" -ForegroundColor White
Write-Host "2. Run: python run.py" -ForegroundColor White
Write-Host "   Or use: .\run_windows.bat for guided execution" -ForegroundColor White
Write-Host ""
Write-Host "For detailed help, see WINDOWS_SETUP.md" -ForegroundColor Cyan
Write-Host ""
Read-Host "Press Enter to continue"