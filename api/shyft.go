package api

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// MetadataProgramID is the Metaplex Token Metadata Program ID
var MetadataProgramID = solana.MustPublicKeyFromBase58("metaqbxxUerdq28cj1RbAWkYQm3ybzjb6a8bt518x1s")

// ShyftMetadata holds the token metadata
type ShyftMetadata struct {
	Name        string
	Symbol      string
	URI         string
	TotalSupply string
}

// GetShyftMetadata fetches token metadata and supply using Shyft RPC
func GetShyftMetadata(rpcURL, mintAddress string) (*ShyftMetadata, error) {
	client := rpc.New(rpcURL)
	mint, err := solana.PublicKeyFromBase58(mintAddress)
	if err != nil {
		return nil, fmt.Errorf("invalid mint address: %w", err)
	}

	// 1. Fetch Supply
	supplyResult, err := client.GetTokenSupply(context.Background(), mint, rpc.CommitmentFinalized)
	totalSupply := "Unknown"
	if err == nil && supplyResult != nil && supplyResult.Value != nil {
		totalSupply = supplyResult.Value.UiAmountString
	} else {
		log.Printf("⚠️ Failed to fetch supply: %v", err)
	}

	// 2. Derive Metadata PDA
	addr, _, err := solana.FindProgramAddress(
		[][]byte{
			[]byte("metadata"),
			MetadataProgramID.Bytes(),
			mint.Bytes(),
		},
		MetadataProgramID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to derive metadata PDA: %w", err)
	}

	// 3. Fetch Account Info
	account, err := client.GetAccountInfo(context.Background(), addr)
	if err != nil {
		// Metadata might not exist for all tokens
		return &ShyftMetadata{
			Name:        "Unknown",
			Symbol:      "Unknown",
			TotalSupply: totalSupply,
		}, nil
	}

	// 4. Decode Metadata
	data := account.Value.Data.GetBinary()
	if len(data) == 0 {
		return &ShyftMetadata{
			Name:        "Unknown",
			Symbol:      "Unknown",
			TotalSupply: totalSupply,
		}, nil
	}

	// Skip Key (1) + UpdateAuth (32) + Mint (32) = 65 bytes
	offset := 65
	if len(data) < offset+4 {
		return nil, fmt.Errorf("metadata too short")
	}

	// Decode Name
	nameLen := int(uint32(data[offset]) | uint32(data[offset+1])<<8 | uint32(data[offset+2])<<16 | uint32(data[offset+3])<<24)
	offset += 4
	if len(data) < offset+nameLen {
		return nil, fmt.Errorf("metadata name too short")
	}
	name := string(data[offset : offset+nameLen])
	name = strings.Trim(name, "\x00")
	offset += nameLen

	// Decode Symbol
	if len(data) < offset+4 {
		return nil, fmt.Errorf("metadata symbol length missing")
	}
	symbolLen := int(uint32(data[offset]) | uint32(data[offset+1])<<8 | uint32(data[offset+2])<<16 | uint32(data[offset+3])<<24)
	offset += 4
	if len(data) < offset+symbolLen {
		return nil, fmt.Errorf("metadata symbol too short")
	}
	symbol := string(data[offset : offset+symbolLen])
	symbol = strings.Trim(symbol, "\x00")
	offset += symbolLen

	// Decode URI
	if len(data) < offset+4 {
		// URI might be missing or truncated, but we have name/symbol
		return &ShyftMetadata{
			Name:        name,
			Symbol:      symbol,
			TotalSupply: totalSupply,
		}, nil
	}
	uriLen := int(uint32(data[offset]) | uint32(data[offset+1])<<8 | uint32(data[offset+2])<<16 | uint32(data[offset+3])<<24)
	offset += 4
	uri := ""
	if len(data) >= offset+uriLen {
		uri = string(data[offset : offset+uriLen])
		uri = strings.Trim(uri, "\x00")
	}

	return &ShyftMetadata{
		Name:        name,
		Symbol:      symbol,
		URI:         uri,
		TotalSupply: totalSupply,
	}, nil
}

// ExtractAPIKey extracts API key from WebSocket URL
func ExtractAPIKey(wsURL string) string {
	parts := strings.Split(wsURL, "api_key=")
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}
