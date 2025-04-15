# GitHub Copilot API Documentation

This document outlines the GitHub Copilot API format, headers, authentication, and response patterns based on our mitmproxy capture analysis.

## API Endpoints

### Token Retrieval
- **URL**: `https://api.github.com/copilot_internal/v2/token`
- **Method**: GET
- **Headers**:
  - `Authorization: token <GITHUB_OAUTH_TOKEN>`
  - `Accept: application/json`
  - `User-Agent: copilot-proxy` (or your application name)
- **Response**: JSON containing a Copilot API token

### Chat Completions
- **URL**: `https://api.githubcopilot.com/chat/completions`
- **Method**: POST
- **Headers**: See "Required Headers" section below

## Authentication

GitHub Copilot uses a specialized token format. The token has the following structure:

```
tid=<token-id>;exp=<expiration-timestamp>;sku=<subscription-type>;proxy-ep=<proxy-endpoint>;st=<status>;<feature-flags>
```

Components:
- `tid`: Token ID
- `exp`: Expiration timestamp (Unix format)
- `sku`: Subscription type (e.g., "free_educational")
- `proxy-ep`: Proxy endpoint for API calls
- Feature flags: Various flags like `chat=1;cit=1;malfil=1`

### Token Retrieval Process

1. **OAuth Token**: Obtain a GitHub OAuth token with the necessary scopes
2. **Exchange for Copilot Token**: Call the token endpoint with the OAuth token
3. **Use in API Calls**: Include the token in the Authorization header (Bearer format)

### Token Verification

Tokens can be verified by checking:
1. Token format (starts with "tid=")
2. Expiration timestamp (compared to current time)

## Required Headers

For GitHub Copilot Chat API calls, the following headers are required:

```
Authorization: Bearer <COPILOT_API_TOKEN>
Content-Type: application/json
Editor-Version: vscode/1.99.2
Editor-Plugin-Version: copilot-chat/0.26.3
Copilot-Integration-ID: vscode-chat
User-Agent: GitHubCopilotChat/0.26.3
OpenAI-Intent: conversation-agent
X-GitHub-API-Version: 2025-04-01
X-Initiator: user
X-Interaction-Type: conversation-agent
X-Request-ID: <UNIQUE_REQUEST_ID>
```

Optional VS Code specific headers:
```
Vscode-Machineid: <MACHINE_ID>
Vscode-Sessionid: <SESSION_ID>
```

## Request Format

GitHub Copilot Chat API uses a format similar to OpenAI's API:

```json
{
  "model": "gpt-4o",
  "messages": [
    {"role": "user", "content": "Write a Go function to reverse a string"}
  ],
  "temperature": 0,
  "top_p": 1,
  "max_tokens": 4096
}
```

Default parameters if not specified:
- `model`: "gpt-4o"
- `temperature`: 0
- `top_p`: 1
- `max_tokens`: 4096

## Response Format

The response follows a structure similar to OpenAI's API:

```json
{
  "id": "chatcmpl-123",
  "object": "chat.completion",
  "created": 1725443708,
  "model": "gpt-4o",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "```go\nfunc reverseString(s string) string {\n\trunes := []rune(s)\n\tfor i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {\n\t\trunes[i], runes[j] = runes[j], runes[i]\n\t}\n\treturn string(runes)\n}\n```\n\nThis Go function takes a string as input and returns its reverse. Here's how it works:\n\n1. It converts the string to a slice of runes to properly handle Unicode characters\n2. It uses two pointers (i from start, j from end) to swap characters\n3. It continues swapping until the pointers meet in the middle\n4. Finally, it converts the rune slice back to a string and returns it"
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 12,
    "completion_tokens": 289,
    "total_tokens": 301
  }
}
```

## Error Handling

Common error responses:
- 401: Unauthorized (invalid or expired token)
- 429: Rate limited 
- 500: Server error

Error response format:
```json
{
  "error": {
    "message": "Error message",
    "type": "error_type",
    "code": "error_code"
  }
}
```

## Implementation Best Practices

1. **Token Management**:
   - Store tokens securely
   - Refresh when expired
   - Use a prioritized approach (direct API key, OAuth token, local config)

2. **Header Consistency**:
   - Always include all required headers
   - Maintain consistent editor and plugin versions

3. **Request Formatting**:
   - Follow OpenAI-compatible format
   - Include reasonable defaults for missing parameters

4. **Error Handling**:
   - Implement retry logic for rate limits
   - Provide clear error messages to users

## Environment Variables

Configure your application with:

- `COPILOT_API_KEY`: Direct API key (if available)
- `COPILOT_OAUTH_TOKEN` or `OAUTH_TOKEN`: GitHub OAuth token
- `EDITOR_VERSION`: Editor identifier (e.g., "vscode/1.99.2")
- `EDITOR_PLUGIN_VERSION`: Plugin version (e.g., "copilot-chat/0.26.3")
- `VSCODE_MACHINE_ID`: VS Code machine identifier
- `VSCODE_SESSION_ID`: VS Code session identifier