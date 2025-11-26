#!/usr/bin/env python3
"""
Playwright Multi-Page Wallet Scanner
=====================================

Features:
- Select number of concurrent pages (browser tabs)
- Each page scans different wallets simultaneously
- Beautiful real-time UI showing which page is analyzing which wallet
- No duplicate scans (tracks analyzed wallets)
- Uses Playwright with fixed XPath selectors

Author: AI Assistant
Date: 2025-10-02
"""

import asyncio
import json
import os
import re
import random
import time
from typing import List, Dict, Optional, Set
from datetime import datetime
from playwright.async_api import async_playwright, Page, Browser, BrowserContext
from dataclasses import dataclass
import argparse

# Simple wallet tracking (inline)
TRACKING_AVAILABLE = True
SCANNED_WALLETS_FILE = os.path.join("..", "data", "scanned_wallets.txt")

def load_scanned_wallets() -> set:
    """Load previously scanned wallets from file"""
    scanned_file = os.path.join(os.path.dirname(__file__), SCANNED_WALLETS_FILE)
    if os.path.exists(scanned_file):
        try:
            with open(scanned_file, 'r') as f:
                wallets = set()
                for line in f:
                    line = line.strip()
                    if line and '|' in line:
                        wallet = line.split('|')[0]
                        wallets.add(wallet)
                return wallets
        except Exception:
            pass
    return set()

def save_scanned_wallet(wallet_address: str):
    """Save a wallet address as scanned"""
    scanned_file = os.path.join(os.path.dirname(__file__), SCANNED_WALLETS_FILE)
    try:
        os.makedirs(os.path.dirname(scanned_file), exist_ok=True)
        with open(scanned_file, 'a') as f:
            timestamp = int(time.time())
            f.write(f"{wallet_address}|{timestamp}\n")
    except Exception:
        pass

# Configuration
URL_TEMPLATE = "https://dexcheck.ai/app/wallet-analyzer/{wallet_address}"
INPUT_FILE = "owner_addresses.txt"
OUTPUT_FILE = "good_wallets.json"

# Filter criteria (can be overridden by environment variables)
MIN_WINRATE = float(os.environ.get("MIN_WINRATE", "70.0"))
MIN_REALIZED_PNL = float(os.environ.get("MIN_REALIZED_PNL_USD", "100.0"))

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


@dataclass
class PageStatus:
    """Status of a single page worker"""
    page_id: int
    current_wallet: Optional[str] = None
    status: str = "Idle"
    wallets_processed: int = 0
    wallets_passed: int = 0


class PlaywrightWalletScanner:
    """Multi-page concurrent wallet scanner using Playwright"""
    
    def __init__(self, num_pages: int, min_winrate: float, min_pnl: float):
        self.num_pages = num_pages
        self.min_winrate = min_winrate
        self.min_pnl = min_pnl
        
        # State tracking
        self.page_statuses: List[PageStatus] = [PageStatus(i) for i in range(num_pages)]
        self.results: List[Dict] = []
        self.results_lock = asyncio.Lock()
        self.scanned_wallets: Set[str] = set()
        self.scanned_lock = asyncio.Lock()
        
        # Statistics
        self.total_processed = 0
        self.total_passed = 0
        self.total_skipped = 0
        self.start_time = None
        
    def print_header(self):
        """Print beautiful header"""
        print("\n" + "=" * 80)
        print(f"{Colors.BOLD}{Colors.CYAN}🚀 PLAYWRIGHT MULTI-PAGE WALLET SCANNER 🚀{Colors.ENDC}")
        print("=" * 80)
        print(f"{Colors.BOLD}Configuration:{Colors.ENDC}")
        print(f"  📊 Number of Pages: {Colors.YELLOW}{self.num_pages}{Colors.ENDC}")
        print(f"  🎯 Min Win Rate: {Colors.YELLOW}{self.min_winrate}%{Colors.ENDC}")
        print(f"  💰 Min Realized PnL: {Colors.YELLOW}{self.min_pnl}%{Colors.ENDC}")
        print(f"  📁 Input File: {Colors.CYAN}{INPUT_FILE}{Colors.ENDC}")
        print(f"  💾 Output File: {Colors.CYAN}{OUTPUT_FILE}{Colors.ENDC}")
        print("=" * 80 + "\n")
    
    def print_status_update(self):
        """Print machine-readable status update for orchestrator"""
        failed = self.total_processed - self.total_passed
        status_data = {
            'processed': self.total_processed,
            'passed': self.total_passed,
            'failed': failed,
            'skipped': self.total_skipped
        }
        print(f"STATUS: {json.dumps(status_data)}")
    
    async def load_wallets(self) -> List[str]:
        """Load wallet addresses from file"""
        if not os.path.exists(INPUT_FILE):
            print(f"{Colors.RED}❌ Error: {INPUT_FILE} not found{Colors.ENDC}")
            return []
        
        with open(INPUT_FILE, 'r') as f:
            wallets = [line.strip() for line in f if line.strip()]
        
        print(f"{Colors.GREEN}✅ Loaded {len(wallets)} wallet addresses{Colors.ENDC}\n")
        return wallets
    
    def load_existing_results(self) -> Set[str]:
        """Load already processed wallets from output file"""
        if not os.path.exists(OUTPUT_FILE):
            return set()
        
        try:
            with open(OUTPUT_FILE, 'r') as f:
                existing = json.load(f)
                return {w['wallet'] for w in existing}
        except:
            return set()
    
    def distribute_wallets(self, wallets: List[str]) -> List[List[str]]:
        """Distribute wallets across pages (round-robin)"""
        chunks = [[] for _ in range(self.num_pages)]
        for idx, wallet in enumerate(wallets):
            chunks[idx % self.num_pages].append(wallet)
        
        print(f"{Colors.BOLD}📦 WALLET DISTRIBUTION:{Colors.ENDC}")
        for idx, chunk in enumerate(chunks):
            print(f"  Page #{idx}: {Colors.CYAN}{len(chunk)} wallets{Colors.ENDC}")
        print()
        
        return chunks
    
    async def extract_winrate(self, page: Page) -> Optional[float]:
        """Extract win rate using fixed selector"""
        try:
            text = await page.locator(
                "xpath=//div[h3[contains(text(), 'Win Rate')]]//"
                "p[contains(@class, 'font-cousine') and contains(@class, 'text-2xl') and contains(text(), '%')]"
            ).first.text_content(timeout=5000)
            
            match = re.search(r'([\d\.,]+)', text)
            if match:
                value = float(match.group(1).replace(',', ''))
                if 0 <= value <= 100:
                    return value
            return None
        except Exception:
            # Fallback to regex
            try:
                html = await page.content()
                match = re.search(r"Win Rate\s*</h3>\s*<p[^>]*>\s*([\d\.,]+)\s*%?", html, re.IGNORECASE)
                if match:
                    value = float(match.group(1).replace(',', ''))
                    if 0 <= value <= 100:
                        return value
            except:
                pass
            return None
    
    async def extract_realized_pnl(self, page: Page) -> Optional[float]:
        """Extract realized PnL percentage (return %) using fixed selector"""
        try:
            # Get the first <p> element which contains both $ and (%)
            # Text format: "$1,234 (15.5%)" or "$0 (0%)"
            text = await page.locator(
                "xpath=//div[h3[contains(text(), 'Gross Profit')]]//"
                "div[p[contains(text(), 'Realized')]]//"
                "p[contains(@class, 'font-cousine') and contains(@class, 'text-sm')]"
            ).first.text_content(timeout=5000)
            
            # Extract percentage from parentheses: "$1,234 (15.5%)" -> 15.5
            match = re.search(r'\(([\d\.,]+)%\)', text)
            if match:
                percentage = float(match.group(1).replace(',', ''))
                return percentage
            
            # If no parentheses, try to extract any percentage
            match = re.search(r'([\d\.,]+)%', text)
            if match:
                percentage = float(match.group(1).replace(',', ''))
                return percentage
            
            return None
        except Exception:
            # Fallback to regex
            try:
                html = await page.content()
                match = re.search(r'Realized\s*</p>\s*<p[^>]*>\s*\$[\d\.,]+\s*\(([\d\.,]+)%\)', html, re.IGNORECASE)
                if match:
                    percentage = float(match.group(1).replace(',', ''))
                    return percentage
            except:
                pass
            return None
    
    async def analyze_wallet(self, page: Page, wallet: str, page_id: int) -> Optional[Dict]:
        """Analyze a single wallet with retry mechanism"""
        max_retries = 3
        
        for attempt in range(max_retries):
            try:
                # Update status
                self.page_statuses[page_id].current_wallet = wallet
                self.page_statuses[page_id].status = "Loading"
                
                # Navigate
                url = URL_TEMPLATE.format(wallet_address=wallet)
                await page.goto(url, wait_until="domcontentloaded", timeout=60000)
                
                # Quick 8-second timeout check for VPS
                success = await self._wait_for_data_with_timeout(page, 8)
                
                if not success:
                    print(f"Page {page_id} attempt {attempt+1}: No data within 8 seconds for {wallet[:8]}...")
                    if attempt < max_retries - 1:
                        await asyncio.sleep(2)  # Brief pause before retry
                        continue
                    else:
                        self.page_statuses[page_id].status = "Failed"
                        return None
                
                # Update status
                self.page_statuses[page_id].status = "Analyzing"
                
                # Extract stats
                winrate = await self.extract_winrate(page)
                realized_pnl = await self.extract_realized_pnl(page)
                
                if winrate is None or realized_pnl is None:
                    print(f"Page {page_id} attempt {attempt+1}: Failed to extract data for {wallet[:8]}...")
                    if attempt < max_retries - 1:
                        await asyncio.sleep(2)  # Brief pause before retry
                        continue
                    else:
                        return None
                
                # Success! Check criteria
                if winrate >= self.min_winrate and realized_pnl >= self.min_pnl:
                    print(f"Page {page_id} ✅ Found good wallet {wallet[:8]}... (WR: {winrate}%, PnL: {realized_pnl}%)")
                    return {
                        "wallet": wallet,
                        "winrate": f"{winrate}%",
                        "realizedPnL": f"{realized_pnl}%",
                        "timestamp": datetime.now().isoformat()
                    }
                else:
                    # Wallet doesn't meet criteria - no retry needed
                    return None
                
            except Exception as e:
                print(f"Page {page_id} attempt {attempt+1} error: {wallet[:8]}... - {str(e)[:50]}")
                if attempt < max_retries - 1:
                    await asyncio.sleep(2)  # Brief pause before retry
                    continue
                else:
                    return None
        
        return None
    
    async def _wait_for_data_with_timeout(self, page: Page, timeout_seconds: int) -> bool:
        """Wait for DexCheck data to load within the specified timeout"""
        try:
            # Wait for stats section to appear
            await asyncio.wait_for(
                page.wait_for_selector(
                    "xpath=//div[h3[contains(text(), 'Win Rate') or contains(text(), 'Gross Profit')]]",
                    timeout=timeout_seconds * 1000
                ),
                timeout=timeout_seconds
            )
            return True
        except asyncio.TimeoutError:
            # Check if it's a "no data" case
            try:
                page_text = await page.content()
                if "no data for this wallet" in page_text.lower() or "not a wallet address" in page_text.lower():
                    return False  # Valid "no data" response, don't retry
                return False  # Timeout without data, should retry
            except Exception:
                return False
        except Exception:
            return False
    
    async def page_worker(self, page: Page, wallet_chunk: List[str], page_id: int):
        """Worker function for a single page"""
        for wallet in wallet_chunk:
            # Check if already scanned
            async with self.scanned_lock:
                if wallet in self.scanned_wallets:
                    self.total_skipped += 1
                    continue
                self.scanned_wallets.add(wallet)
            
            # Mark as scanned in tracker
            if TRACKING_AVAILABLE:
                save_scanned_wallet(wallet)
            
            # Analyze wallet
            result = await self.analyze_wallet(page, wallet, page_id)
            
            # Update statistics
            self.page_statuses[page_id].wallets_processed += 1
            self.total_processed += 1
            
            if result:
                self.page_statuses[page_id].wallets_passed += 1
                self.total_passed += 1
                async with self.results_lock:
                    self.results.append(result)
            
            # Update display
            self.print_status_update()
            
            # Small delay between wallets (optimized for fast VPS)
            await asyncio.sleep(1.0)
        
        # Mark page as complete and clear current wallet
        self.page_statuses[page_id].status = "Complete"
        self.page_statuses[page_id].current_wallet = None
    
    async def run(self):
        """Main execution method"""
        self.start_time = time.time()
        
        # Print header
        self.print_header()
        
        # Load wallets
        all_wallets = await self.load_wallets()
        if not all_wallets:
            return
        
        # Load existing results
        existing = self.load_existing_results()
        print(f"{Colors.YELLOW}ℹ️  Found {len(existing)} already processed wallets{Colors.ENDC}\n")
        
        # Load scanned wallets from tracker
        if TRACKING_AVAILABLE:
            tracked = load_scanned_wallets()
            self.scanned_wallets = existing.union(tracked)
            print(f"{Colors.YELLOW}ℹ️  Loaded {len(tracked)} tracked wallets{Colors.ENDC}\n")
        else:
            self.scanned_wallets = existing
        
        # Filter out already scanned
        unscanned = [w for w in all_wallets if w not in self.scanned_wallets]
        print(f"{Colors.GREEN}✅ {len(unscanned)} new wallets to scan{Colors.ENDC}\n")
        
        if not unscanned:
            print(f"{Colors.YELLOW}⚠️  No new wallets to scan!{Colors.ENDC}")
            return
        
        # Distribute wallets
        wallet_chunks = self.distribute_wallets(unscanned)
        
        # Launch Playwright
        async with async_playwright() as playwright:
            print(f"{Colors.CYAN}🚀 Launching Chromium browser...{Colors.ENDC}\n")
            
            browser = await playwright.chromium.launch(
                headless=True,
                args=[
                    "--disable-blink-features=AutomationControlled",
                    "--no-sandbox",
                    "--disable-dev-shm-usage"
                ]
            )
            
            context = await browser.new_context(
                viewport={"width": 1920, "height": 1080},
                user_agent="Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Safari/537.36"
            )
            
            # Create pages
            pages = [await context.new_page() for _ in range(self.num_pages)]
            
            print(f"{Colors.GREEN}✅ Created {self.num_pages} concurrent pages{Colors.ENDC}\n")
            print(f"{Colors.BOLD}{Colors.CYAN}🔥 STARTING CONCURRENT ANALYSIS 🔥{Colors.ENDC}\n")
            
            await asyncio.sleep(2)  # Brief pause before starting
            
            # Initial display
            print(f"{Colors.BOLD}🔥 Starting concurrent wallet analysis...{Colors.ENDC}\n")
            
            # Run all page workers concurrently
            tasks = [
                self.page_worker(page, chunk, i)
                for i, (page, chunk) in enumerate(zip(pages, wallet_chunks))
                if chunk  # Only create task if chunk has wallets
            ]
            
            await asyncio.gather(*tasks)
            
            # Final status update to clear "Analyzing" display
            self.print_status_update()
            
            # Close browser
            await browser.close()
        
        # Save results
        await self.save_results()
        
        # Print final summary
        self.print_final_summary()
    
    async def save_results(self):
        """Save results to file"""
        if not self.results:
            return
        
        # Load existing results
        existing = []
        if os.path.exists(OUTPUT_FILE):
            try:
                with open(OUTPUT_FILE, 'r') as f:
                    existing = json.load(f)
            except:
                pass
        
        # Merge and save
        all_results = existing + self.results
        
        with open(OUTPUT_FILE, 'w') as f:
            json.dump(all_results, f, indent=2)
        
        # Also save to TXT
        txt_file = OUTPUT_FILE.replace('.json', '.txt')
        with open(txt_file, 'w') as f:
            f.write("=" * 60 + "\n")
            f.write("GOOD WALLETS - ANALYSIS RESULTS\n")
            f.write("=" * 60 + "\n\n")
            for wallet in all_results:
                f.write(f"Wallet: {wallet['wallet']}\n")
                f.write(f"Win Rate: {wallet['winrate']}\n")
                f.write(f"Realized PnL: {wallet['realizedPnL']}\n")
                if 'timestamp' in wallet:
                    f.write(f"Timestamp: {wallet['timestamp']}\n")
                f.write("-" * 60 + "\n\n")
    
    def print_final_summary(self):
        """Print final summary"""
        elapsed = time.time() - self.start_time
        failed = self.total_processed - self.total_passed
        
        print("\n\n" + "=" * 80)
        print(f"{Colors.BOLD}{Colors.GREEN}🎉 ANALYSIS COMPLETE! 🎉{Colors.ENDC}")
        print("=" * 80)
        print(f"{Colors.BOLD}Final Statistics:{Colors.ENDC}")
        print(f"  ⏱️  Total Time: {Colors.CYAN}{elapsed:.1f}s ({elapsed/60:.1f} minutes){Colors.ENDC}")
        print(f"  📊 Wallets Analyzed: {Colors.CYAN}{self.total_processed}{Colors.ENDC}")
        print(f"  ✅ Wallets Passed: {Colors.GREEN}{self.total_passed}{Colors.ENDC}")
        print(f"  ❌ Wallets Failed: {Colors.RED}{failed}{Colors.ENDC}")
        print(f"  ⏭️  Wallets Skipped: {Colors.YELLOW}{self.total_skipped}{Colors.ENDC}")
        print(f"  💎 Success Rate: {Colors.CYAN}{(self.total_passed / self.total_processed * 100) if self.total_processed > 0 else 0:.1f}%{Colors.ENDC}")
        print(f"  📈 Avg Time/Wallet: {Colors.CYAN}{elapsed/self.total_processed:.1f}s{Colors.ENDC}" if self.total_processed > 0 else "")
        print(f"\n{Colors.BOLD}Output Files:{Colors.ENDC}")
        print(f"  💾 JSON: {Colors.CYAN}{OUTPUT_FILE}{Colors.ENDC}")
        print(f"  📄 TXT: {Colors.CYAN}{OUTPUT_FILE.replace('.json', '.txt')}{Colors.ENDC}")
        print("=" * 80 + "\n")


async def main():
    """Main entry point"""
    parser = argparse.ArgumentParser(
        description="Playwright Multi-Page Wallet Scanner",
        formatter_class=argparse.RawDescriptionHelpFormatter
    )
    
    parser.add_argument(
        '--pages',
        type=int,
        default=3,
        help='Number of concurrent pages (default: 3)'
    )
    
    parser.add_argument(
        '--min-winrate',
        type=float,
        default=MIN_WINRATE,
        help=f'Minimum win rate percentage (default: {MIN_WINRATE})'
    )
    
    parser.add_argument(
        '--min-pnl',
        type=float,
        default=MIN_REALIZED_PNL,
        help=f'Minimum realized PnL percentage (default: {MIN_REALIZED_PNL}%% - means return %, not dollars)'
    )
    
    args = parser.parse_args()
    
    # Validate pages
    if args.pages < 1:
        print(f"{Colors.RED}❌ Error: Number of pages must be at least 1{Colors.ENDC}")
        return
    
    if args.pages > 10:
        print(f"{Colors.YELLOW}⚠️  Warning: Using more than 10 pages may cause performance issues{Colors.ENDC}")
        response = input("Continue? (y/n): ")
        if response.lower() != 'y':
            return
    
    # Create and run scanner
    scanner = PlaywrightWalletScanner(
        num_pages=args.pages,
        min_winrate=args.min_winrate,
        min_pnl=args.min_pnl
    )
    
    await scanner.run()


if __name__ == "__main__":
    asyncio.run(main())
