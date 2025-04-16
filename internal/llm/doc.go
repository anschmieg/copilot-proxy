/*
Package llm implements language model integration for various AI providers.

# Architecture Overview

The LLM package follows a layered architecture pattern:

1. HTTP Handlers (handlers.go)
  - Provide HTTP API endpoints for model listing and completion requests
  - Handle authentication, validation, and request routing
  - Convert between HTTP and internal data formats

2. Service Layer (service.go)
  - Contains business logic for working with language models
  - Manages rate limiting, provider selection, and token counting
  - Routes requests to appropriate provider APIs

3. Authorization (authorization.go)
  - Enforces access control based on user permissions
  - Handles geographical restrictions and rate limits
  - Manages subscription-based access to models

4. Configuration (config.go)
  - Manages API keys and provider settings
  - Controls which models are enabled
  - Sets default parameters and limits

5. Token Management (token.go)
  - Creates and validates JWT tokens for API authentication
  - Handles token encryption and signing
  - Manages token lifetime and expiration

# Integration Flow

The typical request flow through the system is:

1. HTTP request arrives at /completion or /models endpoint
2. Handler validates the JWT token and extracts claims
3. Authorization layer checks if the user can access the requested model
4. Service layer routes the request to the appropriate provider API
5. Response is streamed back to the client

# Provider Integration

The system supports multiple LLM providers:

- GitHub Copilot Chat API
- OpenAI API (for GPT models)
- Anthropic API (for Claude models)
- Google AI API (for Gemini models)

# GitHub Copilot Integration

For GitHub Copilot requests, the service:

1. Authenticates using one of three methods:
  - Direct Copilot API key (from environment variables)
  - GitHub OAuth token (exchanged for Copilot API key)
  - Local configuration (from VS Code extensions)

2. Formats requests to match Copilot API requirements with appropriate headers:
  - X-GitHub-API-Version: 2025-04-01
  - Editor-Version: vscode/1.99.2
  - Editor-Plugin-Version: copilot-chat/0.26.3

3. Handles token refreshing when the current token expires
  - Automatically retries with a fresh token on 401 responses
  - Maintains a token cache to minimize authentication overhead

4. Supports streaming responses with Server-Sent Events format
  - Converts between Copilot and OpenAI streaming formats
  - Handles backpressure and connection management

# Rate Limiting

Rate limiting occurs at multiple levels:

1. Per-user, per-minute request limits
2. Per-user, per-minute token limits (separate for input/output)
3. Per-user, per-day token limits
4. Dynamic adjustment based on active user counts

The rate limits are designed to:
- Prevent abuse and excessive usage
- Ensure fair resource allocation among users
- Manage costs associated with API usage
- Scale dynamically based on system load

# Subscription Management

The system supports different access levels:

- Free tier with basic access and limited usage
- Paid subscriptions with higher limits
- Staff access with unrestricted usage

Each level has configurable spending limits and model access permissions.

# Error Handling

The service implements robust error handling with:

1. Detailed logging of API errors
2. User-friendly error messages
3. Automatic retries for transient failures
4. Circuit breaking for persistent API issues
5. Graceful degradation when specific providers are unavailable

# Monitoring and Debugging

The service provides various monitoring capabilities:

1. Request/response logging (when enabled)
2. VS Code extension monitoring via the `--monitor-vscode` flag
3. Performance metrics collection
4. Token usage tracking
5. Error rate monitoring

# Version Compatibility

Current API version: 2025-04-01

See CHANGELOG.md for version history and compatibility information.
*/
package llm
