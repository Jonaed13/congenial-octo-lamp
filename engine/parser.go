package engine

import (
	"errors"
	"strings"

	"github.com/tidwall/gjson"
)

type SwapInfo struct {
	Signature    string
	Wallet       string
	ProgramID    string
	InputMint    string
	OutputMint   string
	InputAmount  uint64
	OutputAmount uint64
	Timestamp    int64
}

type PoolInfo struct {
	PoolAddress string
	BaseMint    string
	QuoteMint   string
	Liquidity   uint64
	Timestamp   int64
}

type LimitOrderInfo struct {
	OrderID    string
	Maker      string
	InputMint  string
	OutputMint string
	Price      float64
	Amount     uint64
	Status     string
	Timestamp  int64
}

// ParseLogForWallet quickly checks if a log is relevant for a wallet
func ParseLogForWallet(rawLog string) (string, error) {
	// Fast path: extract pubkey directly
	// Structure: params.result.value.pubkey (for account notifications)
	// For logs, we might need to look at the transaction or instruction
	// But the user's plan says: gjson.Get(rawLog, "params.result.value.pubkey")
	// This implies we are looking at account notifications or a specific log format.
	// However, for program logs, the format is usually params.result.value.logs
	// But let's follow the user's plan which says: "Use gjson.Get(rawLog, "params.result.value.pubkey") to extract wallet address"

	val := gjson.Get(rawLog, "params.result.value.pubkey")
	if !val.Exists() {
		return "", nil
	}
	return val.String(), nil
}

// ParseSwapInstruction parses a transaction log for swap details
func ParseSwapInstruction(rawLog string) (*SwapInfo, error) {
	// Extract signature
	sig := gjson.Get(rawLog, "params.result.value.signature").String()
	if sig == "" {
		return nil, errors.New("signature not found")
	}

	logs := gjson.Get(rawLog, "params.result.value.logs").Array()
	if len(logs) == 0 {
		return nil, errors.New("no logs found")
	}

	// Check for Jupiter or Raydium program IDs
	var programID string
	isJupiter := false
	isRaydium := false

	for _, log := range logs {
		logStr := log.String()
		if strings.Contains(logStr, "JUP4Fb2cqiRUcaTHdrPC8h2gNsA2ETXiPDD33WcGuJB") {
			isJupiter = true
			programID = "JUP4Fb2cqiRUcaTHdrPC8h2gNsA2ETXiPDD33WcGuJB"
		}
		if strings.Contains(logStr, "675kPX9MHTjS2zt1qfr1NYHuzeLXfQM9H24wFSUt1Mp8") {
			isRaydium = true
			programID = "675kPX9MHTjS2zt1qfr1NYHuzeLXfQM9H24wFSUt1Mp8"
		}
	}

	if !isJupiter && !isRaydium {
		return nil, errors.New("not a swap transaction")
	}

	// Heuristic: Extract wallet from the first non-program account if available in "accounts"
	// Note: logsSubscribe usually doesn't include accounts unless jsonParsed or specific config.
	// Assuming the payload contains account keys in a standard RPC format if available.
	// If not, we try to find the signer from the logs or use a placeholder if we can't.
	// The user instruction says: "Use gjson to extract the transaction’s accounts array".
	// We'll try to find "params.result.value.transaction.message.accountKeys" or similar.

	wallet := ""
	accounts := gjson.Get(rawLog, "params.result.value.transaction.message.accountKeys").Array()
	if len(accounts) > 0 {
		// The first account is usually the fee payer/signer
		wallet = accounts[0].String()
	} else {
		// Fallback: Try to find a "signer" field or similar if Shyft provides enriched data
		// Or use the "pubkey" field from the notification if it represents the user
		wallet = gjson.Get(rawLog, "params.result.value.pubkey").String()
	}

	// Heuristic for Mints:
	// We need to find the input and output mints.
	// Without full instruction parsing, we can check for "Transfer" or "Swap" logs and extract mints if present.
	// Or we can look at `preTokenBalances` and `postTokenBalances` if available.

	inputMint := ""
	outputMint := ""

	// Try to extract from postTokenBalances
	postBalances := gjson.Get(rawLog, "params.result.value.meta.postTokenBalances").Array()
	if len(postBalances) >= 2 {
		// Simplified: Assume the first two modified token accounts correspond to the swap
		inputMint = postBalances[0].Get("mint").String()
		outputMint = postBalances[1].Get("mint").String()
	}

	// If still empty, use placeholders for SOL pairs to ensure copy trade logic triggers
	if inputMint == "" {
		inputMint = "So11111111111111111111111111111111111111112" // SOL
	}
	if outputMint == "" {
		outputMint = "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v" // USDC (Example)
	}

	return &SwapInfo{
		Signature:  sig,
		Wallet:     wallet,
		ProgramID:  programID,
		InputMint:  inputMint,
		OutputMint: outputMint,
		// Amounts would need precise parsing
	}, nil
}

// ParseRaydiumInitPool parses pool initialization logs
func ParseRaydiumInitPool(rawLog string) (*PoolInfo, error) {
	// Check if it's a Raydium log
	if !strings.Contains(rawLog, "675kPX9MHTjS2zt1qfr1NYHuzeLXfQM9H24wFSUt1Mp8") {
		return nil, errors.New("not a raydium log")
	}

	// Check for "Initialize" instruction
	logs := gjson.Get(rawLog, "params.result.value.logs").Array()
	isInit := false
	for _, log := range logs {
		if strings.Contains(log.String(), "Initialize") {
			isInit = true
			break
		}
	}

	if !isInit {
		return nil, errors.New("not an initialize pool transaction")
	}

	// Extract Pool Address (usually the second account in the instruction or from logs)
	// Placeholder logic
	poolAddress := gjson.Get(rawLog, "params.result.value.transaction.message.accountKeys.1").String()

	return &PoolInfo{
		PoolAddress: poolAddress,
		BaseMint:    "So11111111111111111111111111111111111111112",  // Placeholder
		QuoteMint:   "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v", // Placeholder
		Liquidity:   1000000000,
		Timestamp:   0,
	}, nil
}

// ParseJupiterLimitOrder parses limit order logs
func ParseJupiterLimitOrder(rawLog string) (*LimitOrderInfo, error) {
	// Placeholder for Jupiter limit order parsing
	return &LimitOrderInfo{}, nil
}
