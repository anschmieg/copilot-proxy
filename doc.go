// Package main provides a client for GitHub Copilot and other LLM services.
//
// This application serves as a proxy between users and various language model
// providers, handling authentication, authorization, rate limiting, and other
// access control features.
//
// Main components:
//   - LLM authorization and access control
//   - Token and request validation
//   - Provider-specific API integrations
//   - Automatic .env file loading
//
// Authentication:
//
// The application provides a flexible, prioritized authentication system for GitHub Copilot:
//
//  1. Direct API Key: Use COPILOT_API_KEY environment variable
//  2. OAuth Token: Use COPILOT_OAUTH_TOKEN or OAUTH_TOKEN environment variables
//  3. Local Config: Automatically read from GitHub Copilot local configuration
//
// The system will try these methods in order when authenticating with GitHub Copilot.
// It also checks for token expiration and automatically attempts to refresh expired tokens.
//
// Environment Files:
//
// The application automatically loads environment variables from .env files in:
//   - The current directory
//   - Parent directories (searching up to the root)
//
// This allows for easy configuration without manual environment variable management.
//
// CLI Usage:
//
// The application supports the following CLI flags for granular control:
//
//   - Retrieve an API key using an OAuth token:
//     ./coproxy --get-api-key="your-oauth-token"
//     ./coproxy --get-api-key   (auto-retrieves token from environment)
//
//   - Test the validity of an API key:
//     ./coproxy --test-auth="your-api-key"
//     ./coproxy --test-auth     (auto-retrieves key using prioritized approach)
//
//   - Make a test call to verify the API is working:
//     ./coproxy --test-call="test-payload"
//
//   - Disable API key validation (useful for development):
//     ./coproxy --disable-auth
//
// Environment Variables:
//
//   - VALID_API_KEYS: Comma-separated list of valid API keys for the proxy
//   - DISABLE_AUTH: Set to "true" or "1" to disable API key verification
//   - COPILOT_API_KEY: GitHub Copilot API token
//   - COPILOT_OAUTH_TOKEN: GitHub OAuth token to exchange for Copilot API key
//   - OAUTH_TOKEN: Alternative to COPILOT_OAUTH_TOKEN
//   - GITHUB_ACCESS_TOKEN: GitHub API token for additional functionality
//   - LLM_API_SECRET: Secret key for LLM API access
//   - STRIPE_API_KEY: Stripe API key for billing functionality
//
// Configuration Paths:
//
// The application looks for GitHub Copilot configuration in various standard locations:
//   - Windows: %APPDATA%\GitHub Copilot\apps.json
//   - macOS:
//   - ~/.config/github-copilot/apps.json
//   - ~/Library/Application Support/GitHub Copilot/apps.json
//   - ~/.vscode/extensions/github.copilot-*/config/apps.json
//   - Linux:
//   - ~/.config/github-copilot/apps.json
//   - ~/.vscode/extensions/github.copilot-*/config/apps.json
package main
