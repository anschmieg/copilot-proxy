// GitHub Copilot Client for Go
//
// This application serves as a proxy between clients and the GitHub Copilot API,
// allowing you to use GitHub Copilot like any other LLM API. It handles authentication,
// token management, and request formatting.
//
// CLI Usage:
//
//	The application supports the following command-line flags:
//
//	--get-api-key="oauth-token"
//	  Retrieves a GitHub Copilot API key using the provided OAuth token.
//	  Example: ./coproxy --get-api-key="ghu_your_github_oauth_token"
//
//	--test-auth="api-key"
//	  Tests if an API key or GitHub Copilot token is valid.
//	  Example: ./coproxy --test-auth="your-api-key"
//
//	--test-call="payload"
//	  Makes a test call to verify the API is working.
//	  Example: ./coproxy --test-call="test-message"
//
//	--disable-auth
//	  Disables API key authorization, allowing all API requests without validation.
//	  Example: ./coproxy --disable-auth
//
//	--test-copilot
//	  Tests the Copilot API with a sample prompt.
//	  Example: ./coproxy --test-copilot
//
// Environment Variables:
//   - VALID_API_KEYS: Comma-separated list of valid API keys for accessing this application
//   - DISABLE_AUTH: Set to "true" or "1" to disable API key verification
//   - COPILOT_API_KEY: GitHub Copilot API token
//   - GITHUB_ACCESS_TOKEN: GitHub API token for additional functionality
//   - OAUTH_TOKEN: OAuth token for authenticating with GitHub
//   - LLM_API_SECRET: Secret key for LLM API access
//   - STRIPE_API_KEY: Stripe API key for billing functionality
package main

import (
	"bytes"
	"context"
	"copilot-proxy/internal"
	"copilot-proxy/internal/app"
	"copilot-proxy/internal/auth"
	"copilot-proxy/internal/llm"
	"copilot-proxy/internal/rpc"
	"copilot-proxy/pkg/utils"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
)

// loadEnvFile loads environment variables from a .env file if present.
// It attempts to load from the current directory and parent directories
// up to the root directory.
func loadEnvFile() {
	// Try current directory first
	err := godotenv.Load()
	if err == nil {
		log.Println("Loaded environment variables from .env file in current directory")
		return
	}

	// Get the current working directory
	workDir, err := os.Getwd()
	if err != nil {
		log.Printf("Warning: Could not determine current directory: %v", err)
		return
	}

	// Try parent directories recursively
	for dir := workDir; dir != "/"; dir = filepath.Dir(dir) {
		envPath := filepath.Join(dir, ".env")
		if _, err := os.Stat(envPath); err == nil {
			err = godotenv.Load(envPath)
			if err == nil {
				log.Printf("Loaded environment variables from %s", envPath)
				return
			}
		}
	}

	log.Println("No .env file found. Using existing environment variables.")
}

func testCopilotAPI() {
	log.Println("Starting Copilot API test...")

	// Step 1: Read OAuth token from .env
	log.Println("Reading OAuth token from environment variables...")
	oauthToken, err := utils.GetCopilotOAuthToken()
	if err != nil {
		log.Fatalf("Failed to retrieve OAuth token: %v", err)
	}
	log.Printf("Successfully retrieved OAuth token: %s", utils.MaskToken(oauthToken))

	// Step 2: Exchange OAuth token for API key
	log.Println("Exchanging OAuth token for API key...")
	application := app.NewApp()
	apiKey, err := application.GetAPIKey(oauthToken)
	if err != nil {
		log.Fatalf("Failed to exchange OAuth token for API key: %v", err)
	}
	log.Printf("Successfully retrieved API key: %s", utils.MaskToken(apiKey))

	// Step 3: Submit a test request to the Copilot API
	log.Println("Submitting test request to Copilot API...")

	// Set the API key in the config environment variable so NewService() picks it up
	os.Setenv("COPILOT_API_KEY", apiKey)

	// Create a new LLM service that will use the API key from environment
	llmService := llm.NewService()

	response, err := llmService.SubmitTestPrompt("Write a Go function to reverse a string")
	if err != nil {
		log.Fatalf("Failed to submit test request: %v", err)
	}
	log.Println("Successfully received response from Copilot API")

	// Step 4: Return the response to the user
	fmt.Println("\nResponse from Copilot API:")
	fmt.Println(response)
}

func monitorVSCodeRequests() {
	log.Println("Setting up a proxy to monitor VS Code requests to Copilot API...")

	// Define the proxy server
	proxy := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only log requests to Copilot endpoints
		if strings.Contains(r.Host, "githubcopilot.com") ||
			strings.Contains(r.Host, "api.github.com") && strings.Contains(r.URL.Path, "copilot") {

			// Log the request details
			log.Printf("==== VS Code Copilot Request ====")
			log.Printf("URL: %s %s", r.Method, r.URL)

			// Log all headers
			log.Printf("Headers:")
			for key, values := range r.Header {
				for _, value := range values {
					log.Printf("  %s: %s", key, value)
				}
			}

			// Log body if it exists
			if r.Body != nil {
				bodyBytes, err := io.ReadAll(r.Body)
				if err != nil {
					log.Printf("Error reading body: %v", err)
				} else {
					r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
					log.Printf("Body: %s", string(bodyBytes))
				}
			}

			log.Printf("=================================")
		}

		// Forward the request to the original destination
		client := &http.Client{}
		resp, err := client.Do(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		defer resp.Body.Close()

		// Copy response headers
		for key, values := range resp.Header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}

		// Copy response status code
		w.WriteHeader(resp.StatusCode)

		// Copy response body
		io.Copy(w, resp.Body)
	})

	// Start the proxy server
	log.Println("Starting proxy server on port 9999...")
	log.Println("To use this proxy with VS Code:")
	log.Println("1. Open VS Code settings (Ctrl+,)")
	log.Println("2. Search for 'proxy'")
	log.Println("3. Set 'Http: Proxy' to 'http://localhost:9999'")
	log.Println("4. Restart VS Code")
	log.Println("Press Ctrl+C to stop the proxy server")

	if err := http.ListenAndServe(":9999", proxy); err != nil {
		log.Fatalf("Failed to start proxy server: %v", err)
	}
}

func main() {
	// Load environment variables from .env file
	loadEnvFile()

	// Define CLI flags
	getAPIKey := flag.String("get-api-key", "", "Retrieve an API key using the provided OAuth token")
	testAuth := flag.String("test-auth", "", "Test the Authorization/API key")
	testCall := flag.String("test-call", "", "Make a test call to verify the API is working")
	disableAuth := flag.Bool("disable-auth", false, "Disable API key authorization and accept all requests")
	testCopilot := flag.Bool("test-copilot", false, "Test the Copilot API with a sample prompt")

	flag.Parse()

	// Set environment variable if disable-auth flag is set
	if *disableAuth {
		os.Setenv("DISABLE_AUTH", "true")
		log.Println("API authorization is disabled - all requests will be accepted")
	}

	// Initialize the app
	a := app.NewApp()

	// Set default behaviors that can be overridden by flags
	serverMode := true

	// Process CLI flags
	if flag.Lookup("get-api-key").Value.String() != "" || flag.CommandLine.Lookup("get-api-key").DefValue != flag.Lookup("get-api-key").Value.String() {
		serverMode = false

		if *getAPIKey == "" {
			log.Println("No OAuth token provided as argument, trying to retrieve from environment...")
			var err error
			*getAPIKey, err = utils.GetCopilotOAuthToken()
			if err != nil {
				log.Fatalf("Failed to automatically retrieve OAuth token: %v", err)
			}
			log.Printf("Using OAuth token from environment: %s", utils.MaskToken(*getAPIKey))
		}

		// Get API key using OAuth token
		apiKey, err := a.GetAPIKey(*getAPIKey)
		if err != nil {
			log.Fatalf("Failed to retrieve API key: %v", err)
		}
		fmt.Printf("Retrieved API key: %s\n", apiKey)
		os.Exit(0)
	}

	if flag.Lookup("test-auth").Value.String() != "" || flag.CommandLine.Lookup("test-auth").DefValue != flag.Lookup("test-auth").Value.String() {
		serverMode = false
		apiKeyArg := *testAuth

		// If no API key was provided in the argument, try to get it from our API key retrieval process
		if apiKeyArg == "" {
			log.Println("No API key provided as argument, trying to retrieve automatically...")
			var err error
			apiKeyArg, err = a.GetCopilotAPIKey()
			if err != nil {
				log.Fatalf("Failed to automatically retrieve API key: %v", err)
			}
			log.Printf("Using API key retrieved automatically")
		}

		// Test the Authorization/API key
		if auth.VerifyAppAPIKey(apiKeyArg) {
			fmt.Println("✅ Valid application API key")
		} else if auth.VerifyCopilotAPIKey(apiKeyArg) {
			fmt.Println("✅ Valid GitHub Copilot API token")
		} else {
			log.Fatalf("❌ Invalid API key or token")
		}
		os.Exit(0)
	}

	if *testCall != "" {
		serverMode = false
		// Make a test call to verify the API is working
		response, err := a.TestAPI(*testCall)
		if err != nil {
			log.Fatalf("Test call failed: %v", err)
		}
		fmt.Printf("Test call response: %s\n", response)
		os.Exit(0)
	}

	if *testCopilot {
		testCopilotAPI()
		os.Exit(0)
	}

	// If no command-line flags were used, run in server mode
	if !serverMode {
		return
	}

	// Print help message if no flags were used
	if flag.NFlag() == 0 {
		fmt.Println("Running in server mode. Use --help for CLI options.")
	}

	// Create a context that will be canceled on program termination
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("Shutting down...")
		cancel()
	}()

	// Initialize connection pool for RPC
	_ = rpc.NewConnectionPool() // Discard the unused connection pool

	// Initialize the authentication service
	_ = auth.NewService()

	// Initialize Copilot API key using our prioritized approach
	log.Println("Initializing GitHub Copilot API key...")
	copilotKey, err := a.GetCopilotAPIKey()
	if err != nil {
		log.Printf("Warning: %v", err)
		log.Println("Continuing without Copilot API key. Will attempt to retrieve one when needed.")
	} else {
		log.Printf("Successfully initialized GitHub Copilot API key")
		// Store the key in environment variable for future use
		os.Setenv("COPILOT_API_KEY", copilotKey)
	}

	// Initialize user backfiller if GitHub token is provided
	githubToken := os.Getenv("GITHUB_ACCESS_TOKEN")
	if githubToken != "" {
		userBackfiller := internal.NewUserBackfiller(nil, githubToken) // Replace nil with actual DB interface
		go userBackfiller.Start(ctx)
	}

	// Initialize Stripe billing if API key is provided
	stripeKey := os.Getenv("STRIPE_API_KEY")
	if stripeKey != "" {
		stripeBilling, err := internal.NewStripeBilling(stripeKey)
		if err != nil {
			log.Printf("Failed to initialize Stripe billing: %v", err)
		} else {
			if err := stripeBilling.Initialize(); err != nil {
				log.Printf("Failed to initialize Stripe meters and prices: %v", err)
			}
		}
	}

	// Initialize LLM server
	llmSecret := os.Getenv("LLM_API_SECRET")
	if llmSecret != "" {
		llmState := llm.NewLLMServerState(llmSecret)
		// Register LLM handlers
		llmState.RegisterHandlers(a.Router)

		// Log available LLM providers
		config := llm.GetConfig()
		for _, provider := range config.EnabledProviders {
			log.Printf("Enabled LLM provider: %s", provider)
		}
	}

	// Authenticate and retrieve API key using OAuth token
	oauthToken := os.Getenv("OAUTH_TOKEN")
	if oauthToken != "" {
		apiKey, err := a.GetAPIKey(oauthToken)
		if err != nil {
			log.Fatalf("Failed to retrieve API key: %v", err)
		}
		log.Printf("Retrieved API key: %s", apiKey)
	}

	// Start HTTP server with graceful shutdown
	server := &http.Server{
		Addr:    ":8080",
		Handler: a.Router,
	}

	// Start the server in a goroutine
	go func() {
		log.Println("Starting server on :8080...")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Could not start server: %v", err)
		}
	}()

	// Start the proxy server to monitor VS Code requests
	go monitorVSCodeRequests()

	// Wait for shutdown signal
	<-ctx.Done()

	// Create a deadline for server shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	// Attempt graceful shutdown
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Error during server shutdown: %v", err)
	} else {
		log.Println("Server gracefully stopped")
	}
}
