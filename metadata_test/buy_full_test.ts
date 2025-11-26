import { Connection, PublicKey, VersionedTransaction } from "@solana/web3.js";
import { Metaplex } from "@metaplex-foundation/js";
import { performance } from "perf_hooks";

const RPC_URL = "https://rpc.shyft.to?api_key=48KZbYxP-9e9SpqR";
const connection = new Connection(RPC_URL);
const metaplex = Metaplex.make(connection);

// Configuration
const SOL_MINT = "So11111111111111111111111111111111111111112";
const USDC_MINT = "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v";
const AMOUNT_LAMPS = 100000000; // 0.1 SOL
// Mock wallet for transaction building (must be valid base58)
const USER_WALLET = "So11111111111111111111111111111111111111112";

interface JupiterQuote {
    outAmount: string;
    error?: string;
    outputMint: string;
    inputMint: string;
    amount: string;
    swapMode: string;
    slippageBps: number;
    platformFee: any;
    priceImpactPct: string;
    routePlan: any[];
    contextSlot?: number;
    timeTaken?: number;
}

async function getTokenInfo(mintAddress: string) {
    const start = performance.now();
    try {
        const mint = new PublicKey(mintAddress);
        const metadata = await metaplex.nfts().findByMint({ mintAddress: mint });
        const end = performance.now();
        return {
            name: metadata.name,
            symbol: metadata.symbol,
            latency: end - start
        };
    } catch (e) {
        return { name: "Unknown", symbol: "???", latency: performance.now() - start };
    }
}

async function getQuote(inputMint: string, outputMint: string, amount: number): Promise<{ quote: JupiterQuote, latency: number }> {
    const start = performance.now();
    const url = `https://quote-api.jup.ag/v6/quote?inputMint=${inputMint}&outputMint=${outputMint}&amount=${amount}&slippageBps=50`;

    try {
        const response = await fetch(url);
        if (!response.ok) throw new Error("Network response not ok");
        const quote = await response.json() as JupiterQuote;
        return { quote, latency: performance.now() - start };
    } catch (e) {
        // Mocking for blocked environment
        console.log("   ⚠️  Jupiter API blocked/failed. Using MOCK quote.");
        await new Promise(r => setTimeout(r, 200)); // Simulate latency
        return {
            quote: {
                outAmount: "25000000",
                outputMint,
                inputMint,
                amount: amount.toString(),
                swapMode: "ExactIn",
                slippageBps: 50,
                platformFee: null,
                priceImpactPct: "0.01",
                routePlan: []
            },
            latency: performance.now() - start
        };
    }
}

async function getSwapTransaction(quote: JupiterQuote, userPublicKey: string): Promise<{ txSize: number, latency: number }> {
    const start = performance.now();
    const url = "https://quote-api.jup.ag/v6/swap";

    try {
        const response = await fetch(url, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                quoteResponse: quote,
                userPublicKey: userPublicKey,
                wrapAndUnwrapSol: true
            })
        });

        if (!response.ok) throw new Error("Network response not ok");
        const { swapTransaction } = await response.json() as any;

        // Deserialize to check size/validity
        const swapTransactionBuf = Buffer.from(swapTransaction, 'base64');
        const transaction = VersionedTransaction.deserialize(swapTransactionBuf);

        return { txSize: swapTransactionBuf.length, latency: performance.now() - start };
    } catch (e) {
        // Mocking for blocked environment
        console.log("   ⚠️  Jupiter Swap API blocked/failed. Using MOCK transaction build.");
        await new Promise(r => setTimeout(r, 150)); // Simulate latency
        return { txSize: 1234, latency: performance.now() - start };
    }
}

async function runFullSimulation() {
    console.log(`--- 🚀 Starting Full Buy Simulation ---`);
    console.log(`RPC: ${RPC_URL}\n`);

    const startTotal = performance.now();

    // 1. Get Token Info (Parallel)
    console.log("1. 🔍 Fetching Token Metadata...");
    const [solInfo, usdcInfo] = await Promise.all([
        getTokenInfo(SOL_MINT),
        getTokenInfo(USDC_MINT)
    ]);
    console.log(`   Input:  ${solInfo.name} (${solInfo.symbol}) - ${solInfo.latency.toFixed(2)}ms`);
    console.log(`   Output: ${usdcInfo.name} (${usdcInfo.symbol}) - ${usdcInfo.latency.toFixed(2)}ms`);

    // 2. Get Blockhash
    console.log("\n2. 🔗 Fetching Blockhash...");
    const startBh = performance.now();
    const { blockhash } = await connection.getLatestBlockhash();
    const endBh = performance.now();
    console.log(`   Blockhash: ${blockhash.slice(0, 10)}...`);
    console.log(`   Latency: ${(endBh - startBh).toFixed(2)}ms`);

    // 3. Get Quote
    console.log("\n3. 💱 Getting Jupiter Quote...");
    const { quote, latency: quoteLatency } = await getQuote(SOL_MINT, USDC_MINT, AMOUNT_LAMPS);
    console.log(`   Quote: ${AMOUNT_LAMPS / 1e9} SOL -> ${parseInt(quote.outAmount) / 1e6} USDC`);
    console.log(`   Latency: ${quoteLatency.toFixed(2)}ms`);

    // 4. Build Transaction
    console.log("\n4. 🏗️  Building Swap Transaction...");
    const { txSize, latency: txLatency } = await getSwapTransaction(quote, USER_WALLET);
    console.log(`   Tx Size: ${txSize} bytes`);
    console.log(`   Latency: ${txLatency.toFixed(2)}ms`);

    const endTotal = performance.now();

    console.log(`\n--- 📊 Performance Summary ---`);
    console.log(`Metadata Fetch:  ${Math.max(solInfo.latency, usdcInfo.latency).toFixed(2)} ms (Parallel)`);
    console.log(`RPC Blockhash:   ${(endBh - startBh).toFixed(2)} ms`);
    console.log(`Jupiter Quote:   ${quoteLatency.toFixed(2)} ms`);
    console.log(`Tx Build:        ${txLatency.toFixed(2)} ms`);
    console.log(`---------------------------------`);
    console.log(`TOTAL TIME:      ${(endTotal - startTotal).toFixed(2)} ms`);
}

runFullSimulation();
