package main

import (
	"context"
	"copilot-proxy/internal"
	"copilot-proxy/internal/app"
	"copilot-proxy/internal/auth"
	"copilot-proxy/internal/llm"
	"copilot-proxy/internal/rpc"
	"copilot-proxy/pkg/utils"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
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

	// Initialize app
	a := app.NewApp()

	// Check for GitHub Copilot token in local config if not in environment
	if os.Getenv("COPILOT_API_KEY") == "" {
		if token, err := utils.GetCopilotToken(); err == nil {
			os.Setenv("COPILOT_API_KEY", token)
			log.Printf("Retrieved GitHub Copilot token from local configuration")
		} else {
			log.Printf("Could not retrieve GitHub Copilot token: %v", err)
		}
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
