// Package main provides a client for GitHub Copilot and other LLM services.
//
// # GitHub Copilot API Integration
//
// This application enables interaction with the GitHub Copilot API through several
// authentication methods and provides proper request formatting to successfully
// communicate with the Copilot service.
//
// # API Endpoints
//
// The GitHub Copilot API exposes several endpoints:
//
//   - Chat Completions: https://api.githubcopilot.com/chat/completions
//     The main endpoint for chat and code completions
//
//   - Token Exchange: https://api.github.com/copilot_internal/v2/token
//     Used to exchange a GitHub OAuth token for a Copilot API token
//
// # Authentication
//
// The application supports a prioritized authentication system:
//
//  1. Direct API Key: Use COPILOT_API_KEY environment variable
//  2. OAuth Token: Use COPILOT_OAUTH_TOKEN or OAUTH_TOKEN environment variables
//  3. Local Config: Automatically read from GitHub Copilot local configuration
//
// # Copilot API Token Format
//
// The Copilot API token format is:
// tid=token_id;exp=expiration_timestamp;sku=subscription_type;proxy-ep=endpoint;st=status;
// followed by various feature flags like chat=1;cit=1;etc.
//
// # Required Headers
//
// The GitHub Copilot API requires specific headers to function properly:
//
//   - Authorization: Bearer {COPILOT_API_TOKEN}
//   - Content-Type: application/json
//   - Editor-Version: Editor identifier (e.g., "vscode/1.99.2")
//   - Editor-Plugin-Version: Plugin version (e.g., "copilot-chat/0.26.3")
//   - Copilot-Integration-ID: Integration identifier (e.g., "vscode-chat")
//   - User-Agent: Client identifier (e.g., "GitHubCopilotChat/0.26.3")
//   - OpenAI-Intent: Purpose of the request (e.g., "conversation-agent")
//   - X-GitHub-API-Version: API version (e.g., "2025-04-01")
//
// # Request Format
//
// The chat completions API expects a JSON request body with:
//
//   - messages: Array of message objects with role and content
//   - model: The model to use (e.g., "gpt-4o")
//   - temperature: Controls randomness (0.0-1.0)
//   - top_p: Controls diversity via nucleus sampling
//   - max_tokens: Maximum tokens to generate
//   - tools: Optional array of tools the model can use
//   - stream: Boolean for streaming responses
//
// Example request body:
//
//	{
//	  "messages": [
//	    {"role": "system", "content": "You are a helpful assistant."},
//	    {"role": "user", "content": "Write a Go function to reverse a string."}
//	  ],
//	  "model": "gpt-4o",
//	  "temperature": 0,
//	  "top_p": 1,
//	  "max_tokens": 4096
//	}
//
// # Rate Limits
//
// GitHub Copilot implements rate limiting based on:
//   - Requests per minute
//   - Tokens per minute
//   - Tokens per day
//
// The application implements token bucket algorithm for rate limiting to manage usage.
//
// # Environment Variables
//
//   - COPILOT_API_KEY: GitHub Copilot API token
//   - COPILOT_OAUTH_TOKEN: GitHub OAuth token to exchange for a Copilot API key
//   - OAUTH_TOKEN: Alternative to COPILOT_OAUTH_TOKEN
//   - GITHUB_ACCESS_TOKEN: GitHub API token for additional functionality
//   - EDITOR_VERSION: Editor identifier for API requests (e.g., "vscode/1.99.2")
//   - EDITOR_PLUGIN_VERSION: Plugin version for API requests (e.g., "copilot-chat/0.26.3")
//
// For more details, run the application with the --help flag or refer to the README.md.
package main
