#!/usr/bin/env python3

"""
Troubleshoot environment variable issues, particularly the TARGET_FOLDER.
This script isolates the environment variable expansion to identify exactly
where the problem is occurring.
"""

import os
import sys
from pathlib import Path

from dotenv import dotenv_values


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


def main():
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
                    if line.strip() and not line.strip().startswith("#"):
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
    try:
        print("\n--- Checking CONFIG dictionary ---")
        from assets.core.config import CONFIG

        print(f"TARGET_FOLDER in CONFIG: {CONFIG['TARGET_FOLDER']}")
    except Exception as e:
        print(f"Error importing CONFIG: {e}")

    # Check Python version and environment
    print("\n--- Python Environment Info ---")
    print(f"Python version: {sys.version}")
    print(f"Current working directory: {os.getcwd()}")
    print(f"HOME environment variable: {os.getenv('HOME')}")
    print(f"USER environment variable: {os.getenv('USER')}")


if __name__ == "__main__":
    main()
