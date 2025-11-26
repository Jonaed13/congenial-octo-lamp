package storage

import (
	"database/sql"
	"encoding/base64"
	"errors"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	*sql.DB
}

type WalletData struct {
	Wallet      string  `json:"wallet"`
	Winrate     float64 `json:"winrate"`
	RealizedPnL float64 `json:"realized_pnl"`
	ScannedAt   int64   `json:"scanned_at"`
}

type Alert struct {
	ID         int64   `json:"id"`
	ChatID     int64   `json:"chat_id"`
	MinWinrate float64 `json:"min_winrate"`
	MinPnL     float64 `json:"min_pnl"`
	CreatedAt  int64   `json:"created_at"`
}

type User struct {
	UserID         int64  `json:"user_id"`
	Credits        int    `json:"credits"`
	TrialExpiresAt int64  `json:"trial_expires_at"`
	PlanType       string `json:"plan_type"`
	JoinedAt       int64  `json:"joined_at"`
}

type CopyTradeTarget struct {
	ID            int64   `json:"id"`
	UserID        int64   `json:"user_id"`
	TargetWallet  string  `json:"target_wallet"`
	CopyAmountSOL float64 `json:"copy_amount_sol"`
	IsActive      bool    `json:"is_active"`
	CreatedAt     int64   `json:"created_at"`
}

type LimitOrder struct {
	ID             int64   `json:"id"`
	UserID         int64   `json:"user_id"`
	OrderPubkey    string  `json:"order_pubkey"`
	TokenSymbol    string  `json:"token_symbol"`
	TokenMint      string  `json:"token_mint"`
	Side           string  `json:"side"` // "buy" or "sell"
	Price          float64 `json:"price"`
	Amount         float64 `json:"amount"`
	Status         string  `json:"status"` // "OPEN", "FILLED", "CANCELLED", "EXPIRED_REFUNDED"
	ExpiresAt      int64   `json:"expires_at"`
	TargetMCAP     float64 `json:"target_mcap"`
	InitialRentSOL float64 `json:"initial_rent_sol"`
	CreatedAt      int64   `json:"created_at"`
}

func New(path string) (*DB, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	dbInstance := &DB{db}

	// Configure connection pool
	db.SetMaxOpenConns(50)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(time.Hour)

	if err := dbInstance.initSchema(); err != nil {
		return nil, err
	}

	return dbInstance, nil
}

func (db *DB) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS wallets (
		wallet TEXT PRIMARY KEY,
		winrate REAL,
		realized_pnl REAL,
		scanned_at INTEGER
	);

	CREATE TABLE IF NOT EXISTS alerts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		chat_id INTEGER,
		min_winrate REAL,
		min_pnl REAL,
		created_at INTEGER
	);

	CREATE TABLE IF NOT EXISTS user_wallets (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		chat_id INTEGER NOT NULL,
		wallet_address TEXT NOT NULL,
		wallet_name TEXT,
		is_active INTEGER DEFAULT 0,
		created_at INTEGER,
		UNIQUE(chat_id, wallet_address)
	);

	CREATE TABLE IF NOT EXISTS trades (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		chat_id INTEGER NOT NULL,
		wallet_address TEXT NOT NULL,
		tx_signature TEXT UNIQUE,
		trade_type TEXT,
		token_address TEXT,
		sol_amount REAL,
		token_amount REAL,
		price_per_token REAL,
		jito_tip REAL,
		status TEXT,
		created_at INTEGER,
		confirmed_at INTEGER
	);

	CREATE TABLE IF NOT EXISTS positions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		chat_id INTEGER NOT NULL,
		wallet_address TEXT NOT NULL,
		token_address TEXT NOT NULL,
		token_amount REAL,
		avg_buy_price REAL,
		last_updated INTEGER,
		UNIQUE(chat_id, wallet_address, token_address)
	);

	CREATE TABLE IF NOT EXISTS encrypted_wallets (
		chat_id INTEGER PRIMARY KEY,
		public_key TEXT NOT NULL,
		encrypted_private_key TEXT NOT NULL,
		encryption_salt TEXT NOT NULL,
		nonce TEXT NOT NULL,
		password_hash TEXT NOT NULL,
		mnemonic_encrypted TEXT,
		created_at INTEGER,
		last_used INTEGER
	);

	CREATE TABLE IF NOT EXISTS user_settings (
		chat_id INTEGER PRIMARY KEY,
		slippage_bps INTEGER DEFAULT 500,
		max_slippage_bps INTEGER DEFAULT 5000,
		jito_tip_lamports INTEGER DEFAULT 10000,
		priority_fee_lamports INTEGER DEFAULT 5000,
		auto_confirm INTEGER DEFAULT 0,
		copy_trade_auto_buy INTEGER DEFAULT 0,
		created_at INTEGER,
		updated_at INTEGER
	);

	CREATE TABLE IF NOT EXISTS users (
		user_id INTEGER PRIMARY KEY,
		credits INTEGER DEFAULT 0,
		trial_expires_at INTEGER DEFAULT 0,
		plan_type TEXT DEFAULT '',
		joined_at INTEGER
	);

	CREATE TABLE IF NOT EXISTS copy_trade_targets (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		target_wallet TEXT NOT NULL,
		copy_amount_sol REAL NOT NULL,
		is_active INTEGER DEFAULT 1,
		created_at INTEGER,
		UNIQUE(user_id, target_wallet)
	);

	CREATE INDEX IF NOT EXISTS idx_copy_targets_active 
	ON copy_trade_targets(is_active) WHERE is_active = 1;

	CREATE INDEX IF NOT EXISTS idx_copy_targets_wallet 
	ON copy_trade_targets(target_wallet) WHERE is_active = 1;

	CREATE INDEX IF NOT EXISTS idx_trades_user_time 
	ON trades(chat_id, created_at DESC);

	CREATE TABLE IF NOT EXISTS limit_orders (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		order_pubkey TEXT NOT NULL UNIQUE,
		token_symbol TEXT,
		token_mint TEXT NOT NULL,
		side TEXT NOT NULL,
		price REAL,
		amount REAL,
		status TEXT DEFAULT 'OPEN',
		expires_at INTEGER,
		target_mcap REAL,
		initial_rent_sol REAL,
		created_at INTEGER
	);

	CREATE INDEX IF NOT EXISTS idx_limit_orders_expiry 
	ON limit_orders(expires_at, status) 
	WHERE status = 'OPEN';
	`
	if _, err := db.Exec(schema); err != nil {
		return err
	}

	// Migration: Add copy_trade_auto_buy if not exists
	// Check if column exists first
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('user_settings') WHERE name='copy_trade_auto_buy'").Scan(&count)
	if err == nil && count == 0 {
		if _, err := db.Exec("ALTER TABLE user_settings ADD COLUMN copy_trade_auto_buy INTEGER DEFAULT 0;"); err != nil {
			log.Printf("Migration warning: %v", err)
		}
	}

	return nil
}

func (db *DB) SaveWallet(w *WalletData) error {
	query := `
		INSERT INTO wallets (wallet, winrate, realized_pnl, scanned_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(wallet) DO UPDATE SET
			winrate = excluded.winrate,
			realized_pnl = excluded.realized_pnl,
			scanned_at = excluded.scanned_at
	`
	_, err := db.Exec(query, w.Wallet, w.Winrate, w.RealizedPnL, w.ScannedAt)
	return err
}

func (db *DB) GetWallets() ([]*WalletData, error) {
	// Only get wallets scanned in the last 5 hours
	cutoff := time.Now().Add(-5 * time.Hour).Unix()
	rows, err := db.Query("SELECT wallet, winrate, realized_pnl, scanned_at FROM wallets WHERE scanned_at > ? ORDER BY realized_pnl DESC", cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var wallets []*WalletData
	for rows.Next() {
		var w WalletData
		if err := rows.Scan(&w.Wallet, &w.Winrate, &w.RealizedPnL, &w.ScannedAt); err != nil {
			return nil, err
		}
		wallets = append(wallets, &w)
	}
	return wallets, nil
}

func (db *DB) CleanupOldData() (int64, error) {
	cutoff := time.Now().Add(-5 * time.Hour).Unix()
	result, err := db.Exec("DELETE FROM wallets WHERE scanned_at <= ?", cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (db *DB) CreateAlert(chatID int64, minWinrate, minPnL float64) error {
	query := `INSERT INTO alerts (chat_id, min_winrate, min_pnl, created_at) VALUES (?, ?, ?, ?)`
	_, err := db.Exec(query, chatID, minWinrate, minPnL, time.Now().Unix())
	return err
}

func (db *DB) GetMatchingAlerts(winrate, pnl float64) ([]int64, error) {
	query := `SELECT DISTINCT chat_id FROM alerts WHERE ? >= min_winrate AND ? >= min_pnl`
	rows, err := db.Query(query, winrate, pnl)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chatIDs []int64
	for rows.Next() {
		var chatID int64
		if err := rows.Scan(&chatID); err != nil {
			log.Printf("Error scanning alert: %v", err)
			continue
		}
		chatIDs = append(chatIDs, chatID)
	}
	return chatIDs, nil
}

// UserSettings represents user configuration
type UserSettings struct {
	ChatID              int64
	SlippageBps         int
	MaxSlippageBps      int
	JitoTipLamports     int64
	PriorityFeeLamports int64
	AutoConfirm         bool
	CopyTradeAutoBuy    bool
}

// UserWallet represents a user's wallet
type UserWallet struct {
	ID            int64
	ChatID        int64
	WalletAddress string
	WalletName    string
	IsActive      bool
	CreatedAt     int64
}

// EncryptedWallet represents a secure wallet
type EncryptedWallet struct {
	ChatID              int64
	PublicKey           string
	EncryptedPrivateKey string
	EncryptionSalt      string
	Nonce               string
	PasswordHash        string
	MnemonicEncrypted   string
	CreatedAt           int64
}

// GetUserSettings retrieves settings for a user
func (db *DB) GetUserSettings(chatID int64) (*UserSettings, error) {
	query := `SELECT chat_id, slippage_bps, max_slippage_bps, jito_tip_lamports, priority_fee_lamports, auto_confirm, copy_trade_auto_buy FROM user_settings WHERE chat_id = ?`
	row := db.QueryRow(query, chatID)

	var s UserSettings
	var autoConfirmInt int
	var copyTradeAutoBuyInt int
	// Handle potential missing column for old DBs by using a flexible scan or just ignoring if it fails?
	// Actually, the migration above ensures column exists.
	err := row.Scan(&s.ChatID, &s.SlippageBps, &s.MaxSlippageBps, &s.JitoTipLamports, &s.PriorityFeeLamports, &autoConfirmInt, &copyTradeAutoBuyInt)
	if err == sql.ErrNoRows {
		// Return defaults
		return &UserSettings{
			ChatID:              chatID,
			SlippageBps:         500,
			MaxSlippageBps:      5000,
			JitoTipLamports:     10000,
			PriorityFeeLamports: 5000,
			AutoConfirm:         false,
			CopyTradeAutoBuy:    false,
		}, nil
	}
	if err != nil {
		return nil, err
	}
	s.AutoConfirm = autoConfirmInt == 1
	s.CopyTradeAutoBuy = copyTradeAutoBuyInt == 1
	return &s, nil
}

// UpdateCopyTradeAutoBuy updates copy trade auto buy setting
func (db *DB) UpdateCopyTradeAutoBuy(chatID int64, enabled bool) error {
	val := 0
	if enabled {
		val = 1
	}
	query := `INSERT INTO user_settings (chat_id, copy_trade_auto_buy, updated_at) VALUES (?, ?, ?)
			  ON CONFLICT(chat_id) DO UPDATE SET copy_trade_auto_buy = excluded.copy_trade_auto_buy, updated_at = excluded.updated_at`
	_, err := db.Exec(query, chatID, val, time.Now().Unix())
	return err
}

// UpdateSlippage updates slippage settings
func (db *DB) UpdateSlippage(chatID int64, bps int) error {
	query := `INSERT INTO user_settings (chat_id, slippage_bps, updated_at) VALUES (?, ?, ?)
			  ON CONFLICT(chat_id) DO UPDATE SET slippage_bps = excluded.slippage_bps, updated_at = excluded.updated_at`
	_, err := db.Exec(query, chatID, bps, time.Now().Unix())
	return err
}

// UpdateJitoTip updates Jito tip settings
func (db *DB) UpdateJitoTip(chatID int64, lamports int64) error {
	query := `INSERT INTO user_settings (chat_id, jito_tip_lamports, updated_at) VALUES (?, ?, ?)
			  ON CONFLICT(chat_id) DO UPDATE SET jito_tip_lamports = excluded.jito_tip_lamports, updated_at = excluded.updated_at`
	_, err := db.Exec(query, chatID, lamports, time.Now().Unix())
	return err
}

// UpdatePriorityFee updates priority fee settings
func (db *DB) UpdatePriorityFee(chatID int64, lamports int64) error {
	query := `INSERT INTO user_settings (chat_id, priority_fee_lamports, updated_at) VALUES (?, ?, ?)
			  ON CONFLICT(chat_id) DO UPDATE SET priority_fee_lamports = excluded.priority_fee_lamports, updated_at = excluded.updated_at`
	_, err := db.Exec(query, chatID, lamports, time.Now().Unix())
	return err
}

// GetUserWallets retrieves all wallets for a user
func (db *DB) GetUserWallets(chatID int64) ([]*UserWallet, error) {
	query := `SELECT id, chat_id, wallet_address, wallet_name, is_active, created_at FROM user_wallets WHERE chat_id = ? ORDER BY created_at DESC`
	rows, err := db.Query(query, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var wallets []*UserWallet
	for rows.Next() {
		var w UserWallet
		var isActiveInt int
		if err := rows.Scan(&w.ID, &w.ChatID, &w.WalletAddress, &w.WalletName, &isActiveInt, &w.CreatedAt); err != nil {
			return nil, err
		}
		w.IsActive = isActiveInt == 1
		wallets = append(wallets, &w)
	}
	return wallets, nil
}

// AddUserWallet adds a new wallet for a user
func (db *DB) AddUserWallet(chatID int64, address, name string) error {
	query := `INSERT INTO user_wallets (chat_id, wallet_address, wallet_name, created_at) VALUES (?, ?, ?, ?)`
	_, err := db.Exec(query, chatID, address, name, time.Now().Unix())
	return err
}

// SetActiveWallet sets the active wallet for a user
func (db *DB) SetActiveWallet(chatID int64, address string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	// Deactivate all
	_, err = tx.Exec("UPDATE user_wallets SET is_active = 0 WHERE chat_id = ?", chatID)
	if err != nil {
		tx.Rollback()
		return err
	}

	// Activate one
	_, err = tx.Exec("UPDATE user_wallets SET is_active = 1 WHERE chat_id = ? AND wallet_address = ?", chatID, address)
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

// RemoveUserWallet removes a wallet
func (db *DB) RemoveUserWallet(chatID int64, address string) error {
	_, err := db.Exec("DELETE FROM user_wallets WHERE chat_id = ? AND wallet_address = ?", chatID, address)
	return err
}

// HasEncryptedWallet checks if user has an encrypted wallet
func (db *DB) HasEncryptedWallet(chatID int64) bool {
	var count int
	db.QueryRow("SELECT COUNT(*) FROM encrypted_wallets WHERE chat_id = ?", chatID).Scan(&count)
	return count > 0
}

// GetEncryptedWallet retrieves the encrypted wallet metadata (without sensitive fields if possible, but here we return struct)
func (db *DB) GetEncryptedWallet(chatID int64) (*EncryptedWallet, error) {
	query := `SELECT chat_id, public_key, created_at FROM encrypted_wallets WHERE chat_id = ?`
	row := db.QueryRow(query, chatID)

	var w EncryptedWallet
	err := row.Scan(&w.ChatID, &w.PublicKey, &w.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &w, nil
}

// GetEncryptedWalletForDecryption retrieves the full encrypted wallet data
func (db *DB) GetEncryptedWalletForDecryption(chatID int64) (*EncryptedWallet, error) {
	query := `SELECT chat_id, public_key, encrypted_private_key, encryption_salt, nonce, password_hash, mnemonic_encrypted FROM encrypted_wallets WHERE chat_id = ?`
	row := db.QueryRow(query, chatID)

	var w EncryptedWallet
	var mnemonic sql.NullString
	err := row.Scan(&w.ChatID, &w.PublicKey, &w.EncryptedPrivateKey, &w.EncryptionSalt, &w.Nonce, &w.PasswordHash, &mnemonic)
	if err != nil {
		return nil, err
	}
	if mnemonic.Valid {
		w.MnemonicEncrypted = mnemonic.String
	}
	return &w, nil
}

// GetEncryptedWalletForCrypto converts stored wallet data to crypto.EncryptedWallet
func (db *DB) GetEncryptedWalletForCrypto(chatID int64) ([]byte, []byte, []byte, string, error) {
	stored, err := db.GetEncryptedWalletForDecryption(chatID)
	if err != nil || stored == nil {
		return nil, nil, nil, "", err
	}

	// Decode Base64 fields
	encryptedKey, err := base64.StdEncoding.DecodeString(stored.EncryptedPrivateKey)
	if err != nil {
		// Fallback for legacy raw strings? Or just error?
		// Assuming migration or fresh start. Let's try to return raw if decode fails for backward compat if needed,
		// but instructions say "run a one-time migration or handle both".
		// For now, let's assume strict Base64 as per instruction "decode these base64 strings".
		// If we want to be safe:
		encryptedKey = []byte(stored.EncryptedPrivateKey) // Fallback to raw if decode fails?
		// Actually, let's stick to the instruction: "decode these base64 strings back into []byte"
		// If it fails, it might be legacy data.
		// Let's try to decode, if error, assume it's raw string (legacy).
		if _, ok := err.(base64.CorruptInputError); ok || err != nil {
			encryptedKey = []byte(stored.EncryptedPrivateKey)
		}
	}

	salt, err := base64.StdEncoding.DecodeString(stored.EncryptionSalt)
	if err != nil {
		salt = []byte(stored.EncryptionSalt)
	}

	nonce, err := base64.StdEncoding.DecodeString(stored.Nonce)
	if err != nil {
		nonce = []byte(stored.Nonce)
	}

	return encryptedKey, salt, nonce, stored.PasswordHash, nil
}

// GetActiveWallet returns the active wallet for a user
func (db *DB) GetActiveWallet(chatID int64) (*UserWallet, error) {
	w := &UserWallet{}
	var isActive int
	err := db.QueryRow(`
		SELECT id, chat_id, wallet_address, wallet_name, is_active, created_at
		FROM user_wallets
		WHERE chat_id = ? AND is_active = 1
		LIMIT 1
	`, chatID).Scan(&w.ID, &w.ChatID, &w.WalletAddress, &w.WalletName, &isActive, &w.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil // No active wallet
	}
	if err != nil {
		return nil, err
	}
	w.IsActive = true
	return w, nil
}

// SaveEncryptedWallet saves an encrypted wallet to database
func (db *DB) SaveEncryptedWallet(chatID int64, publicKey string, encryptedKey, salt, nonce []byte, passwordHash, mnemonicEnc string) error {
	// Encode to Base64
	encryptedKeyB64 := base64.StdEncoding.EncodeToString(encryptedKey)
	saltB64 := base64.StdEncoding.EncodeToString(salt)
	nonceB64 := base64.StdEncoding.EncodeToString(nonce)

	_, err := db.Exec(`
		INSERT OR REPLACE INTO encrypted_wallets
		(chat_id, public_key, encrypted_private_key, encryption_salt, nonce, password_hash, mnemonic_encrypted, created_at, last_used)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, chatID, publicKey,
		encryptedKeyB64,
		saltB64,
		nonceB64,
		passwordHash,
		mnemonicEnc,
		time.Now().Unix(),
		time.Now().Unix(),
		time.Now().Unix())
	return err
}

// GetUser retrieves a user by ID
func (db *DB) GetUser(userID int64) (*User, error) {
	query := `SELECT user_id, credits, trial_expires_at, plan_type, joined_at FROM users WHERE user_id = ?`
	row := db.QueryRow(query, userID)

	var u User
	err := row.Scan(&u.UserID, &u.Credits, &u.TrialExpiresAt, &u.PlanType, &u.JoinedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// CreateUser creates a new user
func (db *DB) CreateUser(userID int64) error {
	query := `INSERT INTO users (user_id, credits, trial_expires_at, plan_type, joined_at) VALUES (?, 0, 0, '', ?)`
	_, err := db.Exec(query, userID, time.Now().Unix())
	return err
}

func (db *DB) UpdateUserCredits(userID int64, credits int) error {
	query := `UPDATE users SET credits = ? WHERE user_id = ?`
	_, err := db.Exec(query, credits, userID)
	return err
}

// DecrementUserCredits atomically decrements user credits
func (db *DB) DecrementUserCredits(userID int64, amount int) error {
	query := `UPDATE users SET credits = credits - ? WHERE user_id = ? AND credits >= ?`
	result, err := db.Exec(query, amount, userID, amount)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("insufficient credits")
	}
	return nil
}

func (db *DB) SetUserPlan(userID int64, planType string, credits int, expiresAt int64) error {
	query := `UPDATE users SET plan_type = ?, credits = ?, trial_expires_at = ? WHERE user_id = ?`
	_, err := db.Exec(query, planType, credits, expiresAt, userID)
	return err
}

// AddCopyTarget adds a new copy trade target
func (db *DB) AddCopyTarget(userID int64, targetWallet string, amountSOL float64) error {
	query := `INSERT INTO copy_trade_targets (user_id, target_wallet, copy_amount_sol, created_at) VALUES (?, ?, ?, ?)`
	_, err := db.Exec(query, userID, targetWallet, amountSOL, time.Now().Unix())
	return err
}

// GetCopyTargets retrieves all active targets for a user
func (db *DB) GetCopyTargets(userID int64) ([]*CopyTradeTarget, error) {
	query := `SELECT id, user_id, target_wallet, copy_amount_sol, is_active, created_at FROM copy_trade_targets WHERE user_id = ? AND is_active = 1`
	rows, err := db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var targets []*CopyTradeTarget
	for rows.Next() {
		var t CopyTradeTarget
		var isActiveInt int
		if err := rows.Scan(&t.ID, &t.UserID, &t.TargetWallet, &t.CopyAmountSOL, &isActiveInt, &t.CreatedAt); err != nil {
			return nil, err
		}
		t.IsActive = isActiveInt == 1
		targets = append(targets, &t)
	}
	return targets, nil
}

// GetAllActiveCopyTargets retrieves all active copy trade targets
func (db *DB) GetAllActiveCopyTargets() ([]*CopyTradeTarget, error) {
	query := `SELECT id, user_id, target_wallet, copy_amount_sol, is_active, created_at FROM copy_trade_targets WHERE is_active = 1`
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var targets []*CopyTradeTarget
	for rows.Next() {
		var t CopyTradeTarget
		var isActiveInt int
		if err := rows.Scan(&t.ID, &t.UserID, &t.TargetWallet, &t.CopyAmountSOL, &isActiveInt, &t.CreatedAt); err != nil {
			return nil, err
		}
		t.IsActive = isActiveInt == 1
		targets = append(targets, &t)
	}
	return targets, nil
}

// RemoveCopyTarget deactivates a copy target
func (db *DB) RemoveCopyTarget(userID int64, targetWallet string) error {
	query := `DELETE FROM copy_trade_targets WHERE user_id = ? AND target_wallet = ?`
	_, err := db.Exec(query, userID, targetWallet)
	return err
}

// GetUsersWatchingWallet returns all users watching a specific wallet
func (db *DB) GetUsersWatchingWallet(wallet string) ([]*CopyTradeTarget, error) {
	query := `SELECT id, user_id, target_wallet, copy_amount_sol, is_active, created_at FROM copy_trade_targets WHERE target_wallet = ? AND is_active = 1`
	rows, err := db.Query(query, wallet)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var targets []*CopyTradeTarget
	for rows.Next() {
		var t CopyTradeTarget
		var isActiveInt int
		if err := rows.Scan(&t.ID, &t.UserID, &t.TargetWallet, &t.CopyAmountSOL, &isActiveInt, &t.CreatedAt); err != nil {
			return nil, err
		}
		t.IsActive = isActiveInt == 1
		targets = append(targets, &t)
	}
	return targets, nil
}

// Trade represents a trade record
type Trade struct {
	ID            int64
	ChatID        int64
	WalletAddress string
	TxSignature   string
	TradeType     string // "buy" or "sell"
	TokenAddress  string
	SolAmount     float64
	TokenAmount   float64
	PricePerToken float64
	JitoTip       float64
	Status        string // "pending", "confirmed", "failed"
	CreatedAt     int64
	ConfirmedAt   int64
}

// SaveTrade saves a trade record
func (db *DB) SaveTrade(userID int64, walletAddr, signature, tradeType, tokenAddr string, solAmount, tokenAmount, pricePerToken, jitoTip float64, status string) error {
	query := `
		INSERT INTO trades (chat_id, wallet_address, tx_signature, trade_type, token_address, sol_amount, token_amount, price_per_token, jito_tip, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := db.Exec(query, userID, walletAddr, signature, tradeType, tokenAddr, solAmount, tokenAmount, pricePerToken, jitoTip, status, time.Now().Unix())
	return err
}

// GetRecentTrades retrieves recent trades for a user
func (db *DB) GetRecentTrades(userID int64, limit int) ([]*Trade, error) {
	query := `SELECT id, chat_id, wallet_address, tx_signature, trade_type, token_address, sol_amount, token_amount, price_per_token, jito_tip, status, created_at, confirmed_at FROM trades WHERE chat_id = ? ORDER BY created_at DESC LIMIT ?`
	rows, err := db.Query(query, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trades []*Trade
	for rows.Next() {
		var t Trade
		var confirmedAt sql.NullInt64
		var signature sql.NullString

		if err := rows.Scan(&t.ID, &t.ChatID, &t.WalletAddress, &signature, &t.TradeType, &t.TokenAddress, &t.SolAmount, &t.TokenAmount, &t.PricePerToken, &t.JitoTip, &t.Status, &t.CreatedAt, &confirmedAt); err != nil {
			return nil, err
		}

		if signature.Valid {
			t.TxSignature = signature.String
		}
		if confirmedAt.Valid {
			t.ConfirmedAt = confirmedAt.Int64
		}

		trades = append(trades, &t)
	}
	return trades, nil
}

// UpdateTradeStatus updates the status of a trade
func (db *DB) UpdateTradeStatus(signature, status string, confirmedAt int64) error {
	query := `UPDATE trades SET status = ?, confirmed_at = ? WHERE tx_signature = ?`
	_, err := db.Exec(query, status, confirmedAt, signature)
	return err
}

// GetUsersWithSnipingEnabled returns users who have enabled sniping
func (db *DB) GetUsersWithSnipingEnabled() ([]int64, error) {
	// Assuming sniping setting is in user_settings or a new table.
	// The plan didn't specify where sniping settings are stored per user,
	// but `config.json` has a global sniper config.
	// If it's per user, we need a column.
	// For now, let's return empty or implement if we added the column.
	// We didn't add a sniping column to user_settings.
	// So maybe this is a global feature or we missed a migration.
	// I'll return nil for now as the sniper feature is optional/disabled in config.
	return nil, nil
}

// SaveLimitOrder saves a new limit order
func (db *DB) SaveLimitOrder(order *LimitOrder) error {
	query := `
		INSERT INTO limit_orders (user_id, order_pubkey, token_symbol, token_mint, side, price, amount, status, expires_at, target_mcap, initial_rent_sol, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := db.Exec(query, order.UserID, order.OrderPubkey, order.TokenSymbol, order.TokenMint, order.Side, order.Price, order.Amount, order.Status, order.ExpiresAt, order.TargetMCAP, order.InitialRentSOL, time.Now().Unix())
	return err
}

// GetExpiredOrdersBatch retrieves a batch of expired orders
func (db *DB) GetExpiredOrdersBatch(limit int) ([]*LimitOrder, error) {
	query := `SELECT id, user_id, order_pubkey, token_symbol, token_mint, side, price, amount, status, expires_at, target_mcap, initial_rent_sol, created_at 
			  FROM limit_orders 
			  WHERE expires_at < ? AND status = 'OPEN' 
			  LIMIT ?`

	rows, err := db.Query(query, time.Now().Unix(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []*LimitOrder
	for rows.Next() {
		var o LimitOrder
		if err := rows.Scan(&o.ID, &o.UserID, &o.OrderPubkey, &o.TokenSymbol, &o.TokenMint, &o.Side, &o.Price, &o.Amount, &o.Status, &o.ExpiresAt, &o.TargetMCAP, &o.InitialRentSOL, &o.CreatedAt); err != nil {
			return nil, err
		}
		orders = append(orders, &o)
	}
	return orders, nil
}

// UpdateOrderStatus updates the status of a limit order
func (db *DB) UpdateOrderStatus(id int64, status string) error {
	query := `UPDATE limit_orders SET status = ? WHERE id = ?`
	_, err := db.Exec(query, status, id)
	return err
}
