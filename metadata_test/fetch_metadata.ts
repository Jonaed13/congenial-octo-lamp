import { Connection, PublicKey } from "@solana/web3.js";
import { Metaplex } from "@metaplex-foundation/js";
import { performance } from "perf_hooks";

const RPC_URL = "https://rpc.shyft.to?api_key=48KZbYxP-9e9SpqR";
const connection = new Connection(RPC_URL);

/**
 * Fetches and logs the token metadata for a given mint address.
 * @param mintAddress - The mint address of the token.
 */
async function getTokenMetadata(mintAddress: string) {
    try {
        console.log(`Connecting to RPC: ${RPC_URL}`);
        const startTotal = performance.now();

        const metaplex = Metaplex.make(connection);
        const mint = new PublicKey(mintAddress);

        console.log("Fetching metadata...");
        const startFetch = performance.now();

        const metadata = await metaplex.nfts().findByMint({ mintAddress: mint });

        const endFetch = performance.now();
        const endTotal = performance.now();

        console.log(`Token Name: ${metadata.name}`);
        console.log(`Token Symbol: ${metadata.symbol}`);

        console.log(`\n--- Performance Results ---`);
        console.log(`Metadata Fetch Time: ${(endFetch - startFetch).toFixed(2)} ms`);
        console.log(`Total Execution Time: ${(endTotal - startTotal).toFixed(2)} ms`);

    } catch (error) {
        console.error("Error fetching token metadata:", error);
    }
}

const sampleMintAddress = "So11111111111111111111111111111111111111112"; // Wrapped SOL
getTokenMetadata(sampleMintAddress);
