#!/usr/bin/env python3

import argparse
import logging
import os
import sys

from dotenv import load_dotenv

from assets.core.config import CONFIG
from assets.core.logging_setup import setup_logging
from assets.document_processor import process_documents

# Force load environment variables from .env with priority over existing env vars
load_dotenv(override=True)


# Define your Pinecone API key and other configurations
PINECONE_API_KEY = os.getenv("PINECONE_API_KEY")
# Use the properly expanded path from CONFIG
TARGET_FOLDER = CONFIG["TARGET_FOLDER"]
NAMESPACE = os.getenv("NAMESPACE", "your_namespace")
INDEX_NAME = os.getenv("INDEX_NAME", "your_index_name")

# Get the logger
logger = logging.getLogger(__name__)


def main():
    """Main function with error handling."""
    try:
        parser = argparse.ArgumentParser(description="Process and upload documents.")
        parser.add_argument(
            "--dry-run",
            action="store_true",
            help="Run the script in dry run mode (no uploads).",
        )
        parser.add_argument(
            "--dry-run-update-cache",
            action="store_true",
            help="When used with --dry-run, updates the processed files cache as if files were processed.",
        )
        # Add verbose flag to control logging and debug flag for environment info
        parser.add_argument(
            "--verbose",
            "-v",
            action="store_true",
            help="Enable verbose debug logging.",
        )
        parser.add_argument(
            "--debug-env",
            action="store_true",
            help="Print debug information about environment variables and paths.",
        )
        # Add recursive flag
        parser.add_argument(
            "--no-recursive",
            action="store_true",
            help="Don't search subdirectories recursively.",
        )
        args = parser.parse_args()

        # Setup logging with the appropriate verbosity
        setup_logging(args.verbose)

        # Add the arg to sys.argv to make it accessible in the process_documents function
        if (
            args.dry_run
            and args.dry_run_update_cache
            and "--dry-run-update-cache" not in sys.argv
        ):
            sys.argv.append("--dry-run-update-cache")

        # Print environment debug info if requested
        if args.debug_env:
            from assets.core.logging_setup import check_config, debug_environment

            debug_environment()
            check_config()
            return

        logger.debug(f"TARGET_FOLDER: {TARGET_FOLDER}")
        logger.debug(f"Namespace: {NAMESPACE}")
        logger.debug(f"Index Name: {INDEX_NAME}")

        # Pass the explicit target_folder from CONFIG to process_documents
        process_documents(
            target_folder=TARGET_FOLDER,
            dry_run=args.dry_run,
            recursive=not args.no_recursive,
        )
    except Exception as e:
        logger.error(f"Unexpected error in document processing: {e}")
        sys.exit(1)


if __name__ == "__main__":
    main()
