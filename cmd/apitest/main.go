// Package main implements a CLI tool for testing Copilot API integration.
package main

import (
	"copilot-proxy/internal/app"
	"copilot-proxy/internal/llm"
	"copilot-proxy/pkg/utils"
	"flag"
	"fmt"
	"log"
	"os"
)

func main() {
	// Parse command line flags
	prompt := flag.String("prompt", "Hello, what can you do?", "The prompt to send to Copilot")
	apiKey := flag.String("api-key", "", "GitHub Copilot API key (optional, will use env var or local config if not provided)")
	editorVersion := flag.String("editor-version", "", "Editor version identifier (e.g., vscode/1.99.2)")
	pluginVersion := flag.String("plugin-version", "", "Plugin version (e.g., copilot-chat/0.26.3)")
	machineID := flag.String("machine-id", "", "VS Code machine ID")
	sessionID := flag.String("session-id", "", "VS Code session ID")
	debugToken := flag.Bool("debug-token", false, "Print token debugging information")
	oauthToken := flag.String("oauth-token", "", "GitHub OAuth token for retrieving a Copilot API key")
	flag.Parse()

	// Set environment variables if provided
	if *apiKey != "" {
		os.Setenv("COPILOT_API_KEY", *apiKey)
	}
	if *oauthToken != "" {
		os.Setenv("COPILOT_OAUTH_TOKEN", *oauthToken)
	}
	if *editorVersion != "" {
		os.Setenv("EDITOR_VERSION", *editorVersion)
	}
	if *pluginVersion != "" {
		os.Setenv("EDITOR_PLUGIN_VERSION", *pluginVersion)
	}
	if *machineID != "" {
		os.Setenv("VSCODE_MACHINE_ID", *machineID)
	}
	if *sessionID != "" {
		os.Setenv("VSCODE_SESSION_ID", *sessionID)
	}

	// First, try to get API key using the app methods if no direct key was provided
	if os.Getenv("COPILOT_API_KEY") == "" {
		// Create app instance to use your existing API key retrieval methods
		application := app.NewApp()
		apiKey, err := application.GetCopilotAPIKey()
		if err != nil {
			log.Printf("Warning: Failed to automatically retrieve Copilot API key: %v", err)
		} else {
			log.Printf("Successfully retrieved Copilot API key: %s", utils.MaskToken(apiKey))
			os.Setenv("COPILOT_API_KEY", apiKey)
		}
	}

	// Create a new LLM service (now that we've tried to set the API key)
	service := llm.NewService()

	// Print config info
	fmt.Println("🚀 GitHub Copilot API Tester")
	fmt.Println("----------------------------")
	fmt.Printf("Prompt: %s\n", *prompt)
	fmt.Printf("Editor Version: %s\n", getOrDefault(service.GetConfig().EditorVersion, "vscode/1.99.2"))
	fmt.Printf("Plugin Version: %s\n", getOrDefault(service.GetConfig().EditorPluginVersion, "copilot-chat/0.26.3"))

	// Debug token information if requested
	if *debugToken {
		fmt.Println("\n🔍 Token Debug Information")
		fmt.Println("----------------------------")

		// Get the raw token from utility function
		rawToken, err := utils.GetCopilotToken()
		if err != nil {
			fmt.Printf("Error getting raw token: %v\n", err)
		} else {
			fmt.Printf("Raw token format: %s\n", utils.MaskToken(rawToken))
			fmt.Printf("Raw token starts with 'tid=': %v\n", len(rawToken) >= 4 && rawToken[:4] == "tid=")
		}

		// Print config token information
		configToken := service.GetConfig().CopilotAPIKey
		fmt.Printf("\nConfig token format: %s\n", utils.MaskToken(configToken))
		fmt.Printf("Config token starts with 'tid=': %v\n", len(configToken) >= 4 && configToken[:4] == "tid=")
		fmt.Println("----------------------------")
	}

	fmt.Println("\nSending request to GitHub Copilot API...")
	response, err := service.SubmitTestPrompt(*prompt)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	// Print response
	fmt.Println("Response received:")
	fmt.Println("----------------------------")
	fmt.Println(response)
	fmt.Println("----------------------------")
}

// getOrDefault returns the value if non-empty, otherwise returns the default value
func getOrDefault(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}
