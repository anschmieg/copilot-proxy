// Package llm implements language model integration for various AI providers.
package llm

import (
	"copilot-proxy/pkg/models"
	"copilot-proxy/pkg/utils"
	"fmt"
	"os"
	"sync"
)

// Config contains configuration for the LLM service including API keys,
// enabled providers, and spending limits. This centralizes all configuration
// related to language model providers.
type Config struct {
	// OpenAIAPIKey is the API key for accessing OpenAI services
	OpenAIAPIKey string
	// CopilotAPIKey is the API key for accessing GitHub Copilot Chat
	CopilotAPIKey string
	// AnthropicAPIKey is the API key for accessing Anthropic models
	AnthropicAPIKey string
	// AnthropicStaffAPIKey is a special API key for staff access to Anthropic models
	AnthropicStaffAPIKey string
	// GoogleAIAPIKey is the API key for accessing Google AI models
	GoogleAIAPIKey string
	// EditorVersion identifies the editor (e.g., "vscode/1.99.2")
	EditorVersion string
	// EditorPluginVersion identifies the plugin version (e.g., "copilot-chat/0.26.3")
	EditorPluginVersion string
	// VSCodeMachineID is the unique identifier for the VS Code instance
	VSCodeMachineID string
	// VSCodeSessionID is the session identifier for the VS Code instance
	VSCodeSessionID string
	// EnabledProviders is the list of currently enabled LLM providers
	EnabledProviders []models.LanguageModelProvider
	// ClosedBetaModelName is the name of a model that's in closed beta (if any)
	ClosedBetaModelName string
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
			OpenAIAPIKey:             os.Getenv("OPENAI_API_KEY"),
			CopilotAPIKey:            copilotAPIKey,
			AnthropicAPIKey:          os.Getenv("ANTHROPIC_API_KEY"),
			AnthropicStaffAPIKey:     os.Getenv("ANTHROPIC_STAFF_API_KEY"),
			GoogleAIAPIKey:           os.Getenv("GOOGLE_AI_API_KEY"),
			EditorVersion:            os.Getenv("EDITOR_VERSION"),
			EditorPluginVersion:      os.Getenv("EDITOR_PLUGIN_VERSION"),
			VSCodeMachineID:          os.Getenv("VSCODE_MACHINE_ID"),
			VSCodeSessionID:          os.Getenv("VSCODE_SESSION_ID"),
			EnabledProviders:         defaultEnabledProviders(copilotAPIKey),
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

// defaultEnabledProviders determines which LLM providers should be enabled
// based on available API keys. A provider is only enabled if its API key is available.
//
// This prevents configuration errors where the system might try to use a provider
// without proper authentication.
//
// Parameters:
//   - copilotAPIKey: The GitHub Copilot API key (passed separately as it might be
//     retrieved from local config rather than environment)
//
// Returns a slice of enabled LanguageModelProvider values.
func defaultEnabledProviders(copilotAPIKey string) []models.LanguageModelProvider {
	providers := []models.LanguageModelProvider{}

	if copilotAPIKey != "" {
		providers = append(providers, models.ProviderCopilot)
	}
	if os.Getenv("OPENAI_API_KEY") != "" {
		providers = append(providers, models.ProviderOpenAI)
	}
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		providers = append(providers, models.ProviderAnthropic)
	}
	if os.Getenv("GOOGLE_AI_API_KEY") != "" {
		providers = append(providers, models.ProviderGoogle)
	}

	return providers
}

// DefaultModels returns the default models for each provider with their
// configuration settings including rate limits. This defines all the language
// models available in the system along with their capabilities.
//
// Returns a slice of LanguageModel structures defining each model's properties.
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
		{
			ID:                       "gpt-4",
			Name:                     "gpt-4",
			Provider:                 models.ProviderOpenAI,
			MaxRequestsPerMinute:     20,
			MaxTokensPerMinute:       4000,
			MaxInputTokensPerMinute:  2000,
			MaxOutputTokensPerMinute: 2000,
			MaxTokensPerDay:          80000,
			Enabled:                  true,
		},
		{
			ID:                       "claude-3-opus",
			Name:                     "claude-3-opus",
			Provider:                 models.ProviderAnthropic,
			MaxRequestsPerMinute:     15,
			MaxTokensPerMinute:       3000,
			MaxInputTokensPerMinute:  1500,
			MaxOutputTokensPerMinute: 1500,
			MaxTokensPerDay:          60000,
			Enabled:                  true,
		},
	}
}
