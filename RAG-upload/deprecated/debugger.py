#!/usr/bin/env python3
"""
Consolidated debugging tools for RAG Processor
"""

import argparse
import os
import subprocess
import sys
from pathlib import Path

# Add parent directory to path for imports
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "..")))

from dotenv import dotenv_values, load_dotenv

# Import after path setup
from rag_processor.core.config import CONFIG, SUPPORTED_EXTENSIONS

# Remove circular import (importing from self)
from rag_processor.core.logging_setup import check_config


# Define debug_environment function that was being imported circularly
def debug_environment():
    """Print debug information about environment variables and system paths."""
    print("\n--- Environment Debug Information ---")

    # Python version and executable
    print(f"Python version: {sys.version}")
    print(f"Python executable: {sys.executable}")

    # Current working directory
    print(f"Current working directory: {os.getcwd()}")

    # Module search paths
    print("Python module search paths:")
    for path in sys.path:
        print(f"  - {path}")

    # Environment variables
    print("Environment variables related to RAG:")
    relevant_vars = [
        "TARGET_FOLDER",
        "OPENAI_API_KEY",
        "MONGODB_URI",
        "NAMESPACE",
        "INDEX_NAME",
        "VECTOR_STORE_TYPE",
    ]

    for var in relevant_vars:
        value = os.environ.get(var, "Not set")
        # Mask sensitive values
        if "KEY" in var or "URI" in var:
            if value != "Not set":
                value = (
                    value[:5] + "..." + value[-5:] if len(value) > 10 else "********"
                )
        print(f"  - {var}: {value}")

    print("=============================")


def check_env_conflicts():
    """Check for conflicts between .env file and environment variables."""
    print("\n--- Checking for Environment Variable Conflicts ---")

    # Get environment variables from .env file
    env_file = Path(".env")
    if not env_file.exists():
        print("No .env file found, skipping conflict check.")
        return

    dotenv_vars = dotenv_values(".env")

    # Check each variable in .env against current environment
    for key, dotenv_value in dotenv_vars.items():
        env_value = os.environ.get(key)
        if env_value is not None and env_value != dotenv_value:
            print(f"CONFLICT for {key}:")
            print(f"  - Value in .env file: {dotenv_value}")
            print(f"  - Value in environment: {env_value}")
            print(
                f"  - .env file is {'OVERRIDING' if os.getenv(key) == dotenv_value else 'NOT OVERRIDING'} environment"
            )
        else:
            if env_value is not None:
                print(f"✓ {key}: Same value in .env and environment ({env_value})")
            else:
                print(f"✓ {key}: Set only in .env file ({dotenv_value})")


def expand_with_details(env_var_name):
    """Expand an environment variable with detailed steps for debugging."""
    raw_value = os.getenv(env_var_name)
    print(f"1. Raw value from os.getenv('{env_var_name}'): {raw_value}")

    if raw_value is None:
        print(f"ERROR: Environment variable {env_var_name} is not set!")
        return None

    # Expand variables like $HOME
    expanded_vars = os.path.expandvars(raw_value)
    print(f"2. After os.path.expandvars(): {expanded_vars}")

    # Expand ~ character
    expanded_user = os.path.expanduser(expanded_vars)
    print(f"3. After os.path.expanduser(): {expanded_user}")

    # Final expansion
    normalized = os.path.normpath(expanded_user)
    print(f"4. Final normalized path: {normalized}")

    # Check if path exists
    if os.path.exists(normalized):
        print(f"✓ Path exists: {normalized}")
        try:
            print(f"   Is directory: {os.path.isdir(normalized)}")
            if os.path.isdir(normalized):
                files = os.listdir(normalized)[:5]
                print(f"   Sample contents: {files}")
        except Exception as e:
            print(f"   Error accessing path: {e}")
    else:
        print(f"✗ Path does NOT exist: {normalized}")

    return normalized


def reset_and_run_command(args):
    """Reset critical environment variables and run the specified command."""
    print("\n===== RESETTING ENVIRONMENT VARIABLES AND RUNNING TEST =====\n")

    # Store original environment variables to restore later
    original_env = {}
    env_to_reset = ["TARGET_FOLDER"]

    for var in env_to_reset:
        if var in os.environ:
            original_env[var] = os.environ[var]
            print(f"Temporarily unsetting {var}={os.environ[var]}")
            del os.environ[var]
        else:
            print(f"Variable {var} not set in environment")

    cmd = args.command
    cmd_str = " ".join(cmd)

    try:
        print(f"\n--- Running command with clean environment: {cmd_str} ---\n")
        # Run the command with the current Python executable
        result = subprocess.run([sys.executable] + cmd, check=True)
        exit_code = result.returncode
    except subprocess.CalledProcessError as e:
        print(f"\nCommand failed with exit code {e.returncode}")
        exit_code = e.returncode
    finally:
        # Restore environment variables
        for var, value in original_env.items():
            print(f"Restoring {var}={value}")
            os.environ[var] = value

    print("\n--- Environment reset to original state ---")
    return exit_code


def count_supported_files(directory_path, max_depth=None, current_depth=0):
    """Count files with supported extensions in the given directory."""
    supported_count = 0
    directories = []

    try:
        with os.scandir(directory_path) as entries:
            for entry in entries:
                if entry.is_file():
                    file_extension = os.path.splitext(entry.name)[1].lower()
                    if file_extension in SUPPORTED_EXTENSIONS:
                        supported_count += 1
                elif entry.is_dir() and not entry.name.startswith("."):
                    directories.append(entry.path)
    except PermissionError:
        print(f"Permission denied: {directory_path}")
        return supported_count, []

    if max_depth is None or current_depth < max_depth:
        for subdir in directories:
            sub_count, _ = count_supported_files(
                subdir, max_depth, current_depth + 1 if max_depth else None
            )
            supported_count += sub_count

    return supported_count, directories


def list_directories(root_path, max_depth=None, show_files=False):
    """List directories and optionally files in the root path."""
    print(f"Target folder: {root_path}")
    print("=" * 60)

    if not os.path.exists(root_path):
        print(f"ERROR: Path does not exist: {root_path}")
        return

    if not os.path.isdir(root_path):
        print(f"ERROR: Not a directory: {root_path}")
        return

    def _list_dir_contents(path, depth=0, max_depth=None):
        if max_depth is not None and depth > max_depth:
            return

        prefix = "  " * depth
        try:
            entries = list(os.scandir(path))

            # Count files with supported extensions
            supported_files = []
            dirs = []

            for entry in entries:
                if entry.is_file() and not entry.name.startswith("."):
                    file_extension = os.path.splitext(entry.name)[1].lower()
                    if file_extension in SUPPORTED_EXTENSIONS:
                        supported_files.append(entry.name)
                elif entry.is_dir() and not entry.name.startswith("."):
                    dirs.append(entry)

            # Print directory with supported file count
            if supported_files or dirs:
                print(
                    f"{prefix}📁 {os.path.basename(path)}/  ({len(supported_files)} supported files)"
                )

            # Print files if requested
            if show_files and supported_files:
                for file_name in sorted(supported_files):
                    print(f"{prefix}  📄 {file_name}")

            # Process subdirectories
            for dir_entry in sorted(dirs, key=lambda d: d.name):
                _list_dir_contents(dir_entry.path, depth + 1, max_depth)

        except PermissionError:
            print(f"{prefix}❌ {os.path.basename(path)}/ (Permission denied)")
        except Exception as e:
            print(f"{prefix}❌ {os.path.basename(path)}/ (Error: {e})")

    # Get total stats first
    total_supported, _ = count_supported_files(root_path)
    print(f"Total supported files (recursively): {total_supported}")
    print("-" * 60)

    # List directory structure
    _list_dir_contents(root_path, max_depth=max_depth)


def run_env_diagnostics():
    """Run environment diagnostics (like the former troubleshoot_env.py)"""
    print("\n===== ENVIRONMENT VARIABLE EXPANSION TROUBLESHOOTING =====\n")

    # Check for environment variable conflicts first
    check_env_conflicts()

    # Check .env file
    env_path = Path(".env")
    if env_path.exists():
        print(f".env file found at: {env_path.absolute()}")
        try:
            with open(env_path, "r") as f:
                contents = f.read()
                print("Contents of .env file:")
                for line in contents.splitlines():
                    if (
                        line.strip()
                        and not line.strip().startswith("#")
                        and not line.strip().startswith("//")
                    ):
                        if "API_KEY" in line or "SECRET" in line:
                            key_part = line.split("=")[0]
                            print(f"{key_part}=****HIDDEN****")
                        else:
                            print(line)
        except Exception as e:
            print(f"Error reading .env file: {e}")
    else:
        print(f"WARNING: No .env file found at {env_path.absolute()}")

    print("\n--- Expanding TARGET_FOLDER ---")
    _target_folder = expand_with_details("TARGET_FOLDER")

    # Check the CONFIG from our application
    check_config()

    # Check Python version and environment
    print("\n--- Python Environment Info ---")
    print(f"Python version: {sys.version}")
    print(f"Current working directory: {os.getcwd()}")
    print(f"HOME environment variable: {os.getenv('HOME')}")
    print(f"USER environment variable: {os.getenv('USER')}")


def validate_database_integrity(args):
    """Run validation of database against processed files tracking."""
    print("\n===== DATABASE INTEGRITY VALIDATION =====\n")

    # Use the new Assistant validator instead of Pinecone Vector DB
    from rag_processor.assistant.validator import (
        cleanup_tracking_json,
        find_untracked_assistant_files,
        validate_assistant_integrity,
    )

    print("Checking Pinecone Assistant files")

    # Check for files missing from Assistant
    missing_files, _ = validate_assistant_integrity(auto_reupload=False)

    if missing_files:
        print(
            f"\nFound {len(missing_files)} files that are in tracking JSON but missing from Pinecone Assistant:"
        )
        for file in missing_files[
            :10
        ]:  # Show first 10 files only to avoid flooding output
            print(f" - {os.path.basename(file)}")

        if len(missing_files) > 10:
            print(f" ... and {len(missing_files) - 10} more files")

        if args.reupload:
            print("\nRe-uploading missing files...")
            _, reuploaded = validate_assistant_integrity(auto_reupload=True)
            print(f"Re-uploaded {reuploaded} of {len(missing_files)} files")
    else:
        print("\n✓ All tracked files are present in Pinecone Assistant")

    # Check for files missing from disk
    if args.check_disk:
        print("\nChecking for tracked files missing from disk...")
        missing_count = cleanup_tracking_json(remove_missing=args.clean)

        if missing_count > 0:
            print(
                f"Found {missing_count} files in tracking JSON that don't exist on disk"
            )
            if args.clean:
                print(f"Removed {missing_count} entries from tracking JSON")
            else:
                print("Use --clean to remove these entries from tracking JSON")
        else:
            print("✓ All tracked files exist on disk")

    # Check for untracked Assistant files
    if args.check_untracked:
        print("\nChecking for untracked Pinecone Assistant files...")
        untracked = find_untracked_assistant_files()

        if untracked:
            print(
                f"Found {len(untracked)} files in Pinecone Assistant that aren't in tracking JSON"
            )
            for file_id, file_data in list(untracked.items())[:10]:  # Show first 10
                print(f" - {file_data.get('name', file_id)}")

            if len(untracked) > 10:
                print(f" ... and {len(untracked) - 10} more files")
        else:
            print("✓ All Pinecone Assistant files are tracked in JSON")


def main():
    parser = argparse.ArgumentParser(description="Debugging tools for RAG Processor")
    subparsers = parser.add_subparsers(dest="command", help="Command to run")

    # Environment diagnostics command
    subparsers.add_parser("env", help="Run environment diagnostics")

    # List directories command
    list_parser = subparsers.add_parser("list", help="List directories and files")
    list_parser.add_argument(
        "--path",
        type=str,
        default=None,
        help="Path to examine (defaults to TARGET_FOLDER from CONFIG)",
    )
    list_parser.add_argument(
        "--depth", type=int, default=3, help="Maximum depth to display (default: 3)"
    )
    list_parser.add_argument(
        "--files", action="store_true", help="Show files in addition to directories"
    )

    # Reset and run command
    reset_parser = subparsers.add_parser(
        "reset", help="Reset environment and run a command"
    )
    reset_parser.add_argument(
        "command", nargs="+", help="Command to run with clean environment"
    )

    # Add database validation command
    validate_parser = subparsers.add_parser(
        "validate", help="Validate database against processed files tracking"
    )
    validate_parser.add_argument(
        "--reupload", action="store_true", help="Re-upload missing files"
    )
    validate_parser.add_argument(
        "--check-disk", action="store_true", help="Check if tracked files exist on disk"
    )
    validate_parser.add_argument(
        "--clean",
        action="store_true",
        help="Remove entries from tracking JSON for files that don't exist on disk",
    )
    validate_parser.add_argument(
        "--check-untracked",
        action="store_true",
        help="Check for documents in Pinecone that aren't in tracking JSON",
    )

    args = parser.parse_args()

    # Load environment variables from .env with priority
    load_dotenv(override=True)

    if args.command == "env" or args.command is None:
        run_env_diagnostics()
    elif args.command == "list":
        path = args.path if args.path else CONFIG["TARGET_FOLDER"]
        list_directories(path, args.depth, args.files)
    elif args.command == "reset":
        reset_and_run_command(args)
    elif args.command == "validate":
        validate_database_integrity(args)
    else:
        parser.print_help()


if __name__ == "__main__":
    main()
