#!/usr/bin/env python3
"""
Script to create backward compatibility symlinks
from old script locations to the new structure.
"""

from pathlib import Path


def create_symlink(source, target):
    """Create a symlink from source to target."""
    source_path = Path(source)
    target_path = Path(target)

    # Ensure target directory exists
    target_dir = target_path.parent
    if not target_dir.exists():
        print(f"Creating directory: {target_dir}")
        target_dir.mkdir(parents=True, exist_ok=True)

    # Check if source exists
    if not source_path.exists():
        print(f"Warning: Source file doesn't exist: {source_path}")
        return False

    # Create symlink
    try:
        # Remove existing file or symlink
        if target_path.exists() or target_path.is_symlink():
            target_path.unlink()
            print(f"Removed existing file/symlink: {target_path}")

        # Create the symlink
        target_path.symlink_to(source_path)
        print(f"Created symlink: {target_path} -> {source_path}")
        return True
    except Exception as e:
        print(f"Error creating symlink {target_path}: {e}")
        return False


def main():
    """Create backward compatibility symlinks."""
    print("Setting up backward compatibility...")

    # Define current directory
    current_dir = Path(__file__).parent

    # Define mappings from new to old script locations
    symlinks = [
        # Map bin scripts to root directory
        (current_dir / "bin/process-docs.py", current_dir / "process-docs.py"),
        (current_dir / "bin/utils.py", current_dir / "utils.py"),
        # Provide compatibility for the old document_processor.py
        (
            current_dir / "assets/document_processor.py",
            current_dir / "document_processor.py",
        ),
        # Map new classes to old locations (assets structure)
        (
            current_dir / "rag_processor/core/config.py",
            current_dir / "assets/core/config.py",
        ),
        (
            current_dir / "rag_processor/core/file_utils.py",
            current_dir / "assets/core/file_utils.py",
        ),
        (
            current_dir / "rag_processor/core/logging_setup.py",
            current_dir / "assets/core/logging_setup.py",
        ),
        (
            current_dir / "rag_processor/processor/document_processor.py",
            current_dir / "assets/processors/document_processor.py",
        ),
        (
            current_dir / "rag_processor/processor/file_converter.py",
            current_dir / "assets/processors/file_converter.py",
        ),
        (
            current_dir / "rag_processor/processor/preprocessing.py",
            current_dir / "assets/processors/preprocessing.py",
        ),
        (
            current_dir / "rag_processor/pinecone/uploader.py",
            current_dir / "assets/pinecone/uploader.py",
        ),
    ]

    # Create each symlink
    success_count = 0
    for source, target in symlinks:
        if create_symlink(source, target):
            success_count += 1

    # Print summary
    print(
        f"\nCreated {success_count} of {len(symlinks)} symlinks for backward compatibility."
    )
    print("You can now use either the old or new script locations.")


if __name__ == "__main__":
    main()
