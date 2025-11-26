@echo off
echo ================================================================================
echo 🚀 SOLANA ORCHESTRATOR - Windows Runner 🚀
echo ================================================================================
echo.

REM Check if Python is available
python --version >nul 2>&1
if errorlevel 1 (
    echo ❌ Python is not installed or not in PATH!
    echo Please run setup_windows.bat first
    pause
    exit /b 1
)

REM Check if config file exists
if not exist "config\config.json" (
    echo ❌ Configuration file not found!
    echo Please create config\config.json with your API keys
    echo See WINDOWS_SETUP.md for details
    pause
    exit /b 1
)

echo ✅ Starting Solana Orchestrator...
echo.
echo Choose running mode:
echo 1. Interactive mode (recommended)
echo 2. Quick test (5 tokens, 2 pages)
echo 3. Standard run (50 tokens, 3 pages)
echo 4. Auto-loop mode (runs continuously)
echo 5. Custom command
echo.

set /p choice="Enter your choice (1-5): "

if "%choice%"=="1" (
    echo Running in interactive mode...
    python run.py
) else if "%choice%"=="2" (
    echo Running quick test...
    python run.py --non-interactive --limit 5 --pages 2 --min-winrate 70 --min-pnl 100 --token-source birdeye
) else if "%choice%"=="3" (
    echo Running standard configuration...
    python run.py --non-interactive --limit 50 --pages 3 --min-winrate 70 --min-pnl 100 --token-source birdeye
) else if "%choice%"=="4" (
    echo Running auto-loop mode (every hour)...
    python run.py --non-interactive --loop 60 --limit 30 --pages 3 --min-winrate 75 --min-pnl 150 --token-source birdeye
) else if "%choice%"=="5" (
    echo.
    echo Available options:
    echo   --limit N          : Number of tokens to fetch
    echo   --pages N          : Number of concurrent browser pages
    echo   --min-winrate N    : Minimum win rate percentage
    echo   --min-pnl N        : Minimum realized PnL percentage
    echo   --token-source     : birdeye or moralis
    echo   --loop N           : Auto-loop every N minutes
    echo   --clean            : Delete all data files first
    echo.
    set /p custom_cmd="Enter custom command (or press Enter for help): "
    if not "%custom_cmd%"=="" (
        python run.py %custom_cmd%
    ) else (
        python run.py --help
    )
) else (
    echo Invalid choice. Running interactive mode...
    python run.py
)

echo.
echo ================================================================================
echo 🎉 Execution completed!
echo ================================================================================
echo.
echo Check the following folders for results:
echo - data\     : Token and holder data, good wallets found
echo - logs\     : Detailed execution logs  
echo - results\  : Analysis results
echo.
pause