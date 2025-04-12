#!/usr/bin/env python3

import argparse
import logging
import os
import sys
from datetime import datetime

from pinecone import Pinecone

from assets.core.logging_setup import debug_environment, setup_logging
from assets.document_processor import CONFIG, process_documents, upload_file_to_pinecone

# Get logger
logger = logging.getLogger(__name__)


def upload_single_file(file_path: str, dry_run: bool = False):
    """Upload a single file to Pinecone."""
    pinecone_api_key = os.getenv("PINECONE_API_KEY")
    if not pinecone_api_key:
        logger.error("Pinecone API key not found in .env file")
        return False

    try:
        if dry_run:
            logger.info(f"DRY RUN: Would upload file {file_path} to Pinecone")
            logger.debug(
                f"DRY RUN: File metadata - Size: {os.path.getsize(file_path)} bytes"
            )
            return True

        pc = Pinecone(api_key=pinecone_api_key)
        index = pc.Index(CONFIG["index_name"])

        # Use the upload_file_to_pinecone function which now includes preprocessing
        if upload_file_to_pinecone(file_path, index, CONFIG["namespace"]):
            logger.info(f"Successfully uploaded {file_path} to Pinecone.")
            return True
        else:
            logger.error(f"Failed to upload {file_path} to Pinecone.")
            if logging.getLogger().level == logging.DEBUG:
                logger.debug(f"File details for {file_path}:")
                logger.debug(f"  - Size: {os.path.getsize(file_path)} bytes")
                logger.debug(
                    f"  - Created: {datetime.fromtimestamp(os.path.getctime(file_path)).strftime('%Y-%m-%d %H:%M:%S')}"
                )
                logger.debug(
                    f"  - Modified: {datetime.fromtimestamp(os.path.getmtime(file_path)).strftime('%Y-%m-%d %H:%M:%S')}"
                )
            return False
    except Exception as e:
        logger.error(f"Failed to initialize Pinecone client: {e}")
        return False


def main():
    parser = argparse.ArgumentParser(
        description="Manual actions for document processing."
    )
    parser.add_argument(
        "--process",
        action="store_true",
        help="Process all documents in the target folder.",
    )
    parser.add_argument("--upload", type=str, help="Upload a single file to Pinecone.")
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Run in dry run mode (no actual uploads).",
    )
    parser.add_argument(
        "--dry-run-update-cache",
        action="store_true",
        help="When used with --dry-run, updates the processed files cache as if files were processed.",
    )
    parser.add_argument(
        "--verbose",
        "-v",
        action="store_true",
        help="Enable verbose debug logging.",
    )
    # Add debug-env flag
    parser.add_argument(
        "--debug-env",
        action="store_true",
        help="Print debug information about environment variables and paths.",
    )
    # Add a new argument to force processing of large files
    parser.add_argument(
        "--force",
        action="store_true",
        help="Force processing of large files without confirmation.",
    )

    args = parser.parse_args()

    # Setup logging first based on verbosity
    setup_logging(args.verbose)

    # Print environment debug info if requested
    if args.debug_env:
        debug_environment()
        # Also check config if in verbose mode
        if args.verbose:
            from assets.core.logging_setup import check_config

            check_config()
        return

    # Log the current operation mode
    logger.debug(f"Command line arguments: {sys.argv}")
    logger.debug(f"Operation mode: {'DRY RUN' if args.dry_run else 'ACTUAL RUN'}")
    if args.dry_run_update_cache:
        logger.debug(f"Cache updates in dry run: {args.dry_run_update_cache}")

    # Add force flag to sys.argv if specified
    if args.force and "--force" not in sys.argv:
        sys.argv.append("--force")

    if args.process:
        logger.info(f"{'DRY RUN: ' if args.dry_run else ''}Processing all documents...")
        process_documents(dry_run=args.dry_run)
    elif args.upload:
        if os.path.exists(args.upload):
            upload_single_file(args.upload, dry_run=args.dry_run)
        else:
            logger.error(f"File not found: {args.upload}")
            sys.exit(1)
    else:
        parser.print_help()


if __name__ == "__main__":
    main()
