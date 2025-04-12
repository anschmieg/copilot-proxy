import hashlib
import json
import os
from typing import Any, Dict

from assets.core.config import CONFIG
from assets.core.logging_setup import logger


def generate_file_hash(file_path: str) -> str:
    """Generate a SHA-256 hash for a given file."""
    hash_sha256 = hashlib.sha256()
    with open(file_path, "rb") as f:
        for chunk in iter(lambda: f.read(4096), b""):
            hash_sha256.update(chunk)
    return hash_sha256.hexdigest()


def load_processed_files() -> Dict[str, Dict[str, Any]]:
    """Load log of processed files, creating if not exists."""
    os.makedirs(os.path.dirname(CONFIG["processed_log_path"]), exist_ok=True)

    try:
        with open(CONFIG["processed_log_path"], "r") as f:
            return json.load(f)
    except (FileNotFoundError, json.JSONDecodeError):
        return {}


def save_processed_files(processed_files: Dict[str, Dict[str, Any]]):
    """Save log of processed files."""
    with open(CONFIG["processed_log_path"], "w") as f:
        json.dump(processed_files, f, indent=2)


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
