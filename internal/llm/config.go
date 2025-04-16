// Package llm implements language model integration for GitHub Copilot.
package llm

import (
	"copilot-proxy/pkg/models"
	"copilot-proxy/pkg/utils"
	"fmt"
	"os"
	"sync"
)

// Config contains configuration for the Copilot LLM service including API keys.
// This centralizes all configuration related to GitHub Copilot.
type Config struct {
	// CopilotAPIKey is the API key for accessing GitHub Copilot Chat
	CopilotAPIKey string
	// EditorVersion identifies the editor (e.g., "vscode/1.99.2")
	EditorVersion string
	// EditorPluginVersion identifies the plugin version (e.g., "copilot-chat/0.26.3")
	EditorPluginVersion string
	// VSCodeMachineID is the unique identifier for the VS Code instance
	VSCodeMachineID string
	// VSCodeSessionID is the session identifier for the VS Code instance
	VSCodeSessionID string
	// DefaultMaxMonthlySpend is the default spending limit in cents per month
	DefaultMaxMonthlySpend uint32
	// FreeTierMonthlyAllowance is the free usage allowance in cents per month
	FreeTierMonthlyAllowance uint32
}

var (
	// config is the singleton instance of the configuration
	config *Config
	// configOnce ensures the configuration is initialized only once
	configOnce sync.Once
)

// GetConfig returns the singleton LLM configuration.
// On first call, it initializes the configuration by loading values from
// environment variables and local configuration files.
//
// It attempts to load the Copilot API key from the following sources in order:
// 1. COPILOT_API_KEY environment variable
// 2. Local GitHub Copilot configuration file (~/.config/github-copilot/apps.json)
// 3. Using OAuth token from environment variables via the app.GetCopilotAPIKey() method
//
// Returns a pointer to the configuration structure.
func GetConfig() *Config {
	configOnce.Do(func() {
		// Try to load Copilot API key from local config if not in environment
		copilotAPIKey := os.Getenv("COPILOT_API_KEY")
		if copilotAPIKey == "" {
			if token, err := utils.GetCopilotToken(); err == nil {
				copilotAPIKey = token
			}

			// If still no API key, try to use the app's GetCopilotAPIKey method
			if copilotAPIKey == "" {
				// Import the app package dynamically to avoid import cycle
				appInstance := createAppInstance()
				if appInstance != nil {
					if key, err := getCopilotAPIKeyFromApp(appInstance); err == nil {
						copilotAPIKey = key
						// Cache it for future use
						os.Setenv("COPILOT_API_KEY", copilotAPIKey)
					}
				}
			}
		}

		config = &Config{
			CopilotAPIKey:            copilotAPIKey,
			EditorVersion:            os.Getenv("EDITOR_VERSION"),
			EditorPluginVersion:      os.Getenv("EDITOR_PLUGIN_VERSION"),
			VSCodeMachineID:          os.Getenv("VSCODE_MACHINE_ID"),
			VSCodeSessionID:          os.Getenv("VSCODE_SESSION_ID"),
			DefaultMaxMonthlySpend:   1000, // $10.00 in cents
			FreeTierMonthlyAllowance: 1000, // $10.00 in cents
		}
	})
	return config
}

// createAppInstance creates a new instance of the app.App type using reflection
// to avoid import cycles.
func createAppInstance() interface{} {
	appPkg, err := utils.DynamicImport("copilot-proxy/internal/app")
	if err != nil {
		return nil
	}

	newAppFunc := appPkg.Lookup("NewApp")
	if newAppFunc == nil {
		return nil
	}

	return newAppFunc.Call(nil)[0].Interface()
}

// getCopilotAPIKeyFromApp calls the GetCopilotAPIKey method on the app instance
// using reflection to avoid import cycles.
func getCopilotAPIKeyFromApp(appInstance interface{}) (string, error) {
	method := utils.GetMethod(appInstance, "GetCopilotAPIKey")
	if method == nil {
		return "", fmt.Errorf("GetCopilotAPIKey method not found")
	}

	results := method.Call(nil)
	if len(results) != 2 {
		return "", fmt.Errorf("unexpected result count from GetCopilotAPIKey")
	}

	if !results[1].IsNil() {
		return "", results[1].Interface().(error)
	}

	return results[0].String(), nil
}

// DefaultModels returns the default models for Copilot with their
// configuration settings including rate limits.
//
// Returns a slice of LanguageModel structures defining the Copilot model properties.
func DefaultModels() []models.LanguageModel {
	return []models.LanguageModel{
		{
			ID:                       "copilot-chat",
			Name:                     "copilot-chat",
			Provider:                 models.ProviderCopilot,
			MaxRequestsPerMinute:     25,
			MaxTokensPerMinute:       5000,
			MaxInputTokensPerMinute:  2500,
			MaxOutputTokensPerMinute: 2500,
			MaxTokensPerDay:          100000,
			Enabled:                  true,
		},
	}
}
