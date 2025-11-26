# Scalability Improvements

## Quick Wins (Can implement immediately)

### 1. SQLite WAL Mode
**Problem:** SQLite uses rollback journal mode by default, which has write contention.
**Solution:** Enable Write-Ahead Logging (WAL) mode for better concurrent reads/writes.

**Implementation:**
```go
// In storage/db.go after opening database:
db.Exec("PRAGMA journal_mode=WAL;")
db.Exec("PRAGMA synchronous=NORMAL;")
db.Exec("PRAGMA cache_size=10000;")
```

**Benefits:**
- Multiple readers can access database while writer is active
- 2-3x better write performance
- Reduces database locked errors

---

### 2. Database Connection Pooling
**Problem:** Each query opens a new connection (inefficient).
**Solution:** Use prepared statements and connection pooling.

**Implementation:**
```go
type DB struct {
    *sql.DB
    saveStmt     *sql.Stmt
    getWalletsStmt *sql.Stmt
}

func New(path string) (*DB, error) {
    db, err := sql.Open("sqlite3", path)
    // ...
    saveStmt, _ := db.Prepare("INSERT INTO wallets ...")
    getWalletsStmt, _ := db.Prepare("SELECT * FROM wallets ...")
    
    return &DB{
        DB: db,
        saveStmt: saveStmt,
        getWalletsStmt: getWalletsStmt,
    }, nil
}
```

**Benefits:**
- Query parsing done once, reused
- 20-30% faster queries
- Less memory allocation

---

### 3. Rate Limiting Per User
**Problem:** One user can spam requests and block others.
**Solution:** Implement per-user rate limiting.

**Implementation:**
```go
type RateLimiter struct {
    requests map[int64]*rate.Limiter
    mu       sync.Mutex
}

func (rl *RateLimiter) Allow(chatID int64) bool {
    rl.mu.Lock()
    defer rl.mu.Unlock()
    
    if _, exists := rl.requests[chatID]; !exists {
        // 5 requests per minute
        rl.requests[chatID] = rate.NewLimiter(rate.Every(12*time.Second), 5)
    }
    
    return rl.requests[chatID].Allow()
}
```

**Benefits:**
- Prevent abuse
- Fair resource distribution
- Protect scanner from overload

---

### 4. Memory Cache with LRU Eviction
**Problem:** In-memory cache grows unbounded; could cause OOM.
**Solution:** Use LRU cache with size limit.

**Implementation:**
```go
import "github.com/hashicorp/golang-lru"

type Scanner struct {
    walletsCache *lru.Cache // Instead of map
    // ...
}

func NewScanner() *Scanner {
    cache, _ := lru.New(10000) // Max 10k wallets
    return &Scanner{
        walletsCache: cache,
    }
}
```

**Benefits:**
- Bounded memory usage
- Evicts least recently used wallets
- Prevents OOM crashes

---

## Medium Term (Requires more changes)

### 5. PostgreSQL Migration
**Problem:** SQLite has limited concurrency and no network access.
**Solution:** Migrate to PostgreSQL.

**Why PostgreSQL:**
- True concurrent writes
- Network access (can run DB on separate server)
- Better performance at scale
- JSONB for flexible data storage

**Implementation Plan:**
1. Update `storage/db.go` to support both SQLite and PostgreSQL
2. Use environment variable to switch between them
3. Create PostgreSQL schema migration
4. Update queries to use PostgreSQL-specific optimizations

**Estimated Effort:** 4-6 hours

---

### 6. Redis for Caching and Pub/Sub
**Problem:** In-memory cache doesn't work across multiple bot instances.
**Solution:** Centralized Redis cache.

**Use Cases:**
- Cache wallet data (TTL: 5 hours)
- Pub/Sub for real-time updates
- Distributed locks for scanner coordination
- Session storage for user states

**Implementation:**
```go
import "github.com/go-redis/redis/v8"

type CacheService struct {
    client *redis.Client
}

func (c *CacheService) SaveWallet(wallet *WalletData) {
    data, _ := json.Marshal(wallet)
    c.client.Set(ctx, wallet.Wallet, data, 5*time.Hour)
}

func (c *CacheService) Publish(channel string, data interface{}) {
    c.client.Publish(ctx, channel, data)
}
```

**Benefits:**
- Share data across multiple bot instances
- Real-time notifications
- Much faster than database

**Estimated Effort:** 6-8 hours

---

### 7. WebSocket-Based Real-Time Updates
**Problem:** Polling creates unnecessary load.
**Solution:** WebSocket connections for instant updates.

**Architecture:**
```
Scanner finds wallet → Redis Pub/Sub → WebSocket Server → Push to users
```

**Implementation:**
```go
// Use gorilla/websocket
type WSManager struct {
    clients map[int64]*websocket.Conn
    mu      sync.RWMutex
}

func (m *WSManager) Broadcast(wallet *WalletData) {
    m.mu.RLock()
    defer m.mu.RUnlock()
    
    for _, conn := range m.clients {
        conn.WriteJSON(wallet)
    }
}
```

**Benefits:**
- Instant updates (no 10-second delay)
- Reduce DB queries by 95%
- Better user experience

**Estimated Effort:** 8-12 hours

---

### 8. Metrics and Monitoring
**Problem:** No visibility into performance or errors.
**Solution:** Add Prometheus metrics and Grafana dashboards.

**Metrics to Track:**
- Scanner performance (wallets/minute, errors)
- API call latency (Moralis, Birdeye, DexCheck)
- Database query times
- Active Telegram users
- Cache hit rate
- Memory usage

**Implementation:**
```go
import "github.com/prometheus/client_golang/prometheus"

var (
    scannedWallets = prometheus.NewCounter(...)
    apiLatency = prometheus.NewHistogram(...)
    dbQueryDuration = prometheus.NewHistogram(...)
)

func init() {
    prometheus.MustRegister(scannedWallets)
    prometheus.MustRegister(apiLatency)
    prometheus.MustRegister(dbQueryDuration)
}
```

**Benefits:**
- Identify bottlenecks
- Alert on errors
- Optimize based on data

**Estimated Effort:** 6-8 hours

---

## Long Term (Architectural Changes)

### 9. Microservices Architecture

**Current:** Monolith (bot + scanner in one process)
**Proposed:** Separate services

**Services:**
1. **Scanner Service**
   - Fetches tokens and analyzes wallets
   - Publishes results to message queue
   - Horizontally scalable (multiple instances)

2. **Bot API Service**
   - Handles Telegram requests
   - Queries database/cache
   - Subscribes to scanner updates

3. **Database Service**
   - PostgreSQL cluster
   - Read replicas for scaling

4. **Queue Service**
   - RabbitMQ or Kafka
   - Decouples scanner from bot

**Benefits:**
- Independent scaling
- Better fault isolation
- Deploy updates without downtime
- Can have 10+ scanner workers

**Estimated Effort:** 40-60 hours

---

### 10. Horizontal Scaling with Load Balancer

**Setup:**
```
                    Load Balancer
                         |
        +----------------+----------------+
        |                |                |
    Bot Instance 1   Bot Instance 2   Bot Instance 3
        |                |                |
        +----------------+----------------+
                         |
                  PostgreSQL Cluster
                         |
                      Redis Cluster
```

**Requirements:**
- Stateless bot instances
- Shared database (PostgreSQL)
- Shared cache (Redis)
- Session affinity or distributed sessions

**Estimated Effort:** 20-30 hours

---

### 11. Kubernetes Deployment

**Components:**
- Deployments for each service
- StatefulSets for databases
- ConfigMaps for configuration
- Secrets for API keys
- HorizontalPodAutoscaler for auto-scaling
- Ingress for external access

**Benefits:**
- Auto-scaling based on load
- Self-healing (restarts crashed pods)
- Rolling updates
- Resource limits

**Example manifest:**
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: scanner-service
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: scanner
        image: solana-scanner:latest
        resources:
          limits:
            memory: "2Gi"
            cpu: "1000m"
```

**Estimated Effort:** 30-40 hours

---

## Priority Recommendations

**If expecting 100-500 users:**
- Implement #1, #2, #3, #4 (Quick Wins)
- **Estimated Total Time:** 4-6 hours

**If expecting 500-2000 users:**
- Quick Wins + PostgreSQL (#5) + Redis (#6)
- **Estimated Total Time:** 16-20 hours

**If expecting 2000+ users:**
- All Medium Term improvements + Microservices (#9)
- **Estimated Total Time:** 60-80 hours

---

## Cost Estimates (Monthly)

**Current Setup (Single Server):**
- VPS (4 vCPU, 8GB RAM): $20-40/month
- **Supports:** ~500 users

**Scaled Setup (PostgreSQL + Redis):**
- VPS (8 vCPU, 16GB RAM): $80-120/month
- Managed PostgreSQL: $50-100/month
- Managed Redis: $30-50/month
- **Total:** $160-270/month
- **Supports:** ~2,000 users

**Enterprise Setup (Kubernetes):**
- Kubernetes cluster: $200-500/month
- Database cluster: $150-300/month
- Redis cluster: $100-200/month
- Load balancer: $20-50/month
- **Total:** $470-1,050/month
- **Supports:** 10,000+ users
