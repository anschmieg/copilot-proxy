import os
from datetime import datetime

from assets.core.logging_setup import logger
from assets.processors.file_converter import convert_document_to_text


def upload_file_to_pinecone(file_path: str, index, namespace: str = "") -> bool:
    """
    Upload a file to Pinecone with proper error handling.
    """
    try:
        # Basic file validation
        if not os.path.isfile(file_path):
            logger.error(f"File does not exist: {file_path}")
            return False

        # Read file with error handling
        try:
            file_text = convert_document_to_text(file_path)
            if not file_text:
                logger.error(f"No text extracted from {file_path}")
                return False
        except Exception as e:
            logger.error(f"Error reading file {file_path}: {e}")
            return False

        # For preprocessed files, use the original file metadata
        original_path = file_path
        if file_path.startswith(
            os.path.expanduser("~/Library/Caches/PineconeDocProcessor")
        ):
            # This is a preprocessed file, get original filename from it
            cache_filename = os.path.basename(file_path)
            # Get just the base name without extension
            base_name = os.path.splitext(cache_filename)[0]

            # For display purposes in logs
            logger.debug(f"Using preprocessed version of file: {base_name}")

            # Still use the preprocessed file's content but with original filename
            file_name = f"{base_name}.original"
        else:
            file_name = os.path.basename(file_path)

        # Get file metadata
        file_stats = os.stat(file_path)

        # Create record for Pinecone
        record = {
            "_id": file_name,
            "data": file_text,  # The field that will be embedded
            "file_path": original_path if "original_path" in locals() else file_path,
            "file_name": file_name,
            "file_extension": os.path.splitext(file_name)[1],
            "file_size": file_stats.st_size,
            "creation_date": datetime.fromtimestamp(file_stats.st_ctime).strftime(
                "%Y-%m-%d %H:%M:%S"
            ),
            "modification_date": datetime.fromtimestamp(file_stats.st_mtime).strftime(
                "%Y-%m-%d %H:%M:%S"
            ),
            "uploaded_date": datetime.now().strftime("%Y-%m-%d %H:%M:%S"),
            "preprocessed": file_path != original_path
            if "original_path" in locals()
            else False,
        }

        # Upsert to Pinecone
        index.upsert_records(namespace=namespace, records=[record])
        logger.info(f"Successfully uploaded {file_name} to Pinecone.")
        return True

    except Exception as e:
        logger.error(f"Error uploading {os.path.basename(file_path)}: {e}")
        return False
