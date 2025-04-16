// Package copilotproxy provides an OpenAI-compatible API wrapper for the GitHub Copilot API.
//
// This application serves as a bridge between standard OpenAI API clients and GitHub Copilot's
// proprietary API, allowing users to leverage their existing GitHub Copilot subscription through
// familiar OpenAI-style interfaces.
//
// # Architecture Overview
//
// The application follows a clean architecture pattern with the following components:
//
//   - app: Core application logic and server implementation
//   - auth: Authentication and authorization mechanisms
//   - llm: Language model integration and API handling
//   - rpc: Remote procedure call handling for client-server communication
//   - models: Data structures for API requests and responses
//   - utils: Helper functions and utilities
//
// # Authentication System
//
// Multiple authentication methods are supported:
//  1. Direct Copilot API keys
//  2. GitHub OAuth tokens (automatically exchanged for Copilot API keys)
//  3. Local VS Code Copilot configuration (automatically detected)
//
// The application can also run with authentication disabled using the --disable-auth flag,
// which enables testing and development without requiring API keys or tokens.
// When running with --disable-auth:
//   - No API keys or tokens are required in requests
//   - All requests are processed with administrative privileges
//   - All OpenAI-compatible endpoints are available
//
// For production use, it's recommended to set the LLM_API_SECRET environment variable
// to a secure random value and require proper authentication.
//
// # Feature Highlights
//
//   - OpenAI-compatible endpoint for chat completions
//   - Support for streaming responses
//   - Automatic token refresh
//   - Rate limiting
//   - VS Code Copilot extension monitoring
//   - Comprehensive CLI options
//
// # Getting Started
//
// To use this package as a library:
//
//	import "github.com/anschmieg/copilot-proxy/internal/llm"
//
//	service := llm.NewCopilotService(config)
//	response, err := service.GetChatCompletions(request)
//
// To run as a standalone server:
//
//	go run cmd/main.go
//
// # API Reference
//
// For detailed API documentation, see docs/copilot-api.md
//
// # Version History
//
// Current API Version: 2025-04-01
//
// See CHANGELOG.md for version history details.
package copilotproxy
