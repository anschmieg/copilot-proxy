import json
import os
import subprocess
from typing import Any, Dict, Optional, Tuple

from assets.core.config import CONFIG
from assets.core.logging_setup import logger

# Define size thresholds for large files (in bytes)
# 5MB default threshold for large files
DEFAULT_LARGE_FILE_THRESHOLD = 5 * 1024 * 1024

# File type specific thresholds can be defined here
FILE_SIZE_THRESHOLDS = {
    ".pdf": 10 * 1024 * 1024,  # 10MB for PDFs
    ".docx": 8 * 1024 * 1024,  # 8MB for Word documents
    ".pptx": 15 * 1024 * 1024,  # 15MB for PowerPoint
}

# File types that need pandoc preprocessing
PANDOC_FORMATS = [".docx", ".pptx", ".epub", ".odt", ".rtf"]

# Path to store preprocessed files
CACHE_DIR = os.path.expanduser("~/Library/Caches/PineconeDocProcessor")


def get_user_confirmation(file_path: str, file_size: int) -> bool:
    """
    Display a dialog to the user asking for confirmation to process a large file.

    Args:
        file_path: Path to the file
        file_size: Size of the file in bytes

    Returns:
        bool: True if user confirms, False otherwise
    """
    file_name = os.path.basename(file_path)
    size_mb = file_size / (1024 * 1024)

    # Create the AppleScript dialog command
    apple_script = f"""
    tell application "System Events"
        activate
        set question to "The file '{file_name}' is {size_mb:.2f} MB in size. Processing large files may take a while."
        set theResponse to display dialog question buttons {{"Cancel", "Process Anyway"}} default button 2 with icon caution
        set theButton to button returned of theResponse
        if theButton is "Process Anyway" then
            return "confirmed"
        else
            return "cancelled"
        end if
    end tell
    """

    try:
        # Run the AppleScript and get the result
        result = subprocess.run(
            ["osascript", "-e", apple_script],
            capture_output=True,
            text=True,
            check=True,
        )

        # Check the result
        if "confirmed" in result.stdout:
            logger.info(f"User confirmed processing of large file: {file_name}")
            return True
        else:
            logger.info(f"User cancelled processing of large file: {file_name}")
            return False
    except subprocess.SubprocessError as e:
        logger.error(f"Error displaying dialog for {file_path}: {e}")
        # Default to cancel if dialog fails
        return False


def load_user_preferences() -> Dict[str, Any]:
    """Load user preferences for file processing."""
    prefs_path = os.path.join(
        os.path.dirname(CONFIG["processed_log_path"]), "user_preferences.json"
    )

    try:
        with open(prefs_path, "r") as f:
            return json.load(f)
    except (FileNotFoundError, json.JSONDecodeError):
        return {}


def save_user_preferences(preferences: Dict[str, Any]):
    """Save user preferences for file processing."""
    prefs_path = os.path.join(
        os.path.dirname(CONFIG["processed_log_path"]), "user_preferences.json"
    )
    os.makedirs(os.path.dirname(prefs_path), exist_ok=True)

    with open(prefs_path, "w") as f:
        json.dump(preferences, f, indent=2)


def check_file_size_threshold(file_path: str) -> Tuple[bool, bool]:
    """
    Check if a file exceeds the size threshold and needs confirmation.

    Args:
        file_path: Path to the file

    Returns:
        Tuple[bool, bool]: (exceeds_threshold, should_process)
    """
    # Get file extension for specific thresholds
    file_ext = os.path.splitext(file_path)[1].lower()
    file_size = os.path.getsize(file_path)
    file_name = os.path.basename(file_path)

    # Get threshold for this file type, or use default
    threshold = FILE_SIZE_THRESHOLDS.get(file_ext, DEFAULT_LARGE_FILE_THRESHOLD)

    # Check if file exceeds threshold
    if file_size > threshold:
        # Load saved preferences
        preferences = load_user_preferences()
        large_files = preferences.get("large_files", {})

        # Check if user has already made a decision for this file
        if file_name in large_files:
            file_data = large_files[file_name]
            # If file size or hash changed significantly, ask again
            if abs(file_data.get("size", 0) - file_size) > (
                file_size * 0.1
            ):  # 10% change
                should_process = get_user_confirmation(file_path, file_size)
                # Update preference with new decision
                large_files[file_name] = {"size": file_size, "process": should_process}
                preferences["large_files"] = large_files
                save_user_preferences(preferences)
                return True, should_process
            else:
                # Use saved preference
                return True, file_data.get("process", False)
        else:
            # No saved preference, ask user
            should_process = get_user_confirmation(file_path, file_size)
            # Save user preference
            large_files[file_name] = {"size": file_size, "process": should_process}
            preferences["large_files"] = large_files
            save_user_preferences(preferences)
            return True, should_process

    # File does not exceed threshold
    return False, True


def preprocess_with_pandoc(file_path: str) -> Optional[str]:
    """
    Convert document to plain text using pandoc.

    Args:
        file_path: Path to the file

    Returns:
        Optional[str]: Path to preprocessed file or None if failed
    """
    file_ext = os.path.splitext(file_path)[1].lower()
    file_name = os.path.basename(file_path)

    # Check if this format needs pandoc
    if file_ext not in PANDOC_FORMATS:
        return None

    # Create cache directory if it doesn't exist
    os.makedirs(CACHE_DIR, exist_ok=True)

    # Generate cache file path
    cache_file_base = file_name.replace(file_ext, "")
    cache_file_path = os.path.join(CACHE_DIR, f"{cache_file_base}.txt")

    try:
        # Run pandoc to convert the file
        cmd = ["pandoc", file_path, "-o", cache_file_path, "--quiet"]
        subprocess.run(cmd, check=True)

        logger.debug(f"Successfully preprocessed {file_path} with pandoc")
        return cache_file_path
    except subprocess.SubprocessError as e:
        logger.error(f"Pandoc preprocessing failed for {file_path}: {e}")
        return None
    except Exception as e:
        logger.error(f"Unexpected error during pandoc preprocessing: {e}")
        return None


def needs_preprocessing(file_path: str) -> bool:
    """Check if a file needs preprocessing with pandoc."""
    file_ext = os.path.splitext(file_path)[1].lower()
    return file_ext in PANDOC_FORMATS
