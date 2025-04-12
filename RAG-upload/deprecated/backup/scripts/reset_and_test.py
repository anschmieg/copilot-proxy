#!/usr/bin/env python3

"""
Script to reset environment variables and test the application's behavior
with a clean environment state.
"""

import os
import subprocess
import sys


def reset_and_run():
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

    # Get command to run from arguments
    if len(sys.argv) < 2:
        print("Usage: ./reset_and_test.py <command> [args...]")
        print(
            "Example: ./reset_and_test.py ./launchd-document-processor.py --debug-env"
        )
        return 1

    cmd = sys.argv[1:]
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


if __name__ == "__main__":
    sys.exit(reset_and_run())
