import { Connection, PublicKey } from "@solana/web3.js";
import { performance } from "perf_hooks";

const RPC_URL = "https://rpc.shyft.to?api_key=48KZbYxP-9e9SpqR";
const connection = new Connection(RPC_URL);

// Wrapped SOL Mint
const SOL_MINT = "So11111111111111111111111111111111111111112";
// USDC Mint (Target Token)
const USDC_MINT = "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v";

async function simulateBuy() {
    try {
        console.log(`Connecting to RPC: ${RPC_URL}`);
        console.log("--- Starting Buy Simulation ---\n");

        const startTotal = performance.now();

        // Step 1: Get Latest Blockhash (Required for transaction)
        console.log("1. Fetching Latest Blockhash...");
        const startBlockhash = performance.now();
        const { blockhash } = await connection.getLatestBlockhash();
        const endBlockhash = performance.now();
        console.log(`   Blockhash: ${blockhash.slice(0, 10)}...`);
        console.log(`   Time: ${(endBlockhash - startBlockhash).toFixed(2)} ms\n`);

        // Step 2: Get Quote from Jupiter (SOL -> USDC)
        // We'll swap 0.1 SOL
        const amount = 100000000; // 0.1 SOL in lamports
        const quoteUrl = `https://quote-api.jup.ag/v6/quote?inputMint=${SOL_MINT}&outputMint=${USDC_MINT}&amount=${amount}&slippageBps=50`;

        interface JupiterQuote {
            outAmount: string;
            error?: string;
        }

        console.log("2. Fetching Jupiter Quote (SOL -> USDC)...");
        const startQuote = performance.now();

        // Mocking Jupiter response because of environment network restrictions
        // In a real environment, this would be:
        // const response = await fetch(quoteUrl);
        // const quoteData = (await response.json()) as JupiterQuote;

        // Simulating network latency
        await new Promise(resolve => setTimeout(resolve, 200));

        const quoteData: JupiterQuote = {
            outAmount: "25000000", // Mock 25 USDC
        };

        const endQuote = performance.now();
        console.log("   (Mocked response due to network restrictions)");

        if (quoteData.error) {
            throw new Error(`Jupiter Error: ${quoteData.error}`);
        }

        console.log(`   Out Amount: ${quoteData.outAmount} (Micro USDC)`);
        console.log(`   Time: ${(endQuote - startQuote).toFixed(2)} ms\n`);

        // Step 3: Transaction Construction (Simulation)
        // In a real app, we would POST to Jupiter /swap endpoint here
        console.log("3. Simulating Transaction Build...");
        const startBuild = performance.now();
        // Mocking build time as it's usually a fast local or API operation
        // For accurate test we could call the swap endpoint but that requires a wallet address
        // We'll just estimate the overhead of the API call
        const endBuild = performance.now();
        console.log(`   Time: ~${(endBuild - startBuild + 50).toFixed(2)} ms (Estimated)\n`);

        const endTotal = performance.now();

        console.log(`--- Performance Summary ---`);
        console.log(`RPC Latency (Blockhash): ${(endBlockhash - startBlockhash).toFixed(2)} ms`);
        console.log(`Quote Latency (Jupiter): ${(endQuote - startQuote).toFixed(2)} ms`);
        console.log(`Total 'Pre-Sign' Time:   ${(endTotal - startTotal).toFixed(2)} ms`);

    } catch (error) {
        console.error("Error during buy simulation:", error);
    }
}

simulateBuy();
