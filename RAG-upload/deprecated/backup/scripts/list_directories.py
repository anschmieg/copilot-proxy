#!/usr/bin/env python3

"""
Utility script to list directories and files in the target folder for debugging.
"""

import argparse
import os

from dotenv import load_dotenv

# Force load environment variables from .env with priority over existing env vars
load_dotenv(override=True)

from assets.core.config import CONFIG, SUPPORTED_EXTENSIONS


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


def main():
    parser = argparse.ArgumentParser(
        description="List directories and files in target folder."
    )
    parser.add_argument(
        "--path",
        type=str,
        default=CONFIG["TARGET_FOLDER"],
        help="Path to examine (defaults to TARGET_FOLDER from .env)",
    )
    parser.add_argument(
        "--depth", type=int, default=3, help="Maximum depth to display (default: 3)"
    )
    parser.add_argument(
        "--files", action="store_true", help="Show files in addition to directories"
    )

    args = parser.parse_args()
    list_directories(args.path, args.depth, args.files)


if __name__ == "__main__":
    main()
