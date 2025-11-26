package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// Metaplex Token Metadata Program ID
var MetadataProgramID = solana.MustPublicKeyFromBase58("metaqbxxUerdq28cj1RbAWkYQm3ybzjb6a8bt518x1s")

func main() {
	// Shyft RPC URL
	rpcURL := "https://rpc.shyft.to?api_key=48KZbYxP-9e9SpqR"
	fmt.Printf("🔌 Connecting to Shyft RPC: %s\n", rpcURL)

	client := rpc.New(rpcURL)

	// Test Token: Wrapped SOL
	mintAddress := "So11111111111111111111111111111111111111112"
	mint := solana.MustPublicKeyFromBase58(mintAddress)
	fmt.Printf("🪙 Fetching Metadata for Mint: %s\n", mintAddress)

	// Derive Metadata PDA
	// Seeds: ["metadata", program_id, mint_key]
	addr, _, err := solana.FindProgramAddress(
		[][]byte{
			[]byte("metadata"),
			MetadataProgramID.Bytes(),
			mint.Bytes(),
		},
		MetadataProgramID,
	)
	if err != nil {
		log.Fatalf("❌ Failed to derive metadata PDA: %v", err)
	}
	fmt.Printf("📍 Metadata Account: %s\n", addr)

	// Fetch Account Info
	account, err := client.GetAccountInfo(context.Background(), addr)
	if err != nil {
		log.Fatalf("❌ Failed to fetch account info: %v", err)
	}

	// Decode Metadata (Manual decoding for simplicity without full Metaplex struct)
	// Layout:
	// key: u8
	// update_authority: Pubkey
	// mint: Pubkey
	// data: Data (name, symbol, uri, seller_fee_basis_points, creators)
	// ...

	data := account.Value.Data.GetBinary()
	if len(data) == 0 {
		log.Fatal("❌ No data found in account")
	}

	// Skip Key (1) + UpdateAuth (32) + Mint (32) = 65 bytes
	offset := 65

	// Decode Name
	nameLen := int(uint32(data[offset]) | uint32(data[offset+1])<<8 | uint32(data[offset+2])<<16 | uint32(data[offset+3])<<24)
	offset += 4
	name := string(data[offset : offset+nameLen])
	// Remove null bytes
	name = strings.Trim(name, "\x00")
	offset += nameLen

	// Decode Symbol
	symbolLen := int(uint32(data[offset]) | uint32(data[offset+1])<<8 | uint32(data[offset+2])<<16 | uint32(data[offset+3])<<24)
	offset += 4
	symbol := string(data[offset : offset+symbolLen])
	symbol = strings.Trim(symbol, "\x00")
	offset += symbolLen

	// Decode URI
	uriLen := int(uint32(data[offset]) | uint32(data[offset+1])<<8 | uint32(data[offset+2])<<16 | uint32(data[offset+3])<<24)
	offset += 4
	uri := string(data[offset : offset+uriLen])
	uri = strings.Trim(uri, "\x00")

	fmt.Println("\n✅ Metadata Fetched Successfully!")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("🏷️  Name:   %s\n", name)
	fmt.Printf("🔣 Symbol: %s\n", symbol)
	fmt.Printf("🔗 URI:    %s\n", uri)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}
