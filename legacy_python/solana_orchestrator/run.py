#!/usr/bin/env python3
"""
🚀 Solana Token and Wallet Analysis Orchestrator Launcher 🚀
============================================================

Main entry point for the Solana Orchestrator system.
This script sets up proper paths and launches the orchestrator.

Usage:
    python run.py                           # Interactive mode
    python run.py --auto-loop              # Auto-loop mode
    python run.py --token-source birdeye   # Specify token source
    python run.py --help                   # Show help
"""

import sys
import os

# Add the core directory to Python path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), 'core'))
sys.path.insert(0, os.path.join(os.path.dirname(__file__), 'utils'))

# Change to the orchestrator directory so relative paths work
os.chdir(os.path.dirname(__file__))

# Import and run the orchestrator
from orchestrator import main

if __name__ == "__main__":
    main()