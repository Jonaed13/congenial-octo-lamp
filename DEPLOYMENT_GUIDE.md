# GoSolTrader Deployment Guide

Complete guide for deploying the high-performance Fan-Out Engine on an 8GB VPS.

## System Requirements

### Minimum Specifications
- **RAM**: 8GB (4GB minimum, 8GB recommended)
- **CPU**: 2 cores (4 cores recommended)
- **Storage**: 20GB SSD
- **OS**: Ubuntu 22.04 LTS (recommended)
- **Network**: 100 Mbps (stable connection required)

### Software Requirements
- Go 1.23+
- Redis 6.0+
- SQLite3
- Git

## Step-by-Step Deployment

### 1. Server Setup

```bash
# Update system
sudo apt-get update
sudo apt-get upgrade -y

# Install dependencies
sudo apt-get install -y git build-essential redis-server sqlite3

# Install Go 1.23
wget https://go.dev/dl/go1.23.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.23.0.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# Verify installations
go version
redis-cli --version
```

### 2. Redis Configuration

```bash
# Edit Redis config for production
sudo nano /etc/redis/redis.conf

# Recommended settings:
# maxmemory 2gb
# maxmemory-policy allkeys-lru
# save "" (disable persistence for speed)
# appendonly no

# Restart Redis
sudo systemctl restart redis
sudo systemctl enable redis

# Test Redis
redis-cli ping  # Should return PONG
```

### 3. Clone and Build

```bash
# Clone repository
git clone https://github.com/yourusername/gosoltrader.git
cd gosoltrader

# Install Go dependencies
go mod download

# Build binary
go build -o bin/telegram-bot cmd/bot/*.go
```

### 4. Configuration

```bash
# Copy example config
cp config/config.example.json config/config.json

# Edit configuration
nano config/config.json

# Set required values:
# - shyft_api_key
# - moralis_api_key
# - birdeye_api_key
```

### 5. Environment Variables

```bash
# Create .env file
cat > .env << EOF
TELEGRAM_BOT_TOKEN=your_telegram_bot_token
SHYFT_API_KEY=your_shyft_api_key
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=
REDIS_DB=0
EOF

# Load environment
source .env
```

### 6. Systemd Service (Production)

```bash
# Create service file
sudo nano /etc/systemd/system/gosoltrader.service

# Add content:
[Unit]
Description=GoSolTrader Telegram Bot
After=network.target redis.service

[Service]
Type=simple
User=ubuntu
WorkingDirectory=/home/ubuntu/gosoltrader
EnvironmentFile=/home/ubuntu/gosoltrader/.env
ExecStart=/home/ubuntu/gosoltrader/bin/telegram-bot
Restart=always
RestartSec=10

# Resource limits
LimitNOFILE=65536
MemoryLimit=6G

[Install]
WantedBy=multi-user.target

# Enable and start service
sudo systemctl daemon-reload
sudo systemctl enable gosoltrader
sudo systemctl start gosoltrader

# Check status
sudo systemctl status gosoltrader
```

### 7. Monitoring Setup

```bash
# View logs
sudo journalctl -u gosoltrader -f

# Monitor Redis
redis-cli INFO memory
redis-cli INFO stats

# Monitor system resources
htop
free -h
df -h
```

## Performance Tuning

### For 8GB RAM VPS

**config.json optimizations**:
```json
{
  "fanout_engine": {
    "worker_count": 20,
    "log_buffer_size": 50000,
    "notification_rate_limit": 25
  },
  "redis": {
    "pool_size": 50,
    "min_idle_conns": 10
  }
}
```

**System optimizations**:
```bash
# Increase file descriptors
echo "* soft nofile 65536" | sudo tee -a /etc/security/limits.conf
echo "* hard nofile 65536" | sudo tee -a /etc/security/limits.conf

# Optimize network
sudo sysctl -w net.core.somaxconn=1024
sudo sysctl -w net.ipv4.tcp_max_syn_backlog=2048
```

### For 4GB RAM VPS (Reduced Settings)

```json
{
  "fanout_engine": {
    "worker_count": 10,
    "log_buffer_size": 25000,
    "notification_rate_limit": 15
  },
  "redis": {
    "pool_size": 25,
    "min_idle_conns": 5
  }
}
```

## Scaling Guidelines

### User Capacity

| VPS RAM | Max Users | Max Monitored Wallets | Worker Count |
|---------|-----------|----------------------|-------------|
| 4GB     | 500       | 500                  | 10          |
| 8GB     | 2,000     | 2,000                | 20          |
| 16GB    | 5,000     | 5,000                | 40          |

### When to Scale Up

**Indicators**:
- Memory usage > 80%
- Redis memory > 1.5GB
- Log buffer drops (check logs)
- Trade execution latency > 2s

**Solutions**:
1. Increase VPS RAM
2. Add Redis replica for reads
3. Implement horizontal scaling (multiple bots)

## Security Hardening

```bash
# Firewall setup
sudo ufw default deny incoming
sudo ufw default allow outgoing
sudo ufw allow ssh
sudo ufw allow 6379/tcp  # Redis (only if needed externally)
sudo ufw enable

# Secure Redis
sudo nano /etc/redis/redis.conf
# Add: requirepass your_strong_password
# Add: bind 127.0.0.1

# Encrypt sensitive data
# Store API keys in environment variables, not config files
```

## Backup Strategy

```bash
# Database backup
sqlite3 bot.db ".backup bot_backup.db"

# Redis backup (if persistence enabled)
redis-cli BGSAVE

# Automated daily backup
crontab -e
# Add: 0 2 * * * /home/ubuntu/gosoltrader/scripts/backup.sh
```

## Troubleshooting

### High Memory Usage
```bash
# Check memory breakdown
free -h
ps aux --sort=-%mem | head -10

# Check Redis memory
redis-cli INFO memory

# Restart if needed
sudo systemctl restart gosoltrader
```

### WebSocket Disconnections
```bash
# Check logs
sudo journalctl -u gosoltrader -n 100

# Test Shyft connection
wscat -c "wss://rpc.shyft.to?api_key=YOUR_KEY"

# Check network
ping -c 10 rpc.shyft.to
```

### Database Locked Errors
```bash
# Check open connections
lsof | grep bot.db

# Increase connection pool in code
# db.SetMaxOpenConns(50)
```

## Maintenance

### Daily Tasks
- Check logs for errors
- Monitor memory usage
- Verify Redis is running

### Weekly Tasks
- Review trade execution times
- Check for failed transactions
- Update dependencies

### Monthly Tasks
- Backup database
- Review and optimize config
- Update Go and system packages

## Support

For issues or questions:
- GitHub Issues: https://github.com/yourusername/gosoltrader/issues
- Telegram: @yourusername
- Documentation: https://docs.gosoltrader.com
