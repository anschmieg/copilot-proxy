import os

from dotenv import dotenv_values, load_dotenv

# Get existing environment variables before loading .env
existing_env = dict(os.environ)

# Load environment variables from .env file WITH override
dotenv_values_dict = dotenv_values(".env")
for key, value in dotenv_values_dict.items():
    os.environ[key] = value

# Now load with load_dotenv to ensure all variables are set properly
load_dotenv(override=True)

# Handle $HOME or ~/ expansions in TARGET_FOLDER
target_folder = os.getenv("TARGET_FOLDER", "~/Nextcloud/Documents")
# First expand any $HOME or $USER type variables
target_folder = os.path.expandvars(target_folder)
# Then expand any ~ character
target_folder = os.path.expanduser(target_folder)

# Configuration from .env file
CONFIG = {
    "TARGET_FOLDER": target_folder,
    "processed_log_path": os.path.expanduser(
        "~/Library/Application Support/PineconeDocProcessor/processed_files.json"
    ),
    "log_path": os.path.expanduser("~/Library/Logs/PineconeDocProcessor.log"),
    "namespace": os.getenv("NAMESPACE", ""),
    "index_name": os.getenv("INDEX_NAME", "personal-files"),
    "model_name": os.getenv("MODEL_NAME", "multilingual-e5-large"),
}

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
DOCUMENT_EXTENSIONS = [".pdf", ".doc", ".ppt", ".xls", ".docx", ".pptx", ".xlsx"]

# Combine all supported extensions
SUPPORTED_EXTENSIONS = TEXT_FILE_EXTENSIONS + DOCUMENT_EXTENSIONS
