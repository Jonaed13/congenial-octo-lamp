#!/bin/bash

# ============================================
# 🤖 Telegram Bot Manager Script
# ============================================
# Enhanced run script with build, status, and management

set -e  # Exit on error

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Configuration
export GOPATH=$HOME/go
mkdir -p "$GOPATH"
export TELEGRAM_BOT_TOKEN='8256426089:AAFHfyYFVDyxcbdr9wKKhhAcEGf-CEgiGCk'
BOT_BINARY="./bin/telegram-bot"
LOG_FILE="bot.log"
PID_FILE="bot.pid"

# Functions
print_header() {
    echo -e "${CYAN}╔═══════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║     🤖 Telegram Bot Manager              ║${NC}"
    echo -e "${CYAN}╔═══════════════════════════════════════════╗${NC}"
    echo ""
}

print_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_help() {
    echo -e "${CYAN}Usage: $0 <command>${NC}"
    echo -e "${CYAN}Commands:${NC}"
    echo -e "  ${GREEN}build${NC}      - Build the bot binary"
    echo -e "  ${GREEN}start${NC}      - Start the bot"
    echo -e "  ${GREEN}monitor${NC}    - Open TUI dashboard"
    echo -e "  ${GREEN}stop${NC}       - Stop the bot"
    echo -e "  ${GREEN}restart${NC}    - Restart the bot"
    echo -e "  ${GREEN}status${NC}     - Check bot status"
    echo -e "  ${GREEN}logs${NC}       - Tail bot logs"
    echo -e "  ${GREEN}clean${NC}      - Remove binaries and logs"
    echo -e "  ${GREEN}help${NC}       - Show this help message"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

print_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

# Check if bot is running
is_running() {
    if [ -f "$PID_FILE" ]; then
        PID=$(cat "$PID_FILE")
        if ps -p "$PID" > /dev/null 2>&1; then
            return 0
        else
            # Stale PID file - clean it up
            rm -f "$PID_FILE"
        fi
    fi
    return 1
}

# Get bot PID
get_pid() {
    if [ -f "$PID_FILE" ]; then
        cat "$PID_FILE"
    else
        ps aux | grep "$BOT_BINARY" | grep -v grep | awk '{print $2}' | head -1
    fi
}

# Stop bot
stop_bot() {
    echo -e "${YELLOW}🛑 Stopping bot...${NC}"

    if is_running; then
        PID=$(get_pid)
        kill "$PID" 2>/dev/null

        # Wait for graceful shutdown
        for i in {1..5}; do
            if ! is_running; then
                print_success "Bot stopped (PID: $PID)"
                rm -f "$PID_FILE"
                return 0
            fi
            sleep 1
        done

        # Force kill if still running
        kill -9 "$PID" 2>/dev/null
        print_warning "Bot force killed (PID: $PID)"
        rm -f "$PID_FILE"
    else
        print_info "Bot is not running"
    fi
}

# Check dependencies
check_dependencies() {
    echo -e "${CYAN}🔍 Checking dependencies...${NC}"

    # Determine if we need sudo
    if [ "$EUID" -eq 0 ]; then
        SUDO=""
    else
        SUDO="sudo"
    fi

    # Update package lists first if we need to install things
    UPDATED_APT=false

    ensure_apt_update() {
        if [ "$UPDATED_APT" = false ]; then
            echo -e "${YELLOW}📦 Updating package lists...${NC}"
            $SUDO apt-get update
            UPDATED_APT=true
        fi
    }

    # ==========================================
    # Phase 1: Bootstrap Dependencies
    # (Required to install other things)
    # ==========================================
    local bootstrap_tools=("curl" "wget" "git" "build-essential" "ca-certificates")
    local missing_bootstrap=()

    for cmd in "${bootstrap_tools[@]}"; do
        if ! dpkg -s "$cmd" &> /dev/null && ! command -v "$cmd" &> /dev/null; then
            missing_bootstrap+=("$cmd")
        fi
    done

    if [ ${#missing_bootstrap[@]} -gt 0 ]; then
        echo -e "${YELLOW}🛠️  Installing bootstrap dependencies: ${missing_bootstrap[*]}${NC}"
        ensure_apt_update
        $SUDO apt-get install -y "${missing_bootstrap[@]}"
    fi

    # ==========================================
    # Phase 2: Application Dependencies
    # (Node.js/NPM, Go)
    # ==========================================
    
    # Check Node.js / NPM
    if ! command -v npm &> /dev/null; then
        echo -e "${YELLOW}📦 NPM not found. Installing Node.js...${NC}"
        # Install Node.js 20.x (LTS)
        ensure_apt_update
        # We need curl for this, which is guaranteed by Phase 1
        curl -fsSL https://deb.nodesource.com/setup_20.x | $SUDO -E bash -
        $SUDO apt-get install -y nodejs
        print_success "Node.js & NPM installed"
    fi

    # Check Go (specifically)
    if ! command -v go &> /dev/null; then
        echo -e "${YELLOW}🐹 Go not found. Installing Go 1.22...${NC}"
        
        # Download Go
        wget https://go.dev/dl/go1.22.3.linux-amd64.tar.gz
        
        # Remove old installation
        $SUDO rm -rf /usr/local/go
        
        # Extract
        $SUDO tar -C /usr/local -xzf go1.22.3.linux-amd64.tar.gz
        
        # Cleanup
        rm go1.22.3.linux-amd64.tar.gz
        
        # Add to PATH for current session
        export PATH=$PATH:/usr/local/go/bin
        
        # Persist PATH (try .bashrc and .profile)
        if [ -f "$HOME/.bashrc" ]; then
            if ! grep -q "/usr/local/go/bin" "$HOME/.bashrc"; then
                echo 'export PATH=$PATH:/usr/local/go/bin' >> "$HOME/.bashrc"
            fi
        fi
        if [ -f "$HOME/.profile" ]; then
             if ! grep -q "/usr/local/go/bin" "$HOME/.profile"; then
                echo 'export PATH=$PATH:/usr/local/go/bin' >> "$HOME/.profile"
            fi
        fi
        
        print_success "Go installed"
    else
        # Check version? (Optional, but good practice)
        GO_VERSION=$(go version | awk '{print $3}')
        print_success "Go found ($GO_VERSION)"
    fi

    # Check Playwright
    if [ ! -d "$HOME/.cache/ms-playwright" ] && [ ! -d "$HOME/Library/Caches/ms-playwright" ]; then
        echo -e "${YELLOW}🎭 Installing Playwright browsers...${NC}"
        # Ensure we can run go run
        export PATH=$PATH:/usr/local/go/bin
        
        # Initialize module if needed (should be there)
        if [ ! -f "go.mod" ]; then
             go mod init solana-orchestrator
             go mod tidy
        fi
        
        go run github.com/playwright-community/playwright-go/cmd/playwright@v0.4201.1 install --with-deps
        print_success "Playwright installed"
    else
        print_success "Playwright browsers found"
    fi
    
    # Check Redis
    if ! command -v redis-cli &> /dev/null; then
        echo -e "${YELLOW}🔴 Redis not found. Installing Redis...${NC}"
        ensure_apt_update
        $SUDO apt-get install -y redis-server
        print_success "Redis installed"
    else
        print_success "Redis found"
    fi

    # Check if Redis is running
    if ! redis-cli ping &> /dev/null; then
        echo -e "${YELLOW}🔴 Redis is not running. Starting Redis...${NC}"
        STARTED=false
        
        if command -v systemctl &> /dev/null && systemctl is-system-running &> /dev/null; then
            sudo systemctl start redis-server && STARTED=true
        fi
        
        if [ "$STARTED" = false ] && command -v service &> /dev/null; then
             # Try service but don't fail if it returns non-zero, check ping
             sudo service redis-server start 2>/dev/null || true
             sleep 1
             if redis-cli ping &> /dev/null; then
                STARTED=true
             fi
        fi
        
        if [ "$STARTED" = false ]; then
            # Fallback for non-systemd environments (like docker containers)
            echo -e "${YELLOW}⚠️  Service start failed, trying direct execution...${NC}"
            redis-server --daemonize yes
        fi
        
        sleep 2
        if redis-cli ping &> /dev/null; then
            print_success "Redis started"
        else
            print_warning "Could not start Redis automatically. Please start it manually."
        fi
    else
        print_success "Redis is running"
    fi

    print_success "All dependencies met!"
}

# Build monitor
build_monitor() {
    echo -e "${YELLOW}🔨 Building monitor tool...${NC}"
    if go build -o bin/monitor ./cmd/monitor; then
        print_success "Monitor build successful!"
    else
        print_error "Monitor build failed!"
    fi
}

# Build bot
build_bot() {
    check_dependencies
    echo -e "${YELLOW}🔨 Building telegram bot...${NC}"

    # Check if source exists
    if [ ! -d "cmd/bot" ]; then
        print_error "Source directory 'cmd/bot' not found!"
        exit 1
    fi

    # Create bin directory
    mkdir -p bin

    # Backup old binary
    if [ -f "$BOT_BINARY" ]; then
        cp "$BOT_BINARY" "${BOT_BINARY}.backup"
        print_info "Old binary backed up to ${BOT_BINARY}.backup"
    fi

    # Ensure dependencies are tidy
    echo -e "${YELLOW}📦 Tidying dependencies...${NC}"
    go mod tidy

    # Build Bot
    if go build -o "$BOT_BINARY" ./cmd/bot; then
        print_success "Bot build successful!"
        
        # Build Monitor
        build_monitor

        # Show binary info
        SIZE=$(du -h "$BOT_BINARY" | cut -f1)
        print_info "Binary size: $SIZE"

        return 0
    else
        print_error "Build failed!"

        # Restore backup if build failed
        if [ -f "${BOT_BINARY}.backup" ]; then
            mv "${BOT_BINARY}.backup" "$BOT_BINARY"
            print_warning "Restored previous binary"
        fi

        exit 1
    fi
}

# Start bot
start_bot() {
    echo -e "${YELLOW}🚀 Starting bot...${NC}"

    # Check if already running
    if is_running; then
        PID=$(get_pid)
        print_warning "Bot is already running (PID: $PID)"
        return 0 # Return success so we can proceed to monitor
    fi

    # Kill any orphaned telegram-bot processes
    ORPHANED=$(ps aux | grep "$BOT_BINARY" | grep -v grep | awk '{print $2}')
    if [ -n "$ORPHANED" ]; then
        print_warning "Found orphaned bot process(es), cleaning up..."
        echo "$ORPHANED" | xargs kill -9 2>/dev/null
        sleep 1
    fi

    # Clean up stale PID file
    rm -f "$PID_FILE"

    # Check if binary exists
    if [ ! -f "$BOT_BINARY" ]; then
        print_error "Bot binary not found! Run 'build' first."
        exit 1
    fi

    # Start bot in background
    nohup "$BOT_BINARY" > "$LOG_FILE" 2>&1 &
    NEW_PID=$!

    # Save PID
    echo "$NEW_PID" > "$PID_FILE"

    # Wait a moment and check if it started
    sleep 2

    if is_running; then
        print_success "Bot started successfully (PID: $NEW_PID)"
        return 0
    else
        print_error "Bot failed to start! Check logs:"
        tail -20 "$LOG_FILE"
        exit 1
    fi
}

# Run Monitor
run_monitor() {
    if [ -f "./bin/monitor" ]; then
        ./bin/monitor
    else
        print_error "Monitor binary not found. Run 'build' first."
    fi
}

# Main script
main() {
    # Check if running from correct directory
    if [ ! -f "go.mod" ]; then
        print_error "Must run from project root directory!"
        exit 1
    fi

    COMMAND="${1}"

    if [ -z "$COMMAND" ]; then
        # Default behavior: Install -> Build -> Start -> Monitor
        print_header
        check_dependencies
        build_bot
        start_bot
        run_monitor
        exit 0
    fi

    case "$COMMAND" in
        install)
            print_header
            check_dependencies
            ;;
        build)
            print_header
            build_bot
            ;;
        start)
            print_header
            start_bot
            ;;
        monitor)
            run_monitor
            ;;
        stop)
            print_header
            stop_bot
            ;;
        restart)
            print_header
            restart_bot
            ;;
        status)
            print_header
            show_status
            ;;
        logs)
            show_logs
            ;;
        clean)
            print_header
            clean_logs
            ;;
        rebuild)
            print_header
            stop_bot
            echo ""
            build_bot
            echo ""
            start_bot
            ;;
        help|--help|-h)
            show_help
            ;;
        *)
            print_error "Unknown command: $COMMAND"
            echo ""
            show_help
            exit 1
            ;;
    esac
}

# Run main
main "$@"
