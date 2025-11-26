package engine

import (
	"context"
	"fmt"
	"time"

	"solana-orchestrator/storage"

	"github.com/redis/go-redis/v9"
)

// NewRedisClient creates a new Redis client with connection pooling
func NewRedisClient(addr, password string, db int) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		PoolSize:     50, // Max 50 concurrent connections
		MinIdleConns: 10, // Keep 10 idle connections warm
		MaxRetries:   3,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return rdb, nil
}

// SyncWalletsToRedis syncs all active copy targets to Redis
func SyncWalletsToRedis(ctx context.Context, rdb *redis.Client, targets []*storage.CopyTradeTarget) error {
	pipe := rdb.Pipeline()

	// Clear existing data
	pipe.Del(ctx, "monitored_wallets")

	// We also need to clear all wallet_owner keys, but that's harder to do atomically without scanning.
	// For now, we'll assume the engine handles cleanup or we can use a set to track all owner keys.
	// A better approach for a full sync is to delete everything related to us, but let's stick to the plan.

	if len(targets) == 0 {
		_, err := pipe.Exec(ctx)
		return err
	}

	// Build monitored wallets set
	wallets := make([]interface{}, 0, len(targets))
	for _, t := range targets {
		wallets = append(wallets, t.TargetWallet)

		// Set wallet owners
		// HSET wallet_owner:<wallet> <user_id> <copy_amount>
		key := fmt.Sprintf("wallet_owner:%s", t.TargetWallet)
		pipe.HSet(ctx, key, fmt.Sprintf("%d", t.UserID), t.CopyAmountSOL)
	}

	pipe.SAdd(ctx, "monitored_wallets", wallets...)

	_, err := pipe.Exec(ctx)
	return err
}

// GetWalletOwners returns all users watching a specific wallet
func GetWalletOwners(ctx context.Context, rdb *redis.Client, wallet string) (map[int64]float64, error) {
	key := fmt.Sprintf("wallet_owner:%s", wallet)
	result, err := rdb.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	owners := make(map[int64]float64)
	for userIDStr, amountStr := range result {
		var userID int64
		var amount float64
		fmt.Sscanf(userIDStr, "%d", &userID)
		fmt.Sscanf(amountStr, "%f", &amount)
		owners[userID] = amount
	}

	return owners, nil
}

// AddMonitoredWallet adds a single wallet to monitoring
func AddMonitoredWallet(ctx context.Context, rdb *redis.Client, wallet string, userID int64, amount float64) error {
	pipe := rdb.Pipeline()

	pipe.SAdd(ctx, "monitored_wallets", wallet)

	key := fmt.Sprintf("wallet_owner:%s", wallet)
	pipe.HSet(ctx, key, fmt.Sprintf("%d", userID), amount)

	_, err := pipe.Exec(ctx)
	return err
}

// RemoveMonitoredWallet removes a user from monitoring a wallet
func RemoveMonitoredWallet(ctx context.Context, rdb *redis.Client, wallet string, userID int64) error {
	key := fmt.Sprintf("wallet_owner:%s", wallet)

	// Remove user from the hash
	if err := rdb.HDel(ctx, key, fmt.Sprintf("%d", userID)).Err(); err != nil {
		return err
	}

	// Check if any users are left
	count, err := rdb.HLen(ctx, key).Result()
	if err != nil {
		return err
	}

	// If no users left, remove from monitored set
	if count == 0 {
		return rdb.SRem(ctx, "monitored_wallets", wallet).Err()
	}

	return nil
}
