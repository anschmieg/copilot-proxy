# GitHub Copilot Client for Go

A Go application that enables you to use the GitHub Copilot Chat API like any other OpenAI-compatible model. This client integrates with your local GitHub Copilot configuration to provide AI completions through a convenient API.

## Documentation

For detailed documentation on the application architecture, configuration options, authorization system, and API details, run:

```bash
go doc
```

For package-specific documentation:

```bash
go doc ./internal/llm
go doc ./pkg/models
```

For documentation on specific functions:

```bash
go doc ./internal/llm.AuthorizeAccessToModel
```

## Project Structure

```
copilot-proxy
├── cmd
│   └── main.go          # Entry point of the application
├── pkg                  # Public packages
├── internal             # Internal implementation details
│   ├── app              # Core application logic
│   ├── auth             # Authentication functionality
│   ├── llm              # Language model integration
│   ├── rpc              # RPC connection handling
│   ├── user_backfiller.go
│   └── stripe_billing.go
├── go.mod               # Module definition
└── README.md            # Project documentation
```

## Getting Started

### Prerequisites

- Go 1.18 or later
- A GitHub account with active Copilot subscription
- [Optional] API keys for other LLM providers if you want to use them

### Installation

1. Clone the repository:
   
   ```
   git clone https://github.com/anschmieg/copilot-proxy.git
   cd copilot-proxy
   ```

2. Install dependencies:
   
   ```
   go mod tidy
   ```

3. Build the application:
   
   ```
   go build -o coproxy cmd/main.go
   ```

## Basic Usage

To start the server:

```bash
./coproxy
```

The application will automatically load environment variables from `.env` files in the current or parent directories, making configuration simpler.

By default, the server runs on port 8080. Configure using environment variables:

```bash
LLM_API_SECRET=your-secret-key VALID_API_KEYS=key1,key2 ./coproxy
```

### Simple API Examples

List available models:

```bash
curl http://localhost:8080/models \
  -H "Authorization: Bearer YOUR_API_KEY"
```

Make a completion request:

```bash
curl http://localhost:8080/openai \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "provider": "copilot",
    "model": "copilot-chat",
    "messages": [{"role": "user", "content": "Write a Go function"}]
  }'
```

## GitHub Copilot API Integration

This application integrates with the GitHub Copilot API in three main ways:

### 1. Getting an API Key

The application can retrieve a Copilot API key using a GitHub OAuth token:

```bash
# Use OAuth token from argument
./coproxy --get-api-key="your-github-oauth-token"

# Or use OAuth token from environment variables (.env file)
./coproxy --get-api-key
```

This calls the GitHub API endpoint `https://api.github.com/copilot_internal/v2/token` with the OAuth token in the Authorization header (format: `token YOUR_OAUTH_TOKEN`).

### 2. Making Chat Completion Requests

The application can make chat completion requests to the Copilot API endpoint `https://api.githubcopilot.com/chat/completions` using the retrieved API key.

### 3. Reading Local Copilot Tokens

The application can read existing Copilot tokens from your local configuration at:
- Windows: %APPDATA%\GitHub Copilot\apps.json
- macOS: 
  - ~/.config/github-copilot/apps.json
  - ~/Library/Application Support/GitHub Copilot/apps.json
  - ~/.vscode/extensions/github.copilot-*/config/apps.json
- Linux: 
  - ~/.config/github-copilot/apps.json
  - ~/.vscode/extensions/github.copilot-*/config/apps.json

This allows you to reuse your existing Copilot authentication without obtaining a new token.

## GitHub Copilot API Authentication

This application provides multiple ways to authenticate with the GitHub Copilot API, following a prioritized approach:

### 1. Direct API Key

The simplest method is to provide the GitHub Copilot API token directly:

```bash
export COPILOT_API_KEY="tid=your-token-id;exp=expiration;sku=free_educational;..."
./coproxy
```

### 2. OAuth Token

You can provide a GitHub OAuth token, and the application will automatically exchange it for a Copilot API key:

```bash
export COPILOT_OAUTH_TOKEN="your-github-oauth-token"
# or
export OAUTH_TOKEN="your-github-oauth-token"
./coproxy
```

### 3. Local Configuration

If neither of the above is provided, the application will attempt to read from your local GitHub Copilot configuration in various standard locations.

### Authentication Priority

When a request is made to the Copilot API, the application will try these methods in order:
1. Check for a valid COPILOT_API_KEY environment variable
2. If not found or expired, use COPILOT_OAUTH_TOKEN or OAUTH_TOKEN to get a fresh API key
3. If neither is available, attempt to read from the local GitHub Copilot configuration

This approach ensures maximum flexibility while minimizing the need for manual authentication steps.

## CLI Usage

The application supports granular control via CLI flags. Below are the available options:

### Retrieve an API Key

Use the `--get-api-key` flag to retrieve an API key using a GitHub OAuth token:

```bash
# Provide OAuth token as argument
./coproxy --get-api-key="your-oauth-token"

# Or automatically use token from environment variables or config
./coproxy --get-api-key
```

### Test Authorization/API Key

Use the `--test-auth` flag to test the validity of an API key:

```bash
# Provide API key as argument
./coproxy --test-auth="your-api-key"

# Or automatically retrieve and test API key
./coproxy --test-auth
```

### Make a Test Call

Use the `--test-call` flag to make a test call to verify the API is working:

```bash
./coproxy --test-call="test-payload"
```

### Disable Authentication

Use the `--disable-auth` flag to completely disable API key validation, allowing all requests:

```bash
./coproxy --disable-auth
```

This is useful for development environments where you want to bypass authentication checks.

## Environment Variables

The application can be configured using the following environment variables:

- `VALID_API_KEYS`: Comma-separated list of valid API keys for authenticating with this application
- `DISABLE_AUTH`: Set to "true" or "1" to disable API key verification
- `COPILOT_API_KEY`: GitHub Copilot API token
- `COPILOT_OAUTH_TOKEN`: GitHub OAuth token to exchange for a Copilot API key
- `OAUTH_TOKEN`: Alternative to COPILOT_OAUTH_TOKEN
- `GITHUB_ACCESS_TOKEN`: GitHub API token for additional functionality
- `LLM_API_SECRET`: Secret key for LLM API access
- `STRIPE_API_KEY`: Stripe API key for billing functionality

You can set these variables directly or use a `.env` file, which the application will automatically load:

```
# Example .env file
VALID_API_KEYS=key1,key2,key3
COPILOT_OAUTH_TOKEN=ghu_your_token_here
```

## Troubleshooting

For common issues and solutions, see the detailed documentation:

```bash
go doc
```

## Contributing

Contributions are welcome! Please ensure proper documentation is added for any new code.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Add documentation for your code
4. Commit your changes (`git commit -m 'Add some amazing feature'`)
5. Push to the branch (`git push origin feature/amazing-feature`)
6. Open a Pull Request

## License

This project is licensed under the MIT License. See the LICENSE file for details.