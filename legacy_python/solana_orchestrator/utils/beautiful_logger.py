#!/usr/bin/env python3
"""
Beautiful logging formatter for the orchestrator
"""
import logging
import sys

class Colors:
    HEADER = '\033[95m'
    BLUE = '\033[94m'
    CYAN = '\033[96m'
    GREEN = '\033[92m'
    YELLOW = '\033[93m'
    RED = '\033[91m'
    ENDC = '\033[0m'
    BOLD = '\033[1m'
    DIM = '\033[2m'

class BeautifulFormatter(logging.Formatter):
    """Custom formatter with colors and clean output"""
    
    FORMATS = {
        logging.DEBUG: f"{Colors.DIM}%(message)s{Colors.ENDC}",
        logging.INFO: f"{Colors.CYAN}ℹ️  %(message)s{Colors.ENDC}",
        logging.WARNING: f"{Colors.YELLOW}⚠️  %(message)s{Colors.ENDC}",
        logging.ERROR: f"{Colors.RED}❌ %(message)s{Colors.ENDC}",
        logging.CRITICAL: f"{Colors.RED}{Colors.BOLD}🚨 %(message)s{Colors.ENDC}"
    }
    
    def format(self, record):
        # Get the format for this log level
        log_fmt = self.FORMATS.get(record.levelno, "%(message)s")
        
        # Clean up common verbose patterns
        msg = record.getMessage()
        
        # Shorten long error messages
        if "Failed to establish a new connection" in msg:
            msg = "Network temporarily unavailable (retrying...)"
        elif "Max retries exceeded" in msg:
            # Extract just the essential info
            if "top-holders" in msg:
                token = msg.split("/")[-2] if "/" in msg else "unknown"
                msg = f"Connection failed for token {token[:8]}... (will retry)"
        elif "Network is unreachable" in msg:
            msg = "Network connection issue (continuing...)"
        
        # Apply the format
        record.msg = msg
        formatter = logging.Formatter(log_fmt)
        return formatter.format(record)

def setup_beautiful_logging():
    """Setup beautiful logging with both file and console handlers"""
    
    # File handler - keeps everything with timestamps
    file_handler = logging.FileHandler('orchestrator.log')
    file_handler.setLevel(logging.DEBUG)
    file_handler.setFormatter(
        logging.Formatter('%(asctime)s - %(levelname)s - %(message)s')
    )
    
    # Console handler - beautiful output
    console_handler = logging.StreamHandler(sys.stdout)
    console_handler.setLevel(logging.INFO)
    console_handler.setFormatter(BeautifulFormatter())
    
    # Configure root logger
    root_logger = logging.getLogger()
    root_logger.setLevel(logging.DEBUG)
    root_logger.handlers = []  # Clear existing handlers
    root_logger.addHandler(file_handler)
    root_logger.addHandler(console_handler)
    
    return root_logger
