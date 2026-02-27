#!/usr/bin/env python3
"""
OpenFlare E2E Test Runner.

Usage:
    python run_tests.py              # Run all tests
    python run_tests.py -m cache     # Run only cache tests
    python run_tests.py -k purge     # Run tests matching 'purge'
    python run_tests.py -v           # Verbose output
"""

import subprocess
import sys


def main():
    args = ["python", "-m", "pytest", "-v", "--tb=short"] + sys.argv[1:]
    result = subprocess.run(args, cwd="/app")
    sys.exit(result.returncode)


if __name__ == "__main__":
    main()
