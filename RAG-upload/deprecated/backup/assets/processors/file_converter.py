import os
import subprocess

from assets.core.config import TEXT_FILE_EXTENSIONS
from assets.core.logging_setup import logger


def convert_document_to_text(file_path: str) -> str:
    """
    Convert various document types to plain text using macOS native tools.

    Uses:
    - pdftotext for PDFs
    - textutil for Microsoft Office formats
    - cat for plain text files
    """
    ext = os.path.splitext(file_path)[1].lower()

    try:
        if ext == ".pdf":
            # Use pdftotext (from Poppler, installable via Homebrew)
            result = subprocess.run(
                ["pdftotext", file_path, "-"],
                capture_output=True,
                text=True,
                check=True,
            )
            return result.stdout

        elif ext in [".docx", ".pptx", ".xlsx"]:
            # Use textutil for Office formats
            result = subprocess.run(
                ["textutil", "-convert", "txt", "-stdout", file_path],
                capture_output=True,
                text=True,
                check=True,
            )
            return result.stdout

        elif ext in TEXT_FILE_EXTENSIONS:
            # Simply read plain text files
            with open(file_path, "r", encoding="utf-8") as f:
                return f.read()

        else:
            logger.warning(f"Unsupported file type: {file_path}")
            return ""

    except subprocess.CalledProcessError as e:
        logger.error(f"Conversion error for {file_path}: {e}")
        return ""
    except Exception as e:
        logger.error(f"Unexpected error converting {file_path}: {e}")
        return ""
