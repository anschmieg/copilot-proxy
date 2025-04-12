#!/bin/bash

# Create a virtual environment in a hidden folder
python3 -m venv .venv

# Activate the virtual environment
source .venv/bin/activate

# Install required Python packages
pip install -r assets/requirements.txt

# Install Poppler for PDF conversion
brew install poppler

# Create .env file **if it doesn't exist**
if [ ! -f .env ]; then
  echo "Creating .env file..."
  cat <<EOT >>.env
PINECONE_API_KEY=your_pinecone_api_key_here # TODO: Replace with actual Pinecone API key
TARGET_FOLDER=~/Cloud/
NAMESPACE= # Optional: Add a namespace for your Pinecone index
INDEX_NAME=personal-files
MODEL_NAME=multilingual-e5-large
EOT
  echo ".env file created. Please update it with your Pinecone API key and other configurations."
else
  echo ".env file already exists. Please ensure it contains the correct configurations."
fi

# Make the main script executable
chmod +x launchd-document-processor.py

# Copy launchd configuration
cp com.anschmieg.pinecone-doc-processor.plist ~/Library/LaunchAgents/

# Get the user ID
USER_ID=$(id -u)

# Unload any existing job
launchctl bootout gui/$USER_ID ~/Library/LaunchAgents/com.anschmieg.pinecone-doc-processor.plist

# Load the launchd job
launchctl bootstrap gui/$USER_ID ~/Library/LaunchAgents/com.anschmieg.pinecone-doc-processor.plist

echo "Setup complete. The document processor is now scheduled to run."
