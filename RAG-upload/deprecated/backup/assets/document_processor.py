import os
from typing import Any, Dict, Optional, Tuple

import dotenv

from assets.core.config import CONFIG
from assets.core.file_utils import (
    generate_file_hash,
)
from assets.core.logging_setup import logger

# Import preprocessing functionality
from assets.processors.preprocessing import (
    check_file_size_threshold,
    needs_preprocessing,
    preprocess_with_pandoc,
)

# Load environment variables from .env file
dotenv.load_dotenv()

# Text file extensions to include
TEXT_FILE_EXTENSIONS = [
    ".txt",
    ".md",
    ".markdown",
    ".json",
    ".yaml",
    ".yml",
    ".csv",
    ".xml",
    ".html",
    ".htm",
]

# PDF and other document extensions
DOCUMENT_EXTENSIONS = [
    ".pdf",
    ".doc",
    ".ppt",
    ".xls",
    ".docx",
    ".pptx",
    ".xlsx",
    ".epub",
    ".odt",
    ".rtf",
]  # Added more formats that pandoc can handle

# Combine all supported extensions
SUPPORTED_EXTENSIONS = TEXT_FILE_EXTENSIONS + DOCUMENT_EXTENSIONS


def check_file_changed(
    file_path: str, filename: str, processed_files: Dict[str, Dict[str, Any]]
) -> bool:
    """
    Hybrid approach to detect if a file has changed:
    1. Check modification timestamp first (fast)
    2. If timestamp changed, verify with hash calculation (moderate)
    3. Return True only if content actually changed

    Returns:
        bool: True if file has changed or is new, False otherwise
    """
    # If file not in processed records, it's new
    if filename not in processed_files:
        logger.debug(f"New file detected: {filename}")
        return True

    # Get last modified time of file
    current_mtime = os.path.getmtime(file_path)

    # If we have stored the modification time previously
    if "mtime" in processed_files[filename]:
        stored_mtime = processed_files[filename]["mtime"]

        # If modification time hasn't changed, skip further checks
        if (
            abs(current_mtime - stored_mtime) < 0.001
        ):  # Small epsilon for float comparison
            logger.debug(f"File unchanged (timestamp match): {filename}")
            return False

        # Timestamp changed, now check hash (more expensive)
        logger.debug(f"Timestamp changed for {filename}, verifying with hash...")
        file_hash = generate_file_hash(file_path)

        if processed_files[filename].get("hash") == file_hash:
            # Update timestamp but mark as unchanged
            processed_files[filename]["mtime"] = current_mtime
            logger.debug(
                f"File unchanged (hash match despite timestamp change): {filename}"
            )
            return False

        # Both timestamp and hash differ, file has changed
        logger.debug(f"File changed (timestamp and hash differ): {filename}")
        return True

    # No timestamp record, fall back to hash check
    logger.debug(f"No timestamp record for {filename}, checking hash...")
    file_hash = generate_file_hash(file_path)

    if processed_files[filename].get("hash") == file_hash:
        # Add timestamp but mark as unchanged
        processed_files[filename]["mtime"] = current_mtime
        logger.debug(f"File unchanged (hash match): {filename}")
        return False

    # Hash differs, file has changed
    logger.debug(f"File changed (hash differs): {filename}")
    return True


def preprocess_file(file_path: str) -> Tuple[bool, Optional[str]]:
    """
    Preprocess a file before uploading:
    1. Check if file exceeds size threshold and ask for confirmation
    2. Convert file format if necessary with pandoc

    Args:
        file_path: Path to the file

    Returns:
        Tuple[bool, Optional[str]]: (should_process, preprocessed_file_path)
    """
    # Check file size and get user confirmation if needed
    exceeds_threshold, should_process = check_file_size_threshold(file_path)

    if not should_process:
        logger.info(f"Skipping {file_path} - user chose not to process large file")
        return False, None

    # Check if file needs preprocessing with pandoc
    if needs_preprocessing(file_path):
        preprocessed_path = preprocess_with_pandoc(file_path)
        if preprocessed_path:
            logger.debug(f"Using pandoc-preprocessed version of {file_path}")
            return True, preprocessed_path
        else:
            logger.warning(
                f"Pandoc preprocessing failed for {file_path}, using original file"
            )

    # No preprocessing needed or preprocessing failed, use original file
    return True, None


def process_documents(
    target_folder=None, dry_run=False, recursive=True
) -> Tuple[int, int]:
    """
    Process documents in the target folder.

    Args:
        target_folder: Path to the folder containing documents. If None, use CONFIG["TARGET_FOLDER"].
        dry_run: If True, don't actually upload any documents.
        recursive: If True, process documents in subdirectories recursively.

    Returns:
        Tuple[int, int]: (total_files_processed, successfully_uploaded)
    """
    if target_folder is None:
        # Use CONFIG value which has already been properly expanded
        target_folder = CONFIG["TARGET_FOLDER"]
    else:
        # Make sure any provided path is properly expanded
        target_folder = os.path.expandvars(os.path.expanduser(target_folder))

    logger.debug(f"Using target folder: {target_folder}")
    logger.debug(f"Recursive mode: {recursive}")

    # Verify the target folder exists before proceeding
    if not os.path.exists(target_folder):
        logger.error(f"Target folder does not exist: {target_folder}")
        return 0, 0

    if not os.path.isdir(target_folder):
        logger.error(f"Target path is not a directory: {target_folder}")
        return 0, 0

    from assets.processors.document_processor import process_document_folder

    return process_document_folder(target_folder, dry_run, recursive)


def upload_file_to_pinecone(file_path: str, index, namespace: str = "") -> bool:
    """
    Upload a file to Pinecone with preprocessing.
    """
    # Preprocess the file first
    should_process, preprocessed_path = preprocess_file(file_path)

    if not should_process:
        return False

    # Use the preprocessed file if available, otherwise use the original
    path_to_use = preprocessed_path if preprocessed_path else file_path

    # Import here to avoid circular imports
    from assets.pinecone.uploader import upload_file_to_pinecone as uploader_function

    # Upload the file
    result = uploader_function(path_to_use, index, namespace)

    # Clean up preprocessed file if it was temporary
    if preprocessed_path and os.path.exists(preprocessed_path):
        try:
            # Keep the preprocessed file for caching purposes
            pass
            # If you want to delete temporary files, uncomment the line below:
            # os.remove(preprocessed_path)
        except Exception as e:
            logger.warning(
                f"Failed to clean up temporary file {preprocessed_path}: {e}"
            )

    return result
