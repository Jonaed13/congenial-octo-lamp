@echo off
echo ================================================================================
echo 🚀 SOLANA ORCHESTRATOR - Windows Setup Script 🚀
echo ================================================================================
echo.

REM Check if Python is installed
python --version >nul 2>&1
if errorlevel 1 (
    echo ❌ Python is not installed or not in PATH!
    echo Please install Python from https://www.python.org/downloads/windows/
    echo Make sure to check "Add Python to PATH" during installation
    pause
    exit /b 1
)

echo ✅ Python found:
python --version
echo.

REM Check if pip is available
pip --version >nul 2>&1
if errorlevel 1 (
    echo ❌ pip is not available!
    pause
    exit /b 1
)

echo ✅ pip found:
pip --version
echo.

echo 📦 Installing Python packages...
pip install -r requirements.txt
if errorlevel 1 (
    echo ❌ Failed to install Python packages!
    pause
    exit /b 1
)
echo.

echo 🌐 Installing Playwright browser...
playwright install chromium
if errorlevel 1 (
    echo ❌ Failed to install Playwright browser!
    pause
    exit /b 1
)
echo.

echo ================================================================================
echo ✅ Setup Complete!
echo ================================================================================
echo.
echo Next steps:
echo 1. Edit config\config.json with your API keys
echo 2. Run: python run.py
echo.
echo For help, see WINDOWS_SETUP.md
echo.
pause