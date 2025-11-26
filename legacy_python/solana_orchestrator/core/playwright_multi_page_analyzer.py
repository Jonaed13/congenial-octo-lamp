"""
Playwright Multi-Page Analyzer - Concurrent page-based DexCheck scraping

This module provides a Playwright-based multi-page analyzer that opens one browser
with N concurrent pages, each independently scraping DexCheck for win rates and
realized PnL. Uses async/await patterns for true parallelism without race conditions.
"""

from playwright.async_api import async_playwright, Page, Browser, BrowserContext
import asyncio
import random
import time
import json
import os
import re
from typing import List, Dict, Optional, Tuple
from dataclasses import dataclass
import time as time_module


URL_TEMPLATE = "https://dexcheck.ai/app/wallet-analyzer/{wallet_address}"
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
            timestamp = int(time_module.time())
            f.write(f"{wallet_address}|{timestamp}\n")
    except Exception:
        pass


@dataclass
class PlaywrightAnalyzerConfig:
    """Configuration for Playwright multi-page analysis"""
    num_pages: int  # Number of concurrent pages
    wallets: List[str]  # Wallet addresses to analyze
    preset: str  # Analysis preset name
    output_dir: str = "results"
    min_winrate: float = 70.0
    min_realized_pnl: float = 100.0


class PageWorker:
    """Manages a single page within the browser for wallet analysis"""
    
    def __init__(self, page: Page, worker_id: int, config: PlaywrightAnalyzerConfig):
        self.page = page
        self.worker_id = worker_id
        self.config = config
        self.wallets_processed = 0
        self.wallets_passed = 0
    
    async def analyze_wallet(self, wallet_address: str) -> Optional[Dict]:
        """
        Analyze a single wallet by scraping DexCheck with retry mechanism.
        Returns dict with stats if wallet meets criteria, None otherwise.
        """
        max_retries = 3
        for attempt in range(max_retries):
            try:
                # Build URL
                url = URL_TEMPLATE.format(wallet_address=wallet_address)
                
                # Navigate to DexCheck
                await self.page.goto(url, wait_until="domcontentloaded", timeout=60000)
                
                # Wait up to 30 seconds for data to appear (but continue as soon as it loads)
                print(f"Page {self.worker_id}: Waiting for data (max 30s) for {wallet_address[:8]}...")
                success = await self._wait_for_data_with_timeout(30)
                
                if not success:
                    print(f"Page {self.worker_id}: ⏭️  Skipping {wallet_address[:8]}... (no data after 30s)")
                    return None  # Skip this wallet, don't retry
                
                # Give it 10 more seconds for the SVG spinners to finish and actual values to render
                await asyncio.sleep(10)
                
                # Extract win rate and realized PnL
                winrate = await self._extract_winrate()
                realized_pnl = await self._extract_realized_pnl()
                
                # Debug on first attempt
                if attempt == 0 and (winrate is None or realized_pnl is None):
                    html = await self.page.content()
                    # Save snippet for debugging
                    win_match = re.search(r'.{0,150}Win Rate.{0,150}', html, re.IGNORECASE)
                    if win_match:
                        print(f"DEBUG Win Rate context: {win_match.group(0)[:200]}")
                    real_match = re.search(r'.{0,150}Realized.{0,150}', html, re.IGNORECASE)
                    if real_match:
                        print(f"DEBUG Realized context: {real_match.group(0)[:200]}")
                
                if winrate is None or realized_pnl is None:
                    # Debug: show which field failed
                    missing = []
                    if winrate is None:
                        missing.append("winrate")
                    if realized_pnl is None:
                        missing.append("PnL")
                    print(f"Page {self.worker_id} attempt {attempt+1}: Failed to extract {','.join(missing)} for {wallet_address[:8]}...")
                    if attempt < max_retries - 1:
                        await asyncio.sleep(2)  # Brief pause before retry
                        continue
                    else:
                        return None
                
                # Display wallet stats with beautiful formatting
                self._display_wallet_result(wallet_address, winrate, realized_pnl)
                
                # Check if wallet meets criteria
                if winrate >= self.config.min_winrate and realized_pnl >= self.config.min_realized_pnl:
                    return {
                        "wallet": wallet_address,
                        "winrate": f"{winrate}%",
                        "realizedPnL": f"{realized_pnl}%",
                        "status": "passed"
                    }
                else:
                    # Return data but mark as failed criteria
                    return {
                        "wallet": wallet_address,
                        "winrate": f"{winrate}%",
                        "realizedPnL": f"{realized_pnl}%",
                        "status": "failed_criteria"
                    }
                
            except Exception as e:
                print(f"Page {self.worker_id} attempt {attempt+1} error: {wallet_address[:8]}... - {str(e)[:50]}")
                if attempt < max_retries - 1:
                    await asyncio.sleep(2)  # Brief pause before retry
                    continue
                else:
                    print(f"Page {self.worker_id} failed after {max_retries} attempts: {wallet_address[:8]}...")
                    return None
        
        return None
    
    def _display_wallet_result(self, wallet_address: str, winrate: float, realized_pnl: float):
        """Display wallet analysis result with beautiful formatting"""
        # Colors
        GREEN = '\033[92m'
        RED = '\033[91m'
        YELLOW = '\033[93m'
        CYAN = '\033[96m'
        BOLD = '\033[1m'
        RESET = '\033[0m'
        
        # Shortened wallet address
        short_wallet = wallet_address[:8] + "..." + wallet_address[-4:]
        
        # Check each criteria
        wr_meets = winrate >= self.config.min_winrate
        pnl_meets = realized_pnl >= self.config.min_realized_pnl
        
        # Overall pass/fail
        if wr_meets and pnl_meets:
            status_icon = f"{GREEN}✅ PASS{RESET}"
            status_color = GREEN
        else:
            status_icon = f"{RED}❌ FAIL{RESET}"
            status_color = RED
        
        # Format winrate with color
        if wr_meets:
            wr_display = f"{GREEN}{winrate:.1f}%{RESET} {GREEN}✓{RESET}"
        else:
            wr_display = f"{RED}{winrate:.1f}%{RESET} {RED}✗{RESET} (need {self.config.min_winrate:.0f}%)"
        
        # Format PnL with color
        if pnl_meets:
            pnl_display = f"{GREEN}{realized_pnl:.1f}%{RESET} {GREEN}✓{RESET}"
        else:
            pnl_display = f"{RED}{realized_pnl:.1f}%{RESET} {RED}✗{RESET} (need {self.config.min_realized_pnl:.0f}%)"
        
        # Print beautiful result
        print(f"\n{CYAN}┌─ Page {self.worker_id} ─────────────────────────────────────────┐{RESET}")
        print(f"{CYAN}│{RESET} {BOLD}Wallet:{RESET} {short_wallet:45} {status_icon} {CYAN}│{RESET}")
        print(f"{CYAN}│{RESET} {BOLD}Win Rate:{RESET}  {wr_display:60} {CYAN}│{RESET}")
        print(f"{CYAN}│{RESET} {BOLD}PnL:{RESET}       {pnl_display:60} {CYAN}│{RESET}")
        print(f"{CYAN}└────────────────────────────────────────────────────┘{RESET}")
    
    async def _wait_for_data_with_timeout(self, timeout_seconds: int) -> bool:
        """
        Wait for DexCheck data to load within the specified timeout.
        Returns True if data appears, False if timeout.
        """
        try:
            # Wait for either stats section or no-data message
            await asyncio.wait_for(
                asyncio.gather(
                    self.page.wait_for_selector(
                        "xpath=//div[h3[contains(text(), 'Win Rate') or contains(text(), 'Gross Profit')]]",
                        timeout=timeout_seconds * 1000
                    )
                ),
                timeout=timeout_seconds
            )
            return True
        except asyncio.TimeoutError:
            # Check if it's a "no data" case
            try:
                page_text = await self.page.content()
                if "no data for this wallet" in page_text.lower() or "not a wallet address" in page_text.lower():
                    return False  # Valid "no data" response, don't retry
                return False  # Timeout without data, should retry
            except Exception:
                return False
        except Exception:
            return False
    
    async def _extract_winrate(self) -> Optional[float]:
        """Extract win rate percentage from the page"""
        try:
            # Get full page HTML
            html = await self.page.content()
            
            # Pattern: Win Rate</h3><p class="...text-2xl">XX.XX%
            match = re.search(r'Win Rate</h3><p[^>]*text-2xl[^>]*>([\d\.]+)%', html, re.IGNORECASE)
            if match:
                value = float(match.group(1))
                if 0 <= value <= 100:
                    return value
            
            # Fallback: Simple pattern
            match = re.search(r'Win Rate[^<]*<[^>]*>([\d\.]+)%', html, re.IGNORECASE)
            if match:
                value = float(match.group(1))
                if 0 <= value <= 100:
                    return value
            
            return None
        except Exception as e:
            return None
    
    async def _extract_realized_pnl(self) -> Optional[float]:
        """Extract realized PnL percentage (return %) from the page"""
        try:
            # Get full page HTML
            html = await self.page.content()
            
            # Pattern: Realized</p><p...>$XXX <span...>(Y.YY%)</span>
            match = re.search(r'Realized</p><p[^>]*>\$[\d,\.]+\s*<span[^>]*>\(([\d\.]+)%\)</span>', html, re.IGNORECASE)
            if match:
                value = float(match.group(1))
                return value
            
            # Fallback: Simple pattern - $XXX (Y.YY%)
            match = re.search(r'Realized[^\$]*\$[\d,\.]+[^\(]*\(([\d\.]+)%\)', html, re.IGNORECASE)
            if match:
                value = float(match.group(1))
                return value
            
            return None
        except Exception as e:
            return None


class PlaywrightMultiPageAnalyzer:
    """Main coordinator for Playwright multi-page wallet analysis"""
    
    def __init__(self, config: PlaywrightAnalyzerConfig):
        self.config = config
        self.results = []
        self.scanned_wallets_set = set()
        self.results_lock = asyncio.Lock()
        self.scanned_lock = asyncio.Lock()
        self.processed_total = 0
        self.passed_total = 0
        
        # Setup output directory
        os.makedirs(self.config.output_dir, exist_ok=True)
    
    async def _build_browser(self, playwright) -> Tuple[Browser, BrowserContext]:
        """Launch Chromium browser and create context with robust settings"""
        browser = await playwright.chromium.launch(
            headless=True,
            timeout=60000,  # 60 second launch timeout
            args=[
                "--disable-blink-features=AutomationControlled",
                "--no-sandbox",
                "--disable-dev-shm-usage",
                "--disable-web-security",
                "--disable-extensions",
                "--disable-plugins",
                "--disable-images",
                "--no-first-run",
                "--disable-background-timer-throttling",
                "--disable-backgrounding-occluded-windows",
                "--disable-renderer-backgrounding",
                "--memory-pressure-off"
            ]
        )
        
        context = await browser.new_context(
            viewport={"width": 1920, "height": 1080},
            user_agent="Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Safari/537.36",
            ignore_https_errors=True
        )
        
        # Set default timeouts for the context
        context.set_default_timeout(60000)  # 60 seconds
        context.set_default_navigation_timeout(90000)  # 90 seconds for navigation
        
        return browser, context
    
    async def _create_pages(self, context: BrowserContext, num_pages: int) -> List[PageWorker]:
        """Create multiple page objects and wrap them in PageWorkers"""
        pages = [await context.new_page() for _ in range(num_pages)]
        workers = [PageWorker(page, i, self.config) for i, page in enumerate(pages)]
        return workers
    
    def _load_unscanned_wallets(self) -> List[str]:
        """Load previously scanned wallets and filter them out"""
        scanned = load_scanned_wallets()
        self.scanned_wallets_set = scanned.copy()
        
        unscanned = [w for w in self.config.wallets if w not in scanned]
        
        filtered_count = len(self.config.wallets) - len(unscanned)
        if filtered_count > 0:
            print(f"ℹ️  Filtered out {filtered_count} already-scanned wallets")
        
        if not unscanned:
            print("⚠️  All wallets have already been scanned")
        
        return unscanned
    
    def _distribute_wallets(self, wallets: List[str], num_pages: int) -> List[List[str]]:
        """Split wallet list into N non-overlapping chunks"""
        if not wallets:
            return []
        
        # Adjust if we have fewer wallets than pages
        actual_pages = min(num_pages, len(wallets))
        if actual_pages < num_pages:
            print(f"ℹ️  Only {len(wallets)} wallets available, using {actual_pages} pages instead of {num_pages}")
        
        # Distribute wallets across pages
        chunks = [[] for _ in range(actual_pages)]
        for idx, wallet in enumerate(wallets):
            chunks[idx % actual_pages].append(wallet)
        
        # Log distribution
        for idx, chunk in enumerate(chunks):
            print(f"Page {idx}: {len(chunk)} wallets")
        
        return chunks
    
    async def _page_worker_task(self, worker: PageWorker, wallet_chunk: List[str], progress_callback=None):
        """Task that processes a chunk of wallets on a single page"""
        worker_id = worker.worker_id
        processed_count = 0
        passed_count = 0
        
        for wallet_address in wallet_chunk:
            # Analyze wallet
            try:
                result = await worker.analyze_wallet(wallet_address)
                
                # Mark as scanned
                save_scanned_wallet(wallet_address)
                async with self.scanned_lock:
                    self.scanned_wallets_set.add(wallet_address)
                
                processed_count += 1
                self.processed_total += 1
                
                if result and result.get("status") == "passed":
                    # Good wallet that passed criteria
                    passed_count += 1
                    self.passed_total += 1
                    async with self.results_lock:
                        self.results.append(result)
                        # Save immediately so we don't lose results if stopped
                        self._save_results()
                    
                    if progress_callback:
                        try:
                            progress_callback({
                                "page_id": worker_id,
                                "wallet": wallet_address,
                                "status": "passed",
                                "progress": processed_count
                            })
                        except Exception:
                            pass
                else:
                    if progress_callback:
                        try:
                            progress_callback({
                                "page_id": worker_id,
                                "wallet": wallet_address,
                                "status": "failed",
                                "progress": processed_count
                            })
                        except Exception:
                            pass
                
                # Delay between wallets (optimized for fast VPS)
                await asyncio.sleep(1.0)
                
            except Exception as e:
                print(f"Page {worker_id} error on wallet {wallet_address}: {e}")
        
        print(f"Page {worker_id} completed: processed={processed_count}, passed={passed_count}")
    
    def _save_results(self):
        """Save good wallets to JSON file"""
        json_path = os.path.join(self.config.output_dir, "good_wallets.json")
        try:
            with open(json_path, 'w') as f:
                json.dump(self.results, f, indent=2)
            print(f"Saved {len(self.results)} good wallets to {json_path}")
        except Exception as e:
            print(f"Error saving results: {e}")
    
    async def run(self, progress_callback=None) -> Dict:
        """
        Main execution method:
        - Load unscanned wallets
        - Launch browser and create pages
        - Distribute wallets across pages
        - Run all pages concurrently
        - Aggregate and save results
        """
        try:
            # Load unscanned wallets
            unscanned_wallets = self._load_unscanned_wallets()
            if not unscanned_wallets:
                return {
                    "success": True,
                    "total_scanned": 0,
                    "total_passed": 0,
                    "good_wallets": [],
                    "message": "All wallets already scanned"
                }
            
            # Launch Playwright
            async with async_playwright() as playwright:
                # Build browser and context
                print("Launching Chromium browser...")
                browser, context = await self._build_browser(playwright)
                
                # Create page workers
                print(f"Creating {self.config.num_pages} pages...")
                workers = await self._create_pages(context, self.config.num_pages)
                
                # Distribute wallets
                wallet_chunks = self._distribute_wallets(unscanned_wallets, len(workers))
                
                # Run all pages concurrently
                print(f"Starting concurrent analysis across {len(workers)} pages...")
                tasks = [
                    self._page_worker_task(worker, chunk, progress_callback)
                    for worker, chunk in zip(workers, wallet_chunks)
                    if chunk  # Only create task if chunk is not empty
                ]
                
                await asyncio.gather(*tasks)
                
                # Close browser
                await browser.close()
            
            # Save results
            self._save_results()
            
            # Return summary
            return {
                "success": True,
                "total_scanned": self.processed_total,
                "total_passed": self.passed_total,
                "good_wallets": self.results
            }
            
        except Exception as e:
            print(f"Fatal error in Playwright analyzer: {e}")
            return {
                "success": False,
                "error": str(e),
                "total_scanned": self.processed_total,
                "total_passed": self.passed_total,
                "good_wallets": self.results
            }


def create_playwright_analyzer(num_pages: int, wallets: List[str], preset: str, **kwargs) -> PlaywrightMultiPageAnalyzer:
    """Factory function to create a PlaywrightMultiPageAnalyzer instance"""
    # Load preset thresholds from environment
    min_winrate = float(os.environ.get("MIN_WINRATE", "70.0"))
    min_pnl = float(os.environ.get("MIN_REALIZED_PNL_USD", "100.0"))
    
    config = PlaywrightAnalyzerConfig(
        num_pages=num_pages,
        wallets=wallets,
        preset=preset,
        min_winrate=min_winrate,
        min_realized_pnl=min_pnl,
        **kwargs
    )
    
    return PlaywrightMultiPageAnalyzer(config)
