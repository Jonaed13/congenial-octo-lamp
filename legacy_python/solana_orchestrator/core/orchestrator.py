#!/usr/bin/env python3
"""
🚀 Solana Token and Wallet Analysis Orchestrator 🚀
===================================================

A beautiful, intelligent orchestrator that:
1. 🪙 Fetches tokens from Birdeye or Moralis
2. 👥 Gets top holders for each token
3. 💾 Saves token and holder data
4. 🎯 Analyzes wallets using Playwright Multi-Page Scanner
5. 💎 Filters profitable wallets based on your criteria

Features:
- Beautiful color-coded UI
- Playwright multi-page concurrent scanning
- Birdeye & Moralis token sources
- Customizable filters and page count
- Auto-loop mode
- Resume capability

Author: AI Assistant
Date: 2025-10-03
Version: 2.1 - Complete Edition
"""

import json
import requests
import os
import time
import random
from typing import List, Dict, Optional
import logging
from datetime import datetime
import argparse
import sys
import asyncio
from playwright_multi_page_analyzer import create_playwright_analyzer

# Colors for beautiful terminal output
class Colors:
    HEADER = '\033[95m'
    BLUE = '\033[94m'
    CYAN = '\033[96m'
    GREEN = '\033[92m'
    YELLOW = '\033[93m'
    RED = '\033[91m'
    ENDC = '\033[0m'
    BOLD = '\033[1m'
    UNDERLINE = '\033[4m'

# Set up logging
log_file = os.path.join(os.path.dirname(__file__), '..', 'logs', 'orchestrator.log')
os.makedirs(os.path.dirname(log_file), exist_ok=True)
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s',
    handlers=[
        logging.FileHandler(log_file),
        logging.StreamHandler()
    ]
)
logger = logging.getLogger(__name__)

class SolanaTokenOrchestrator:
    def __init__(self, token_limit=None, num_pages=3, min_winrate=None, min_pnl=None):
        # Load configuration from config.json
        config_path = os.path.join(os.path.dirname(__file__), '..', 'config', 'config.json')
        with open(config_path, 'r') as f:
            config = json.load(f)

        self.moralis_api_key = config.get("moralis_api_key")
        if not self.moralis_api_key:
            raise ValueError("Moralis API key not found in config.json")
        
        # API endpoints
        self.birdeye_api_key = config.get("birdeye_api_key")
        self.pumpfun_graduated_tokens_url = "https://solana-gateway.moralis.io/token/mainnet/exchange/pumpfun/graduated"
        self.birdeye_tokenlist_url = "https://public-api.birdeye.so/defi/tokenlist"
        self.birdeye_top_traders_url = "https://public-api.birdeye.so/defi/v2/tokens/top_traders"
        self.token_holders_url_template = "https://solana-gateway.moralis.io/token/mainnet/{token_address}/top-holders"
        
        # Output files - use data directory
        data_dir = os.path.join(os.path.dirname(__file__), '..', 'data')
        self.tokens_file = os.path.join(data_dir, "tokens.json")
        self.holders_file = os.path.join(data_dir, "holders.json")
        self.tokens_txt = os.path.join(data_dir, "tokens.txt")
        self.holders_txt = os.path.join(data_dir, "holders.txt")
        self.good_wallets_file = os.path.join(data_dir, "good_wallets.json")
        self.good_wallets_txt = os.path.join(data_dir, "good_wallets.txt")
        self.owner_addresses_file = os.path.join(data_dir, "owner_addresses.txt")
        self.processed_wallets_file = os.path.join(data_dir, "processed_wallets.txt")
        
        # Configuration - allow override via parameters
        config_winrate = config.get("analysis_filters", {}).get("min_winrate", 70.0)
        config_pnl = config.get("analysis_filters", {}).get("min_realized_pnl", 100.0)
        
        self.min_winrate = min_winrate if min_winrate is not None else config_winrate
        self.min_realized_pnl = min_pnl if min_pnl is not None else config_pnl
        self.num_pages = num_pages
        self.max_retries = config.get("api_settings", {}).get("max_retries", 3)
        
        # Use provided token_limit if available, otherwise use config value with default of 100
        config_token_limit = config.get("api_settings", {}).get("token_limit", 100)
        self.token_limit = token_limit if token_limit is not None else config_token_limit
        
    def fetch_graduated_tokens(self) -> List[Dict]:
        """Fetch graduated PumpFun tokens from Moralis API"""
        logger.info("Fetching graduated PumpFun tokens...")
        
        headers = {"accept": "application/json", "X-API-Key": self.moralis_api_key}
        params = {"limit": self.token_limit}
        
        for attempt in range(self.max_retries):
            try:
                response = requests.get(self.pumpfun_graduated_tokens_url, headers=headers, params=params)
                response.raise_for_status()
                
                data = response.json()
                if 'result' not in data:
                    logger.error("No 'result' field in response")
                    return []
                
                tokens = data['result']
                logger.info(f"Fetched {len(tokens)} graduated tokens")
                return tokens
                
            except requests.exceptions.RequestException as e:
                logger.error(f"Attempt {attempt+1} failed: {e}")
                if attempt < self.max_retries - 1:
                    time.sleep(2 ** attempt)  # Exponential backoff
                else:
                    logger.error("All attempts failed")
                    return []
            except json.JSONDecodeError:
                logger.error("Failed to decode JSON response")
                return []
                
    def fetch_birdeye_tokens(self) -> List[Dict]:
        """Fetch tokens from Birdeye API based on liquidity."""
        logger.info("Fetching tokens from Birdeye...")
        if not self.birdeye_api_key:
            logger.error("Birdeye API key not found in config.json")
            return []

        headers = {
            "X-API-KEY": self.birdeye_api_key,
            "accept": "application/json",
            "x-chain": "solana"
        }
        params = {
            "sort_by": "liquidity",
            "sort_type": "desc",
            "offset": 0,
            "limit": self.token_limit,
            "min_liquidity": 100000,
            "max_liquidity": 500000,
            "ui_amount_mode": "scaled"
        }

        try:
            response = requests.get(self.birdeye_tokenlist_url, headers=headers, params=params)
            response.raise_for_status()
            result = response.json()
            
            # Birdeye API returns data in format: {"success": true, "data": {"tokens": [...]}}
            if result.get("success") and "data" in result and "tokens" in result["data"]:
                tokens = result["data"]["tokens"]
                # Adapt Birdeye response to match Moralis format
                adapted_tokens = []
                for token in tokens:
                    if "address" in token:
                        adapted_tokens.append({"tokenAddress": token["address"]})
                logger.info(f"Fetched {len(adapted_tokens)} tokens from Birdeye.")
                return adapted_tokens
            else:
                logger.warning("No tokens found in Birdeye response.")
                return []
        except requests.exceptions.RequestException as e:
            logger.error(f"Failed to fetch tokens from Birdeye: {e}")
            return []
        except Exception as e:
            logger.error(f"An unexpected error occurred while fetching from Birdeye: {e}")
            return []
    
    def fetch_top_traders(self, token_address: str) -> Optional[List[Dict]]:
        """
        Fetch top traders for a token from Birdeye API.
        Returns list of top traders with their wallet addresses and trade info.
        """
        logger.debug(f"Fetching top traders for {token_address} from Birdeye...")
        
        if not self.birdeye_api_key:
            logger.error("Birdeye API key not found - cannot fetch top traders")
            return None
        
        headers = {
            "X-API-KEY": self.birdeye_api_key,
            "accept": "application/json",
            "x-chain": "solana"
        }
        params = {
            "address": token_address,
            "time_frame": "24h",
            "sort_by": "volume",
            "sort_type": "desc",
            "offset": 0,
            "limit": 100,
            "ui_amount_mode": "scaled"
        }
        
        for attempt in range(self.max_retries):
            wait_time = (2 ** attempt) + random.uniform(0, 1)
            try:
                start_time = time.time()
                response = requests.get(self.birdeye_top_traders_url, headers=headers, params=params, timeout=30)
                end_time = time.time()
                
                logger.debug(f"Top traders API call for {token_address} took {end_time - start_time:.2f} seconds.")
                
                if response.status_code == 429:
                    logger.warning(f"Rate limit hit for top traders {token_address}. Waiting {wait_time:.2f}s...")
                    time.sleep(wait_time)
                    continue
                
                response.raise_for_status()
                data = response.json()
                
                # Birdeye format: {"success": true, "data": {"items": [...]}}
                if data.get("success") and "data" in data and "items" in data["data"]:
                    traders = data["data"]["items"]
                    processed_traders = []
                    seen_traders = set()
                    
                    for trader in traders:
                        # Extract owner (wallet address) from the trader data
                        owner = trader.get('owner')
                        if owner and owner not in seen_traders:
                            processed_traders.append({
                                "owner": owner,
                                "volume": trader.get('volume'),
                                "trade": trader.get('trade'),
                                "tradeBuy": trader.get('tradeBuy'),
                                "tradeSell": trader.get('tradeSell'),
                                "volumeBuy": trader.get('volumeBuy'),
                                "volumeSell": trader.get('volumeSell')
                            })
                            seen_traders.add(owner)
                    
                    logger.debug(f"Retrieved {len(processed_traders)} top traders for {token_address}")
                    return processed_traders
                else:
                    logger.warning(f"No traders data in Birdeye response for {token_address}")
                    return []
                    
            except requests.exceptions.Timeout:
                logger.warning(f"Timeout fetching top traders for {token_address} (attempt {attempt+1}/{self.max_retries})")
                if attempt < self.max_retries - 1:
                    time.sleep(wait_time)
                else:
                    logger.error(f"All timeout attempts failed for top traders {token_address}")
                    return None
            except requests.exceptions.RequestException as e:
                logger.error(f"Failed to fetch top traders for {token_address} (attempt {attempt+1}): {e}")
                if attempt < self.max_retries - 1:
                    time.sleep(wait_time)
                else:
                    return None
            except json.JSONDecodeError:
                logger.error(f"Failed to decode JSON for top traders {token_address}")
                return None
        
        return None
    
    def get_token_holders(self, token_address: str) -> Optional[List[Dict]]:
        """
        Fetch top 100 holders for a single token with timeouts, retries, and
        exponential backoff.
        """
        logger.debug(f"Fetching holders for {token_address}...")
        
        headers = {
            "accept": "application/json", 
            "X-API-Key": self.moralis_api_key
        }
        params = {"limit": 100}
        url = self.token_holders_url_template.format(token_address=token_address)
        
        for attempt in range(self.max_retries + 1):
            wait_time = (2 ** attempt) + random.uniform(0, 1)
            try:
                start_time = time.time()
                response = requests.get(url, headers=headers, params=params, timeout=30)
                end_time = time.time()
                
                logger.debug(f"API call for {token_address} took {end_time - start_time:.2f} seconds.")

                if response.status_code == 429:
                    logger.warning(f"Rate limit hit for {token_address}. Waiting {wait_time:.2f}s before retrying...")
                    time.sleep(wait_time)
                    continue

                response.raise_for_status()
                
                data = response.json()
                if 'result' not in data:
                    logger.error(f"No 'result' field in response for {token_address}")
                    return None
                
                holders_data = data['result']
                processed_holders = []
                seen_holders = set()

                for holder in holders_data:
                    owner_address = holder.get('ownerAddress')
                    if owner_address and owner_address not in seen_holders:
                        processed_holders.append({
                            "ownerAddress": owner_address,
                            "balance": holder.get('balance'),
                            "balanceFormatted": holder.get('balanceFormatted'),
                            "usdValue": holder.get('usdValue'),
                            "percentageRelativeToTotalSupply": holder.get('percentageRelativeToTotalSupply')
                        })
                        seen_holders.add(owner_address)
                
                logger.debug(f"Retrieved {len(processed_holders)} unique holders for {token_address}")
                return processed_holders
            
            except requests.exceptions.Timeout:
                logger.warning(f"Timeout fetching holders for {token_address} on attempt {attempt+1}/{self.max_retries}.")
                if attempt < self.max_retries:
                    logger.info(f"Waiting {wait_time:.2f}s before retrying...")
                    time.sleep(wait_time)
                else:
                    logger.error(f"All retry attempts timed out for {token_address}. Skipping.")
                    return None
            except requests.exceptions.RequestException as e:
                logger.error(f"Failed to get holders for {token_address} (attempt {attempt+1}): {e}")
                if attempt < self.max_retries:
                    time.sleep(wait_time)
                else:
                    logger.error(f"All attempts failed for {token_address}. Skipping.")
                    return None
            except json.JSONDecodeError:
                logger.error(f"Failed to decode JSON for {token_address}. Skipping.")
                return None
        
        return None
    
    def save_tokens(self, tokens: List[Dict]):
        """Save tokens to JSON and TXT files"""
        logger.info(f"Saving {len(tokens)} tokens to files...")
        
        # Save to JSON
        with open(self.tokens_file, 'w') as f:
            json.dump(tokens, f, indent=2)
        
        # Extract just token addresses and save to TXT
        token_addresses = [token.get('tokenAddress', '') for token in tokens if token.get('tokenAddress')]
        with open(self.tokens_txt, 'w') as f:
            for address in token_addresses:
                f.write(f"{address}\n")
        
        logger.info(f"Saved {len(token_addresses)} token addresses to {self.tokens_txt}")
    
    def save_holders(self, holders_data: Dict[str, List[Dict]]):
        """Save holders to JSON and TXT files"""
        logger.debug(f"Saving holders data for {len(holders_data)} tokens...")
        
        # Save full holders data to JSON
        with open(self.holders_file, 'w') as f:
            json.dump(holders_data, f, indent=2)
        
        # Extract all unique holder addresses and save to TXT
        all_holder_addresses = set()
        for token_holders in holders_data.values():
            for holder in token_holders:
                if 'ownerAddress' in holder and holder['ownerAddress']:
                    all_holder_addresses.add(holder['ownerAddress'])
        
        with open(self.holders_txt, 'w') as f:
            for address in sorted(all_holder_addresses):
                f.write(f"{address}\n")
        
        # Also save to owner_addresses.txt for compatibility
        with open(self.owner_addresses_file, 'w') as f:
            for address in sorted(all_holder_addresses):
                f.write(f"{address}\n")
        
        logger.debug(f"Saved {len(all_holder_addresses)} unique holder addresses to {self.holders_txt} and {self.owner_addresses_file}")
    
    def load_json_file(self, file_path: str, is_dict: bool = False) -> List or Dict:
        """Load JSON file with error handling"""
        if os.path.exists(file_path):
            try:
                with open(file_path, 'r') as f:
                    return json.load(f)
            except json.JSONDecodeError:
                logger.warning(f"Could not decode JSON from {file_path}. Starting fresh.")
                return {} if is_dict else []
        return {} if is_dict else []
    
    def load_text_file(self, file_path: str) -> List[str]:
        """Load text file and return list of lines (stripped)"""
        if os.path.exists(file_path):
            try:
                with open(file_path, 'r') as f:
                    return [line.strip() for line in f if line.strip()]
            except Exception as e:
                logger.warning(f"Could not read {file_path}: {e}")
                return []
        return []
    
    def _copy_analysis_results(self):
        """Copy analysis results from results/ to data/ directory"""
        import shutil
        
        # Files to copy from results/ to data/
        files_to_copy = [
            ("results/good_wallets.json", self.good_wallets_file),
            ("results/good_wallets.txt", self.good_wallets_txt)
        ]
        
        copied_count = 0
        for src_file, dest_file in files_to_copy:
            if os.path.exists(src_file):
                try:
                    shutil.copy2(src_file, dest_file)
                    logger.info(f"Copied {src_file} -> {dest_file}")
                    copied_count += 1
                except Exception as e:
                    logger.warning(f"Failed to copy {src_file}: {e}")
        
        if copied_count > 0:
            logger.info(f"✅ Copied {copied_count} result files to data directory")
        
        # Also create TXT version from JSON if needed
        if os.path.exists(self.good_wallets_file):
            try:
                with open(self.good_wallets_file, 'r') as f:
                    good_wallets = json.load(f)
                
                if good_wallets:
                    with open(self.good_wallets_txt, 'w') as f:
                        for wallet in good_wallets:
                            f.write(f"{wallet.get('wallet', 'unknown')}\n")
                    logger.info(f"Created {self.good_wallets_txt} with {len(good_wallets)} wallet addresses")
            except Exception as e:
                logger.warning(f"Failed to create TXT version: {e}")
    
    def run_dexcheck_analysis(self):
        """
        Runs the wallet analysis using the Playwright multi-page analyzer.
        """
        logger.info("🔍 Starting DexCheck analysis with Playwright...")

        wallets = self.load_text_file(self.owner_addresses_file)
        if not wallets:
            logger.warning("No wallets to analyze (owner_addresses.txt is empty).")
            return

        logger.info(f"Starting Playwright analysis with {self.num_pages} pages on {len(wallets)} wallets...")

        # Progress tracking
        progress_stats = {'scanned': 0, 'passed': 0, 'failed': 0}
        
        def progress_callback(update):
            """Live updating progress display."""
            status = update.get('status', '')
            
            if status == 'passed':
                progress_stats['scanned'] += 1
                progress_stats['passed'] += 1
            elif status == 'failed':
                progress_stats['scanned'] += 1
                progress_stats['failed'] += 1
            
            # Update in place with \r (carriage return)
            scanned = progress_stats['scanned']
            passed = progress_stats['passed']
            failed = progress_stats['failed']
            percent = (scanned / len(wallets)) * 100 if len(wallets) > 0 else 0
            
            # Create progress bar
            bar_width = 30
            filled = int(bar_width * scanned / len(wallets)) if len(wallets) > 0 else 0
            bar = '█' * filled + '░' * (bar_width - filled)
            
            print(f"\r{Colors.CYAN}Progress: [{bar}] {percent:.1f}% | Scanned: {scanned}/{len(wallets)} | {Colors.GREEN}✅ Passed: {passed}{Colors.ENDC} | {Colors.RED}❌ Failed: {failed}{Colors.ENDC}", end='', flush=True)

        try:
            # Set environment variables for the analyzer
            os.environ["MIN_WINRATE"] = str(self.min_winrate)
            os.environ["MIN_REALIZED_PNL_USD"] = str(self.min_realized_pnl)
            
            # Ensure results directory exists
            os.makedirs("results", exist_ok=True)

            analyzer = create_playwright_analyzer(
                num_pages=self.num_pages, 
                wallets=wallets, 
                preset=None  # Using manual env vars instead
            )

            # Run the asynchronous analyzer
            results = asyncio.run(analyzer.run(progress_callback=progress_callback))

            # Print newline after progress bar
            print()
            
            if results.get('success'):
                total_scanned = results.get('total_scanned', 0)
                total_passed = results.get('total_passed', 0)
                logger.info("✅ Playwright analysis complete.")
                logger.info(f"📊 Summary: Scanned={total_scanned}, Passed={total_passed}")
                
                # Copy results from results/ to data/ directory
                self._copy_analysis_results()
                
            else:
                error_msg = results.get('error', 'Unknown error')
                logger.error(f"❌ Playwright analysis failed: {error_msg}")

        except Exception as e:
            logger.error(f"An unexpected error occurred during Playwright analysis: {e}")
            import traceback
            logger.error(traceback.format_exc())
            
    def run_complete_workflow(self, resume_from: int = 0, token_source: str = 'birdeye', fetch_traders: bool = False):
        """Run the complete workflow: tokens → holders → (optionally traders) → analysis, with resume capability."""
        logger.info("Starting complete Solana token and wallet analysis workflow...")
        
        # Step 1: Fetch or load tokens
        print("PROGRESS: Starting token collection", flush=True)
        if os.path.exists(self.tokens_file) and resume_from > 0:
            logger.info(f"Loading existing tokens from {self.tokens_file}")
            tokens = self.load_json_file(self.tokens_file)
            print(f"PROGRESS: Loaded {len(tokens)} existing tokens", flush=True)
        elif token_source == 'birdeye':
            tokens = self.fetch_birdeye_tokens()
        else:  # Default to moralis
            tokens = self.fetch_graduated_tokens()
            
        if not tokens:
            logger.error("Failed to fetch tokens. Exiting workflow.")
            return
            
        self.save_tokens(tokens)
        print(f"PROGRESS: Fetched and saved {len(tokens)} tokens", flush=True)

        # Step 2: Get holders for each token
        holders_data = self.load_json_file(self.holders_file, is_dict=True)
        
        start_index = 0
        if resume_from > 0:
            start_index = resume_from - 1
            logger.info(f"Resuming from token {resume_from}/{len(tokens)}...")
            print(f"{Colors.YELLOW}📍 Resuming from token #{resume_from}...{Colors.ENDC}\n")

        print(f"\n{Colors.BOLD}{Colors.CYAN}Collecting token holders...{Colors.ENDC}\n")
        
        for i in range(start_index, len(tokens)):
            token = tokens[i]
            token_address = token.get('tokenAddress')
            
            if not token_address:
                logger.warning(f"Skipping token {i+1} due to missing address.")
                continue

            # Skip if already processed
            if token_address in holders_data:
                # Show compact skip message
                print(f"\r{Colors.CYAN}Token {i+1}/{len(tokens)}: {token_address[:8]}...{token_address[-8:]} {Colors.YELLOW}(skipped - already processed){Colors.ENDC}", end='', flush=True)
                continue

            # Show current token being processed
            print(f"\r{Colors.CYAN}Token {i+1}/{len(tokens)}: {token_address[:8]}...{token_address[-8:]} {Colors.YELLOW}Fetching holders...{Colors.ENDC}", end='', flush=True)
            
            logger.debug(f"Processing token {i+1}/{len(tokens)}: {token_address}")
            holders = self.get_token_holders(token_address)
            
            if holders is not None:
                holders_data[token_address] = holders
                # Save progress incrementally
                self.save_holders(holders_data)
                # Show success
                print(f"\r{Colors.CYAN}Token {i+1}/{len(tokens)}: {token_address[:8]}...{token_address[-8:]} {Colors.GREEN}✅ {len(holders)} holders | Total wallets: {len(set(h['ownerAddress'] for t in holders_data.values() for h in t))}{Colors.ENDC}")
            else:
                print(f"\r{Colors.CYAN}Token {i+1}/{len(tokens)}: {token_address[:8]}...{token_address[-8:]} {Colors.RED}❌ Failed{Colors.ENDC}")
                logger.error(f"Failed to get holders for {token_address} after all retries.")

            # Add delay between token holder requests
            if i < len(tokens) - 1:
                time.sleep(random.uniform(1.5, 3.0))
        
        # Step 3: Final save of all holders
        self.save_holders(holders_data)
        print(f"\n\n{Colors.GREEN}✅ Collected holders from {len(holders_data)} tokens ({len(set(h['ownerAddress'] for t in holders_data.values() for h in t))} unique wallets){Colors.ENDC}")
        
        # Step 3.5: Fetch top traders if enabled
        if fetch_traders:
            print(f"\n{Colors.BOLD}{Colors.CYAN}Collecting top traders from Birdeye...{Colors.ENDC}\n")
            
            # Collect all trader wallets from all tokens
            all_trader_wallets = set()
            traders_fetched = 0
            
            for i, token in enumerate(tokens):
                token_address = token.get('tokenAddress')
                if not token_address:
                    continue
                
                print(f"\r{Colors.CYAN}Fetching traders {i+1}/{len(tokens)}: {token_address[:8]}...{token_address[-8:]} {Colors.YELLOW}Please wait...{Colors.ENDC}", end='', flush=True)
                
                traders = self.fetch_top_traders(token_address)
                
                if traders:
                    traders_fetched += 1
                    for trader in traders:
                        owner = trader.get('owner')
                        if owner:
                            all_trader_wallets.add(owner)
                    print(f"\r{Colors.CYAN}Fetched traders {i+1}/{len(tokens)}: {token_address[:8]}...{token_address[-8:]} {Colors.GREEN}✅ {len(traders)} traders | Total trader wallets: {len(all_trader_wallets)}{Colors.ENDC}")
                else:
                    print(f"\r{Colors.CYAN}Fetched traders {i+1}/{len(tokens)}: {token_address[:8]}...{token_address[-8:]} {Colors.RED}❌ No data{Colors.ENDC}")
                
                # Add delay between requests to avoid rate limits
                if i < len(tokens) - 1:
                    time.sleep(random.uniform(1.0, 2.0))
            
            # Add trader wallets to owner_addresses.txt (merge with holders)
            if all_trader_wallets:
                print(f"\n{Colors.GREEN}✅ Collected {len(all_trader_wallets)} unique trader wallets from {traders_fetched} tokens{Colors.ENDC}")
                print(f"{Colors.CYAN}Merging traders with holders...{Colors.ENDC}")
                
                # Read existing holders
                existing_wallets = set(self.load_text_file(self.owner_addresses_file))
                
                # Merge traders with holders
                combined_wallets = existing_wallets.union(all_trader_wallets)
                
                # Save combined list
                with open(self.owner_addresses_file, 'w') as f:
                    for wallet in sorted(combined_wallets):
                        f.write(f"{wallet}\n")
                
                added_count = len(all_trader_wallets - existing_wallets)
                print(f"{Colors.GREEN}✅ Added {added_count} new trader wallets to analysis list (Total: {len(combined_wallets)}){Colors.ENDC}")
            else:
                print(f"\n{Colors.YELLOW}⚠️  No trader wallets found{Colors.ENDC}")
        
        # Step 4: Run Playwright Analysis
        print(f"\n{Colors.BOLD}{Colors.GREEN}✅ Token collection complete!{Colors.ENDC}\n")
        logger.info("Token collection complete. Starting wallet analysis phase...")
        self.run_dexcheck_analysis()
        
        logger.info("Complete workflow finished successfully!")
        print("PROGRESS: Analysis complete", flush=True)
        logger.info(f"Output files created/updated:")
        logger.info(f"  - {self.tokens_file}: Token information")
        logger.info(f"  - {self.tokens_txt}: Token addresses only")
        logger.info(f"  - {self.holders_file}: Full holders data")
        logger.info(f"  - {self.holders_txt}: Holder addresses only")
        logger.info(f"  - {self.owner_addresses_file}: Compatible holder addresses")
        logger.info(f"  - {self.good_wallets_file}: Profitable wallets found (JSON)")
        logger.info(f"  - {self.good_wallets_txt}: Profitable wallets found (TXT)")

def print_banner():
    """Print beautiful startup banner"""
    print(f"\n{Colors.BOLD}{Colors.CYAN}{'=' * 80}{Colors.ENDC}")
    print(f"{Colors.BOLD}{Colors.CYAN}🚀 SOLANA TOKEN & WALLET ANALYSIS ORCHESTRATOR 🚀{Colors.ENDC}")
    print(f"{Colors.BOLD}{Colors.CYAN}{'=' * 80}{Colors.ENDC}")
    print(f"{Colors.BOLD}Powered by Playwright Multi-Page Concurrent Scanner{Colors.ENDC}")
    print(f"{Colors.CYAN}Version 2.1 - Fast, Beautiful, Intelligent{Colors.ENDC}")
    print(f"{Colors.BOLD}{Colors.CYAN}{'=' * 80}{Colors.ENDC}\n")

def clean_restart():
    """Delete all data files for a clean restart"""
    print(f"\n{Colors.BOLD}{Colors.RED}🗑️  CLEAN RESTART{Colors.ENDC}")
    print(f"{Colors.RED}{'=' * 80}{Colors.ENDC}")
    print(f"{Colors.YELLOW}⚠️  This will delete ALL data files:{Colors.ENDC}")
    print(f"   - tokens.json, tokens.txt")
    print(f"   - holders.json, holders.txt")
    print(f"   - owner_addresses.txt")
    print(f"   - good_wallets.json, good_wallets.txt")
    print(f"   - scanned_wallets.txt")
    print(f"   - orchestrator.log")
    print(f"{Colors.RED}{'=' * 80}{Colors.ENDC}\n")
    
    confirm = input(f"{Colors.BOLD}{Colors.RED}Are you SURE you want to delete all data? (type 'yes' to confirm): {Colors.ENDC}").strip()
    if confirm.lower() != 'yes':
        print(f"{Colors.GREEN}✓ Clean restart cancelled{Colors.ENDC}\n")
        return False
    
    # Use organized structure paths
    data_dir = os.path.join(os.path.dirname(__file__), '..', 'data')
    logs_dir = os.path.join(os.path.dirname(__file__), '..', 'logs')
    
    files_to_delete = [
        os.path.join(data_dir, 'tokens.json'),
        os.path.join(data_dir, 'tokens.txt'),
        os.path.join(data_dir, 'holders.json'),
        os.path.join(data_dir, 'holders.txt'),
        os.path.join(data_dir, 'owner_addresses.txt'),
        os.path.join(data_dir, 'good_wallets.json'),
        os.path.join(data_dir, 'good_wallets.txt'),
        os.path.join(data_dir, 'scanned_wallets.txt'),
        os.path.join(logs_dir, 'orchestrator.log')
    ]
    
    deleted_count = 0
    for filepath in files_to_delete:
        if os.path.exists(filepath):
            try:
                os.remove(filepath)
                print(f"{Colors.GREEN}✓ Deleted: {os.path.basename(filepath)}{Colors.ENDC}")
                deleted_count += 1
            except Exception as e:
                print(f"{Colors.RED}❌ Failed to delete {os.path.basename(filepath)}: {e}{Colors.ENDC}")
    
    print(f"\n{Colors.GREEN}✅ Clean restart complete! Deleted {deleted_count} files.{Colors.ENDC}\n")
    return True

def get_user_input():
    """Interactive menu to get user configuration"""
    print(f"{Colors.BOLD}{Colors.CYAN}📋 CONFIGURATION MENU{Colors.ENDC}")
    print(f"{Colors.CYAN}{'=' * 80}{Colors.ENDC}\n")
    
    # Clean restart option
    print(f"{Colors.BOLD}0. Clean Restart?{Colors.ENDC}")
    print(f"   {Colors.CYAN}(Delete all data files and start fresh){Colors.ENDC}")
    clean = input(f"   {Colors.YELLOW}Clean restart? (y/n) [default: n]: {Colors.ENDC}").strip().lower()
    if clean == 'y':
        if clean_restart():
            print(f"{Colors.GREEN}✓ Starting with clean slate{Colors.ENDC}\n")
        else:
            print(f"{Colors.YELLOW}✓ Keeping existing data{Colors.ENDC}\n")
    else:
        print(f"   {Colors.GREEN}✓ Keeping existing data{Colors.ENDC}\n")
    
    # Token Source
    while True:
        print(f"{Colors.BOLD}1. Which token source to use?{Colors.ENDC}")
        print(f"   {Colors.CYAN}1. Moralis (PumpFun Graduated){Colors.ENDC}")
        print(f"   {Colors.CYAN}2. Birdeye (Liquidity-based){Colors.ENDC}")
        source = input(f"   {Colors.YELLOW}Enter choice [default: 2]: {Colors.ENDC}").strip()
        if source == '1':
            token_source = 'moralis'
            print(f"   {Colors.GREEN}✓ Token Source: Moralis{Colors.ENDC}\n")
            break
        elif source in ['', '2']:
            token_source = 'birdeye'
            print(f"   {Colors.GREEN}✓ Token Source: Birdeye{Colors.ENDC}\n")
            break
        else:
            print(f"   {Colors.RED}❌ Invalid input. Please enter 1 or 2.{Colors.ENDC}\n")

    # Token limit
    while True:
        try:
            print(f"{Colors.BOLD}2. How many tokens to fetch?{Colors.ENDC}")
            print(f"   {Colors.CYAN}(Recommended: 50-100, Max: 1000){Colors.ENDC}")
            token_limit = input(f"   {Colors.YELLOW}Enter value [default: 100]: {Colors.ENDC}").strip()
            token_limit = int(token_limit) if token_limit else 100
            if token_limit < 1:
                print(f"   {Colors.RED}❌ Must be at least 1{Colors.ENDC}\n")
                continue
            if token_limit > 1000:
                print(f"   {Colors.YELLOW}⚠️  Large value! This will take a long time.{Colors.ENDC}")
                confirm = input(f"   Continue? (y/n): ").strip().lower()
                if confirm != 'y':
                    continue
            print(f"   {Colors.GREEN}✓ Token limit: {token_limit}{Colors.ENDC}\n")
            break
        except ValueError:
            print(f"   {Colors.RED}❌ Invalid input. Please enter a number.{Colors.ENDC}\n")
    
    # Number of pages
    while True:
        try:
            print(f"{Colors.BOLD}3. How many concurrent browser pages for scanning?{Colors.ENDC}")
            print(f"   {Colors.CYAN}(More pages = faster, but uses more memory){Colors.ENDC}")
            print(f"   {Colors.CYAN}Recommended: 3-5 for most systems, 7-10 for powerful PCs{Colors.ENDC}")
            num_pages = input(f"   {Colors.YELLOW}Enter value [default: 3]: {Colors.ENDC}").strip()
            num_pages = int(num_pages) if num_pages else 3
            if num_pages < 1:
                print(f"   {Colors.RED}❌ Must be at least 1{Colors.ENDC}\n")
                continue
            if num_pages > 10:
                print(f"   {Colors.YELLOW}⚠️  High page count may cause performance issues{Colors.ENDC}")
                confirm = input(f"   Continue? (y/n): ").strip().lower()
                if confirm != 'y':
                    continue
            print(f"   {Colors.GREEN}✓ Concurrent pages: {num_pages}{Colors.ENDC}\n")
            break
        except ValueError:
            print(f"   {Colors.RED}❌ Invalid input. Please enter a number.{Colors.ENDC}\n")
    
    # Win rate
    while True:
        try:
            print(f"{Colors.BOLD}4. Minimum Win Rate (percentage)?{Colors.ENDC}")
            print(f"   {Colors.CYAN}(Higher = stricter filtering, fewer results){Colors.ENDC}")
            print(f"   {Colors.CYAN}Examples: 85=strict, 70=moderate, 60=relaxed, 0=all{Colors.ENDC}")
            min_winrate = input(f"   {Colors.YELLOW}Enter value [default: 70]: {Colors.ENDC}").strip()
            min_winrate = float(min_winrate) if min_winrate else 70.0
            if min_winrate < 0 or min_winrate > 100:
                print(f"   {Colors.RED}❌ Must be between 0 and 100{Colors.ENDC}\n")
                continue
            print(f"   {Colors.GREEN}✓ Min Win Rate: {min_winrate}%{Colors.ENDC}\n")
            break
        except ValueError:
            print(f"   {Colors.RED}❌ Invalid input. Please enter a number.{Colors.ENDC}\n")
    
    # Realized PnL
    while True:
        try:
            print(f"{Colors.BOLD}5. Minimum Realized PnL (return percentage)?{Colors.ENDC}")
            print(f"   {Colors.CYAN}(This is return %, not dollars){Colors.ENDC}")
            print(f"   {Colors.CYAN}Examples: 200=3x return, 100=2x return, 50=1.5x return, 0=any{Colors.ENDC}")
            min_pnl = input(f"   {Colors.YELLOW}Enter value [default: 100]: {Colors.ENDC}").strip()
            min_pnl = float(min_pnl) if min_pnl else 100.0
            if min_pnl < -100:
                print(f"   {Colors.RED}❌ Value too low (minimum -100%){Colors.ENDC}\n")
                continue
            print(f"   {Colors.GREEN}✓ Min Realized PnL: {min_pnl}%{Colors.ENDC}\n")
            break
        except ValueError:
            print(f"   {Colors.RED}❌ Invalid input. Please enter a number.{Colors.ENDC}\n")
    
    # Fetch top traders option
    print(f"{Colors.BOLD}6. Fetch top traders for each token?{Colors.ENDC}")
    print(f"   {Colors.CYAN}(This will fetch top performing traders from Birdeye API){Colors.ENDC}")
    print(f"   {Colors.CYAN}(Traders will be added to the wallet analysis list){Colors.ENDC}")
    fetch_traders = input(f"   {Colors.YELLOW}Fetch top traders? (y/n) [default: n]: {Colors.ENDC}").strip().lower()
    if fetch_traders == 'y':
        print(f"   {Colors.GREEN}✓ Top traders fetching: ENABLED{Colors.ENDC}\n")
    else:
        print(f"   {Colors.GREEN}✓ Top traders fetching: DISABLED{Colors.ENDC}\n")
    
    # Resume option
    print(f"{Colors.BOLD}7. Resume from a specific token?{Colors.ENDC}")
    print(f"   {Colors.CYAN}(Leave empty to start from beginning){Colors.ENDC}")
    while True:
        try:
            resume_from = input(f"   {Colors.YELLOW}Enter token number to resume from [default: 1]: {Colors.ENDC}").strip()
            resume_from = int(resume_from) if resume_from else 0
            if resume_from < 0:
                print(f"   {Colors.RED}❌ Must be 0 or greater{Colors.ENDC}\n")
                continue
            if resume_from > 0:
                print(f"   {Colors.GREEN}✓ Resuming from token {resume_from}{Colors.ENDC}\n")
            else:
                print(f"   {Colors.GREEN}✓ Starting from beginning{Colors.ENDC}\n")
            break
        except ValueError:
            print(f"   {Colors.RED}❌ Invalid input. Please enter a number.{Colors.ENDC}\n")
    
    # Auto-loop option
    print(f"{Colors.BOLD}8. Auto-loop mode?{Colors.ENDC}")
    print(f"   {Colors.CYAN}(Automatically restart and run again after each completion){Colors.ENDC}")
    auto_loop = input(f"   {Colors.YELLOW}Enable auto-loop? (y/n) [default: n]: {Colors.ENDC}").strip().lower()
    
    loop_interval_minutes = 0
    if auto_loop == 'y':
        while True:
            try:
                print(f"\n   {Colors.BOLD}How often to run (in minutes)?{Colors.ENDC}")
                print(f"   {Colors.CYAN}Examples: 60=1 hour, 120=2 hours, 300=5 hours, 600=10 hours{Colors.ENDC}")
                interval = input(f"   {Colors.YELLOW}Enter interval in minutes [default: 60]: {Colors.ENDC}").strip()
                loop_interval_minutes = int(interval) if interval else 60
                if loop_interval_minutes < 1:
                    print(f"   {Colors.RED}❌ Must be at least 1 minute{Colors.ENDC}\n")
                    continue
                if loop_interval_minutes < 30:
                    print(f"   {Colors.YELLOW}⚠️  Very short interval! This may cause high server load.{Colors.ENDC}")
                    confirm = input(f"   Continue? (y/n): ").strip().lower()
                    if confirm != 'y':
                        continue
                hours = loop_interval_minutes / 60
                print(f"   {Colors.GREEN}✓ Auto-loop enabled: every {loop_interval_minutes} minutes ({hours:.1f} hours){Colors.ENDC}\n")
                break
            except ValueError:
                print(f"   {Colors.RED}❌ Invalid input. Please enter a number.{Colors.ENDC}\n")
    else:
        print(f"   {Colors.GREEN}✓ Auto-loop disabled (run once){Colors.ENDC}\n")
    
    # Summary
    print(f"{Colors.BOLD}{Colors.CYAN}{'=' * 80}{Colors.ENDC}")
    print(f"{Colors.BOLD}📊 CONFIGURATION SUMMARY:{Colors.ENDC}")
    print(f"{Colors.CYAN}{'=' * 80}{Colors.ENDC}")
    print(f"  🪙 Tokens to fetch: {Colors.YELLOW}{token_limit}{Colors.ENDC}")
    print(f"  📊 Token Source: {Colors.YELLOW}{token_source.upper()}{Colors.ENDC}")
    print(f"  🎭 Concurrent pages: {Colors.YELLOW}{num_pages}{Colors.ENDC}")
    print(f"  🎯 Min Win Rate: {Colors.YELLOW}{min_winrate}%{Colors.ENDC}")
    print(f"  💰 Min Realized PnL: {Colors.YELLOW}{min_pnl}%{Colors.ENDC} (return %)")
    if fetch_traders == 'y':
        print(f"  🏆 Top Traders: {Colors.GREEN}ENABLED{Colors.ENDC} (fetching from Birdeye)")
    else:
        print(f"  🏆 Top Traders: {Colors.YELLOW}DISABLED{Colors.ENDC}")
    if resume_from > 0:
        print(f"  📍 Resume from: {Colors.YELLOW}Token {resume_from}{Colors.ENDC}")
    print(f"  ✅ Duplicate prevention: {Colors.GREEN}ENABLED{Colors.ENDC} (won't scan same wallet twice)")
    if loop_interval_minutes > 0:
        hours = loop_interval_minutes / 60
        print(f"  🔁 Auto-loop: {Colors.GREEN}ENABLED{Colors.ENDC} (every {loop_interval_minutes} min / {hours:.1f} hrs)")
    else:
        print(f"  🔁 Auto-loop: {Colors.YELLOW}DISABLED{Colors.ENDC} (run once)")
    print(f"{Colors.CYAN}{'=' * 80}{Colors.ENDC}\n")
    
    confirm = input(f"{Colors.BOLD}Proceed with this configuration? (y/n): {Colors.ENDC}").strip().lower()
    if confirm != 'y':
        print(f"\n{Colors.YELLOW}Configuration cancelled. Exiting...{Colors.ENDC}\n")
        sys.exit(0)
    
    print(f"\n{Colors.GREEN}✅ Configuration confirmed! Starting workflow...{Colors.ENDC}\n")
    
    return {
        'token_limit': token_limit,
        'num_pages': num_pages,
        'min_winrate': min_winrate,
        'min_pnl': min_pnl,
        'token_source': token_source,
        'fetch_traders': fetch_traders == 'y',
        'resume_from': resume_from,
        'auto_loop': loop_interval_minutes > 0,
        'loop_interval_minutes': loop_interval_minutes
    }

def main():
    parser = argparse.ArgumentParser(
        description="🚀 Solana Token and Wallet Analysis Orchestrator with Playwright Scanner",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog='''
{bold}Examples:{endc}
  # Interactive mode (recommended) - prompts for all settings
  python3 orchestrator.py
  
  # Command-line mode with arguments
  python3 orchestrator.py --pages 5 --min-winrate 80 --min-pnl 100
  
  # With Birdeye token source
  python3 orchestrator.py --token-source birdeye --limit 50
  
  # Auto-loop mode
  python3 orchestrator.py --loop 60 --token-source birdeye
'''.format(bold=Colors.BOLD, endc=Colors.ENDC)
    )
    
    parser.add_argument(
        '--non-interactive',
        action='store_true',
        help='Skip interactive menu and use command-line arguments or defaults'
    )
    
    parser.add_argument(
        '--resume',
        type=int,
        default=0,
        help='Token number to resume from (e.g., 38)'
    )
    
    parser.add_argument(
        '--limit',
        type=int,
        help='Maximum number of tokens to fetch (defaults to config value)'
    )
    
    parser.add_argument(
        '--pages',
        type=int,
        help='Number of concurrent browser pages for wallet analysis (default: 3)'
    )
    
    parser.add_argument(
        '--min-winrate',
        type=float,
        help='Minimum win rate percentage (default: from config.json)'
    )
    
    parser.add_argument(
        '--min-pnl',
        type=float,
        help='Minimum realized PnL percentage - return %%, not dollars (default: from config.json)'
    )
    
    parser.add_argument(
        '--loop',
        type=int,
        help='Enable auto-loop mode with specified interval in minutes (e.g., 60 for 1 hour)'
    )
    
    parser.add_argument(
        '--clean',
        action='store_true',
        help='Delete all data files before starting (clean restart)'
    )
    
    parser.add_argument(
        '--token-source',
        type=str,
        choices=['moralis', 'birdeye'],
        default='birdeye',
        help='Token source: moralis (PumpFun graduated) or birdeye (liquidity-based) [default: birdeye]'
    )
    
    args = parser.parse_args()
    
    # Print beautiful banner
    print_banner()
    
    # Handle clean restart from command line
    if args.clean:
        clean_restart()
    
    # Interactive mode or command-line mode
    if not args.non_interactive and not any([args.limit, args.pages, args.min_winrate, args.min_pnl, args.resume > 0, args.loop, args.token_source != 'birdeye']):
        # Interactive mode - get user input
        config = get_user_input()
        token_limit = config['token_limit']
        num_pages = config['num_pages']
        min_winrate = config['min_winrate']
        min_pnl = config['min_pnl']
        resume_from = config['resume_from']
        auto_loop = config['auto_loop']
        loop_interval_minutes = config['loop_interval_minutes']
        token_source = config['token_source']
        fetch_traders = config['fetch_traders']
    else:
        # Command-line mode - use arguments
        token_limit = args.limit
        num_pages = args.pages if args.pages else 3
        min_winrate = args.min_winrate
        min_pnl = args.min_pnl
        resume_from = args.resume
        auto_loop = args.loop is not None
        loop_interval_minutes = args.loop if args.loop else 0
        token_source = args.token_source
        fetch_traders = False  # Default to disabled in non-interactive mode
        
        # Validate pages
        if num_pages < 1:
            print(f"{Colors.RED}❌ Error: Number of pages must be at least 1{Colors.ENDC}")
            return
        
        if num_pages > 10:
            print(f"{Colors.YELLOW}⚠️  Warning: Using more than 10 pages may cause performance issues{Colors.ENDC}")
            response = input("Continue? (y/n): ")
            if response.lower() != 'y':
                return
    
    # Create orchestrator with parameters
    orchestrator = SolanaTokenOrchestrator(
        token_limit=token_limit,
        num_pages=num_pages,
        min_winrate=min_winrate,
        min_pnl=min_pnl
    )
    
    # Print configuration if not already shown
    if args.non_interactive or any([args.limit, args.pages, args.min_winrate, args.min_pnl, args.resume > 0, args.loop]):
        print(f"{Colors.BOLD}🔧 Configuration:{Colors.ENDC}")
        print(f"  🪙 Token Limit: {Colors.CYAN}{orchestrator.token_limit}{Colors.ENDC}")
        print(f"  📊 Token Source: {Colors.CYAN}{token_source.upper()}{Colors.ENDC}")
        print(f"  🎭 Concurrent Pages: {Colors.CYAN}{orchestrator.num_pages}{Colors.ENDC}")
        print(f"  🎯 Min Win Rate: {Colors.CYAN}{orchestrator.min_winrate}%{Colors.ENDC}")
        print(f"  💰 Min Realized PnL: {Colors.CYAN}{orchestrator.min_realized_pnl}%{Colors.ENDC} (return %)")
        print(f"  ✅ Duplicate prevention: {Colors.GREEN}ENABLED{Colors.ENDC}")
        if auto_loop:
            hours = loop_interval_minutes / 60
            print(f"  🔁 Auto-loop: {Colors.GREEN}ENABLED{Colors.ENDC} (every {loop_interval_minutes} min / {hours:.1f} hrs)")
        print()
    
    if resume_from > 0:
        print(f"{Colors.YELLOW}📍 Resuming workflow from token {resume_from}...{Colors.ENDC}\n")
    else:
        print(f"{Colors.GREEN}🚀 Starting complete workflow from the beginning...{Colors.ENDC}\n")

    # Main execution loop
    run_count = 0
    while True:
        run_count += 1
        
        if auto_loop and run_count > 1:
            print(f"\n{Colors.BOLD}{Colors.CYAN}{'=' * 80}{Colors.ENDC}")
            print(f"{Colors.BOLD}{Colors.CYAN}🔁 AUTO-LOOP RUN #{run_count}{Colors.ENDC}")
            print(f"{Colors.BOLD}{Colors.CYAN}{'=' * 80}{Colors.ENDC}\n")
        
        try:
            start_time = time.time()
            orchestrator.run_complete_workflow(resume_from=resume_from if run_count == 1 else 0, token_source=token_source, fetch_traders=fetch_traders)
            elapsed = time.time() - start_time
            
            print(f"\n{Colors.BOLD}{Colors.GREEN}{'=' * 80}{Colors.ENDC}")
            print(f"{Colors.BOLD}{Colors.GREEN}🎉 WORKFLOW COMPLETED SUCCESSFULLY! 🎉{Colors.ENDC}")
            print(f"{Colors.BOLD}{Colors.GREEN}{'=' * 80}{Colors.ENDC}")
            print(f"{Colors.BOLD}Run #{run_count} Time:{Colors.ENDC} {Colors.CYAN}{elapsed:.1f}s ({elapsed/60:.1f} minutes){Colors.ENDC}")
            print(f"{Colors.BOLD}{Colors.GREEN}{'=' * 80}{Colors.ENDC}\n")
            
            # If auto-loop is disabled, break after first run
            if not auto_loop:
                break
            
            # Wait for next run
            hours = loop_interval_minutes / 60
            print(f"{Colors.BOLD}{Colors.CYAN}🕒 Waiting {loop_interval_minutes} minutes ({hours:.1f} hours) before next run...{Colors.ENDC}")
            print(f"{Colors.CYAN}Next run will start at: {datetime.fromtimestamp(time.time() + loop_interval_minutes * 60).strftime('%Y-%m-%d %H:%M:%S')}{Colors.ENDC}")
            print(f"{Colors.YELLOW}Press Ctrl+C to stop auto-loop{Colors.ENDC}\n")
            
            time.sleep(loop_interval_minutes * 60)
            
        except KeyboardInterrupt:
            if auto_loop:
                print(f"\n{Colors.YELLOW}⚠️  Auto-loop interrupted by user{Colors.ENDC}")
                print(f"{Colors.CYAN}Completed {run_count} run(s) before stopping{Colors.ENDC}\n")
            else:
                print(f"\n{Colors.YELLOW}⚠️  Workflow interrupted by user{Colors.ENDC}")
            break
        except Exception as e:
            logger.error(f"Workflow failed with error: {e}", exc_info=True)
            print(f"\n{Colors.RED}❌ Workflow failed: {e}{Colors.ENDC}")
            
            if auto_loop:
                print(f"{Colors.YELLOW}⚠️  Error occurred in run #{run_count}{Colors.ENDC}")
                print(f"{Colors.CYAN}Auto-loop will continue after waiting period...{Colors.ENDC}\n")
                time.sleep(loop_interval_minutes * 60)
            else:
                break

if __name__ == "__main__":
    main()
