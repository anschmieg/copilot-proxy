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
*/
package llm
