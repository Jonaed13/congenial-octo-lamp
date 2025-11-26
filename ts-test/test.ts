import { Connection, PublicKey } from "@solana/web3.js";
import { Metaplex } from "@metaplex-foundation/js";
import axios from "axios";

// Using the Shyft RPC URL provided by the user
const RPC_URL = "https://rpc.shyft.to?api_key=48KZbYxP-9e9SpqR";
const connection = new Connection(RPC_URL);

/**
 * Fetches and logs the token metadata for a given mint address.
 * @param mintAddress - The mint address of the token.
 */
async function getTokenMetadata(mintAddress: string) {
    console.log(`🔌 Connecting to RPC: ${RPC_URL}`);
    console.log(`🪙 Fetching metadata for mint: ${mintAddress}`);

    try {
        const metaplex = Metaplex.make(connection);
        const mint = new PublicKey(mintAddress);
        const metadata = await metaplex.nfts().findByMint({ mintAddress: mint });

        console.log("\n✅ Metadata Fetched Successfully!");
        console.log("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━");
        console.log(`Token Name:   ${metadata.name}`);
        console.log(`Token Symbol: ${metadata.symbol}`);

        // 1. Fetch Supply via RPC
        const supply = await connection.getTokenSupply(mint);
        console.log(`Total Supply: ${supply.value.uiAmountString}`);

        // 2. Fetch Price via Shyft REST API
        const priceUrl = `https://api.shyft.to/sol/v1/market/token_price?network=mainnet-beta&token_address=${mintAddress}`;
        try {
            const response = await axios.get(priceUrl, {
                headers: { 'x-api-key': '48KZbYxP-9e9SpqR' }
            });

            if (response.data.success) {
                console.log(`Price (USD):  $${response.data.result.price}`);
            }
        } catch (err) {
            console.log("Price:        (Could not fetch via Shyft API)");
        }

        console.log("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━");

    } catch (error) {
        console.error("❌ Error fetching token data:", error);
    }
}

const sampleMintAddress = "DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263"; // BONK Token
getTokenMetadata(sampleMintAddress);
