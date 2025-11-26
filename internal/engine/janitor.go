package engine

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"solana-orchestrator/internal/solana"
	"solana-orchestrator/storage"
)

// Notification represents a user alert
type Notification struct {
	UserID int64
	Msg    string
}

// Janitor cleans up expired orders
type Janitor struct {
	DB         *storage.DB
	JitoClient *solana.JitoClient
	// We need a way to build cancel tx. Assuming a helper in solana package or we inject a client.
	// For now, let's assume we have a LimitOrderManager or similar, or we use JitoClient if we add helper there.
	// The prompt implies `j.Solana.BuildCancelOrderTx`.
	// I'll define an interface or struct for Solana interactions needed here.
	SolanaClient *solana.LimitOrderManager
	Notify       chan Notification
	stopChan     chan struct{}
}

// NewJanitor creates a new Janitor
func NewJanitor(db *storage.DB, jito *solana.JitoClient, solClient *solana.LimitOrderManager) *Janitor {
	return &Janitor{
		DB:           db,
		JitoClient:   jito,
		SolanaClient: solClient,
		Notify:       make(chan Notification, 100),
		stopChan:     make(chan struct{}),
	}
}

// Start begins the background cleanup process
func (j *Janitor) Start() {
	ticker := time.NewTicker(1 * time.Minute)
	go func() {
		for {
			select {
			case <-ticker.C:
				j.processExpiredOrders()
			case <-j.stopChan:
				ticker.Stop()
				return
			}
		}
	}()
}

// Stop stops the janitor
func (j *Janitor) Stop() {
	close(j.stopChan)
}

func (j *Janitor) processExpiredOrders() {
	batchSize := 50 // Keep memory footprint tiny

	for {
		// 1. Fetch Batch (Optimized SQL)
		orders, err := j.DB.GetExpiredOrdersBatch(batchSize)
		if err != nil {
			log.Printf("❌ Janitor DB error: %v", err)
			break
		}
		if len(orders) == 0 {
			break // Done for this cycle
		}

		log.Printf("🧹 Janitor: Processing batch of %d orders...", len(orders))

		// 2. Process Batch (Parallel Jito Requests)
		// We use a semaphore to limit concurrent Jito requests to avoid network saturation
		sem := make(chan struct{}, 5) // Max 5 concurrent cancellations
		var wg sync.WaitGroup

		for _, order := range orders {
			wg.Add(1)
			sem <- struct{}{} // Acquire token

			go func(o *storage.LimitOrder) {
				defer wg.Done()
				defer func() { <-sem }() // Release token

				// Build Cancel Transaction
				// Assuming BuildCancelOrderTx returns *solana.Transaction
				tx, err := j.SolanaClient.BuildCancelOrderTx(context.Background(), o.OrderPubkey)
				if err != nil {
					log.Printf("❌ Failed to build cancel tx for %s: %v", o.OrderPubkey, err)
					return
				}

				// Send via Jito (High Priority)
				// Using a small tip for cleanup
				_, err = j.JitoClient.SendJitoBundle(context.Background(), tx, 10000) // 10k lamports tip

				if err == nil {
					// 3. Mark as CANCELLED in DB
					j.DB.UpdateOrderStatus(o.ID, "EXPIRED_REFUNDED")

					// 4. Notify User (Non-blocking)
					select {
					case j.Notify <- Notification{
						UserID: o.UserID,
						Msg:    fmt.Sprintf("⏳ Order Expired & Refunded\n\nToken: %s\nReason: Time Limit Reached", o.TokenSymbol),
					}:
					default:
					}
				} else {
					log.Printf("❌ Failed to send cancel bundle for %s: %v", o.OrderPubkey, err)
				}
			}(order)
		}
		wg.Wait()

		// 3. Optional: Small sleep to let CPU breathe between batches
		time.Sleep(100 * time.Millisecond)
	}
}
