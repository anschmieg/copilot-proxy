import logging
import os
from datetime import datetime
from typing import List, Tuple

from pinecone import Pinecone

from assets.core.config import CONFIG, SUPPORTED_EXTENSIONS
from assets.core.file_utils import (
    check_file_changed,
    generate_file_hash,
    load_processed_files,
    save_processed_files,
)
from assets.core.logging_setup import logger
from assets.document_processor import preprocess_file
from assets.pinecone.uploader import upload_file_to_pinecone


def get_pinecone_index():
    """Get the Pinecone index for uploads."""
    try:
        pinecone_api_key = os.getenv("PINECONE_API_KEY")
        pc = Pinecone(api_key=pinecone_api_key)
        index = pc.Index(CONFIG["index_name"])
        return index
    except Exception as e:
        logger.error(f"Failed to initialize Pinecone client: {e}")
        return None


def find_processable_files(target_folder: str, recursive: bool = True) -> List[str]:
    """
    Find all files that can be processed in the target folder and it's subdirectories.

    Args:
        target_folder: Path to the folder to scan
        recursive: If True, scan subdirectories recursively

    Returns:
        List of paths to processable files
    """
    if not os.path.exists(target_folder):
        logger.error(f"Target folder does not exist: {target_folder}")
        return []

    result = []

    if recursive:
        # Use os.walk to recursively traverse directories
        for root, dirs, files in os.walk(target_folder):
            # Skip hidden directories (those starting with '.')
            dirs[:] = [d for d in dirs if not d.startswith(".")]

            for file_name in files:
                # Skip hidden files
                if file_name.startswith("."):
                    continue

                file_path = os.path.join(root, file_name)
                if os.path.isfile(file_path):
                    file_extension = os.path.splitext(file_name)[1].lower()
                    if file_extension in SUPPORTED_EXTENSIONS:
                        result.append(file_path)

        if result:
            logger.debug(f"Found {len(result)} files recursively in {target_folder}")
    else:
        # Original non-recursive implementation
        try:
            files = os.listdir(target_folder)
            for file_name in files:
                if file_name.startswith("."):
                    continue

                file_path = os.path.join(target_folder, file_name)
                if os.path.isfile(file_path):
                    file_extension = os.path.splitext(file_name)[1].lower()
                    if file_extension in SUPPORTED_EXTENSIONS:
                        result.append(file_path)
        except Exception as e:
            logger.error(f"Error listing directory {target_folder}: {e}")

    return result


def process_document_folder(
    target_folder: str, dry_run: bool = False, recursive: bool = True
) -> Tuple[int, int]:
    """
    Process all documents in the target folder.

    Args:
        target_folder: Path to the folder containing documents
        dry_run: If True, don't actually upload any documents
        recursive: If True, process documents in subdirectories recursively

    Returns:
        Tuple[int, int]: (total_files_processed, successfully_uploaded)
    """
    target_folder = os.path.expanduser(target_folder)
    logger.debug(
        f"Processing documents{' recursively' if recursive else ''} in folder: {target_folder}"
    )

    files = find_processable_files(target_folder, recursive)

    # Add detailed stats about found files if in debug mode
    if files and logger.isEnabledFor(logging.DEBUG):
        # Count files by directory to help with debugging
        dirs_count = {}
        for file_path in files:
            dir_path = os.path.dirname(file_path)
            if dir_path in dirs_count:
                dirs_count[dir_path] += 1
            else:
                dirs_count[dir_path] = 1

        # Log directories and file counts
        logger.debug("Files found by directory:")
        for dir_path, count in dirs_count.items():
            logger.debug(f"  - {dir_path}: {count} files")

    logger.debug(f"Found {len(files)} processable files total")

    # Load processed files log
    processed_files = load_processed_files()

    # Get Pinecone index if not dry run
    index = None if dry_run else get_pinecone_index()
    if not dry_run and index is None:
        logger.error("Failed to get Pinecone index. Cannot proceed with uploads.")
        return 0, 0

    total_files_processed = 0
    successfully_uploaded = 0

    # Process each file
    for file_path in files:
        file_name = os.path.basename(file_path)

        # Check if file has changed
        if not check_file_changed(file_path, file_name, processed_files):
            logger.debug(f"Skipping unchanged file: {file_name}")
            continue

        # Check if file needs preprocessing and user confirmation for large files
        should_process, _ = preprocess_file(file_path)
        if not should_process:
            logger.info(f"Skipping file due to preprocessing rules: {file_name}")
            continue

        total_files_processed += 1

        if dry_run:
            logger.info(f"DRY RUN: Would process file {file_path}")
            successfully_uploaded += 1

            # If specified, update the processed files log even in dry run
            if "--dry-run-update-cache" in os.sys.argv:
                processed_files[file_name] = {
                    "hash": generate_file_hash(file_path),
                    "mtime": os.path.getmtime(file_path),
                    "last_processed": datetime.now().isoformat(),
                }
        else:
            if upload_file_to_pinecone(file_path, index, CONFIG["namespace"]):
                successfully_uploaded += 1

                # Update the processed files log
                processed_files[file_name] = {
                    "hash": generate_file_hash(file_path),
                    "mtime": os.path.getmtime(file_path),
                    "last_processed": datetime.now().isoformat(),
                }

    # Save the processed files log
    save_processed_files(processed_files)

    # Log results
    logger.info("--- DOCUMENT PROCESSING SUMMARY ---")
    logger.info(f"Total files processed: {total_files_processed}")
    logger.info(f"Successfully uploaded: {successfully_uploaded}")
    if total_files_processed == successfully_uploaded:
        if total_files_processed > 0:
            logger.info("All files uploaded successfully!")
        else:
            logger.info("No files needed processing.")
    else:
        logger.warning("Some files failed to upload.")

    return total_files_processed, successfully_uploaded
