package trading

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/time/rate"
)

// WSClient manages WebSocket connection to Shyft
type WSClient struct {
	url            string
	conn           *websocket.Conn
	mu             sync.RWMutex
	subscriptions  map[string]chan interface{}
	reconnectDelay time.Duration
	pingInterval   time.Duration
	rpsLimiter     *rate.Limiter // 20 RPS
	apiLimiter     *rate.Limiter // 1 API/sec
	isConnected    bool
	closeChan      chan struct{}
}

// NewWSClient creates a new WebSocket client with rate limiting
func NewWSClient(url string) *WSClient {
	return &WSClient{
		url:            url,
		subscriptions:  make(map[string]chan interface{}),
		reconnectDelay: 5 * time.Second,
		pingInterval:   30 * time.Second,
		rpsLimiter:     rate.NewLimiter(rate.Limit(20), 20), // 20 RPS
		apiLimiter:     rate.NewLimiter(rate.Limit(1), 1),   // 1 API/sec
		closeChan:      make(chan struct{}),
	}
}

// Connect establishes WebSocket connection
func (ws *WSClient) Connect(ctx context.Context) error {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	// Wait for rate limit
	if err := ws.apiLimiter.Wait(ctx); err != nil {
		return err
	}

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, ws.url, nil)
	if err != nil {
		return fmt.Errorf("websocket dial failed: %w", err)
	}

	ws.conn = conn
	ws.isConnected = true

	// Start message handler
	go ws.handleMessages()

	// Start ping/pong
	go ws.keepAlive()

	return nil
}

// handleMessages processes incoming WebSocket messages
func (ws *WSClient) handleMessages() {
	defer func() {
		ws.mu.Lock()
		ws.isConnected = false
		ws.mu.Unlock()
	}()

	for {
		select {
		case <-ws.closeChan:
			return
		default:
			_, message, err := ws.conn.ReadMessage()
			if err != nil {
				fmt.Printf("WebSocket read error: %v\n", err)
				// Trigger reconnection
				go ws.reconnect()
				return
			}

			// Parse message and route to subscribers
			ws.routeMessage(message)
		}
	}
}

// routeMessage distributes messages to subscribers
func (ws *WSClient) routeMessage(message []byte) {
	var data map[string]interface{}
	if err := json.Unmarshal(message, &data); err != nil {
		return
	}

	// Extract subscription ID and send to appropriate channel
	if subID, ok := data["subscription"].(string); ok {
		ws.mu.RLock()
		if ch, exists := ws.subscriptions[subID]; exists {
			select {
			case ch <- data:
			default:
				// Channel full, skip
			}
		}
		ws.mu.RUnlock()
	}
}

// keepAlive sends periodic pings to keep connection alive
func (ws *WSClient) keepAlive() {
	ticker := time.NewTicker(ws.pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ws.closeChan:
			return
		case <-ticker.C:
			ws.mu.Lock()
			if ws.isConnected {
				if err := ws.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					fmt.Printf("Ping failed: %v\n", err)
					ws.isConnected = false
					ws.mu.Unlock()
					go ws.reconnect()
					return
				}
			}
			ws.mu.Unlock()
		}
	}
}

// reconnect attempts to re-establish connection
func (ws *WSClient) reconnect() {
	ws.mu.Lock()
	if ws.isConnected {
		ws.mu.Unlock()
		return
	}
	ws.mu.Unlock()

	for {
		select {
		case <-ws.closeChan:
			return
		case <-time.After(ws.reconnectDelay):
			fmt.Println("Attempting WebSocket reconnection...")
			if err := ws.Connect(context.Background()); err != nil {
				fmt.Printf("Reconnection failed: %v\n", err)
				continue
			}
			fmt.Println("WebSocket reconnected successfully")

			// Resubscribe to all previous subscriptions
			ws.resubscribeAll()
			return
		}
	}
}

// resubscribeAll re-establishes all active subscriptions
func (ws *WSClient) resubscribeAll() {
	ws.mu.RLock()
	subs := make(map[string]chan interface{})
	for subID, ch := range ws.subscriptions {
		subs[subID] = ch
	}
	ws.mu.RUnlock()

	// Re-send subscription requests for each account
	for account := range subs {
		req := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "accountSubscribe",
			"params": []interface{}{
				account,
				map[string]string{"encoding": "jsonParsed"},
			},
		}

		ws.mu.Lock()
		if ws.conn != nil && ws.isConnected {
			if err := ws.conn.WriteJSON(req); err != nil {
				fmt.Printf("Failed to resubscribe to %s: %v\n", account, err)
			} else {
				fmt.Printf("Resubscribed to account: %s\n", account)
			}
		}
		ws.mu.Unlock()
	}
}

// SubscribeAccount subscribes to account updates
func (ws *WSClient) SubscribeAccount(ctx context.Context, account string) (<-chan interface{}, error) {
	// Wait for rate limit
	if err := ws.rpsLimiter.Wait(ctx); err != nil {
		return nil, err
	}

	ws.mu.Lock()
	defer ws.mu.Unlock()

	if !ws.isConnected {
		return nil, fmt.Errorf("websocket not connected")
	}

	// Create channel for this subscription
	ch := make(chan interface{}, 100)
	ws.subscriptions[account] = ch

	// Send subscription request
	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "accountSubscribe",
		"params": []interface{}{
			account,
			map[string]string{"encoding": "jsonParsed"},
		},
	}

	if err := ws.conn.WriteJSON(req); err != nil {
		delete(ws.subscriptions, account)
		close(ch)
		return nil, fmt.Errorf("failed to send subscription: %w", err)
	}

	return ch, nil
}

// SubscribeLogs subscribes to transaction logs for an address
func (ws *WSClient) SubscribeLogs(ctx context.Context, mention string) (<-chan interface{}, error) {
	// Wait for rate limit
	if err := ws.rpsLimiter.Wait(ctx); err != nil {
		return nil, err
	}

	ws.mu.Lock()
	defer ws.mu.Unlock()

	if !ws.isConnected {
		return nil, fmt.Errorf("websocket not connected")
	}

	// Create channel for this subscription
	ch := make(chan interface{}, 100)
	ws.subscriptions[mention] = ch

	// Send subscription request
	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "logsSubscribe",
		"params": []interface{}{
			map[string]interface{}{
				"mentions": []string{mention},
			},
			map[string]string{"commitment": "finalized"},
		},
	}

	if err := ws.conn.WriteJSON(req); err != nil {
		delete(ws.subscriptions, mention)
		close(ch)
		return nil, fmt.Errorf("failed to send subscription: %w", err)
	}

	return ch, nil
}

// Unsubscribe removes a subscription
func (ws *WSClient) Unsubscribe(account string) {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	if ch, exists := ws.subscriptions[account]; exists {
		close(ch)
		delete(ws.subscriptions, account)
	}
}

// Close closes the WebSocket connection
func (ws *WSClient) Close() error {
	close(ws.closeChan)

	ws.mu.Lock()
	defer ws.mu.Unlock()

	// Close all subscription channels
	for _, ch := range ws.subscriptions {
		close(ch)
	}
	ws.subscriptions = make(map[string]chan interface{})

	if ws.conn != nil {
		return ws.conn.Close()
	}
	return nil
}

// IsConnected returns connection status
func (ws *WSClient) IsConnected() bool {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	return ws.isConnected
}

// SubscribeProgramLogs subscribes to logs for a specific program
func (ws *WSClient) SubscribeProgramLogs(ctx context.Context, programID string) (<-chan interface{}, error) {
	// Wait for rate limit
	if err := ws.rpsLimiter.Wait(ctx); err != nil {
		return nil, err
	}

	ws.mu.Lock()
	defer ws.mu.Unlock()

	if !ws.isConnected {
		return nil, fmt.Errorf("websocket not connected")
	}

	// Create channel for this subscription
	ch := make(chan interface{}, 50000) // Large buffer for program logs
	ws.subscriptions[programID] = ch

	// Send subscription request
	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "logsSubscribe",
		"params": []interface{}{
			map[string]interface{}{
				"mentions": []string{programID},
			},
			map[string]string{"commitment": "processed"},
		},
	}

	if err := ws.conn.WriteJSON(req); err != nil {
		delete(ws.subscriptions, programID)
		close(ch)
		return nil, fmt.Errorf("failed to send subscription: %w", err)
	}

	return ch, nil
}
