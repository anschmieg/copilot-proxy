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
copilot-client
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
   git clone https://github.com/yourusername/copilot-client.git
   cd copilot-client
   ```

2. Install dependencies:
   
   ```
   go mod tidy
   ```

3. Build the application:
   
   ```
   go build -o copilot-client cmd/main.go
   ```

## Basic Usage

To start the server:

```bash
./copilot-client
```

By default, the server runs on port 8080. Configure using environment variables:

```bash
LLM_API_SECRET=your-secret-key VALID_API_KEYS=key1,key2 go run cmd/main.go
```

### Simple API Examples

List available models:

```bash
curl http://localhost:8080/models \
  -H "Authorization: Bearer YOUR_API_KEY"
```

Make a completion request:

```bash
curl http://localhost:8080/completion \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "provider": "copilot",
    "model": "copilot-chat",
    "provider_request": "{\"model\":\"copilot-chat\",\"messages\":[{\"role\":\"user\",\"content\":\"Write a Go function\"}]}"
  }'
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