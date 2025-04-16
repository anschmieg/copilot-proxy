package llm

import (
	"copilot-proxy/pkg/models"
	"errors"
	"fmt"
	"net/http"
)

// Authorization errors
var (
	ErrNoCountryCode     = errors.New("no country code provided")
	ErrTorNetwork        = errors.New("access via TOR network is not allowed")
	ErrRestrictedRegion  = errors.New("access from this region is restricted")
	ErrModelNotAvailable = errors.New("this model is not available in your plan")
	ErrRateLimitExceeded = errors.New("rate limit exceeded")
)

// Restricted countries based on export regulations
var (
	restrictedCountries = map[string]bool{
		"AF": true, // Afghanistan
		"BY": true, // Belarus
		"CF": true, // Central African Republic
		"CN": true, // China
		"CU": true, // Cuba
		"ER": true, // Eritrea
		"ET": true, // Ethiopia
		"IR": true, // Iran
		"KP": true, // North Korea
		"XK": true, // Kosovo
		"LY": true, // Libya
		"MM": true, // Myanmar
		"RU": true, // Russia
		"SO": true, // Somalia
		"SS": true, // South Sudan
		"SD": true, // Sudan
		"SY": true, // Syria
		"VE": true, // Venezuela
		"YE": true, // Yemen
	}

	// TOR network identifier
	torNetwork = "T1"
)

// AuthorizeAccessToModel checks if a user can access a specific model
func AuthorizeAccessToModel(token *models.LLMToken, provider models.LanguageModelProvider, modelName string) error {
	// For personal use, everyone has access to all models
	return nil
}

// AuthorizeAccessForCountry checks if a model can be accessed from the user's country
func AuthorizeAccessForCountry(countryCode *string, provider models.LanguageModelProvider) error {
	// In development, we may not have country codes
	if countryCode == nil || *countryCode == "XX" {
		return ErrNoCountryCode
	}

	// Block TOR network
	if *countryCode == torNetwork {
		return fmt.Errorf("%w: access to Copilot models is not available over TOR",
			ErrTorNetwork)
	}

	// Check country restrictions
	if restrictedCountries[*countryCode] {
		return fmt.Errorf("%w: access to Copilot models is not available in your region (%s)",
			ErrRestrictedRegion, *countryCode)
	}

	return nil
}

// CheckRateLimit verifies the user hasn't exceeded their rate limits
func CheckRateLimit(modelName string, usage models.ModelUsage) error {
	availableModels := DefaultModels()
	var model *models.LanguageModel

	// Find the model configuration
	for _, m := range availableModels {
		if m.Name == modelName {
			modelCopy := m // Create a copy to avoid potential issues
			model = &modelCopy
			break
		}
	}

	if model == nil {
		return fmt.Errorf("unknown model: %s", modelName)
	}

	// Check if request limits are exceeded
	if usage.RequestsThisMinute > model.MaxRequestsPerMinute {
		return fmt.Errorf("%w: maximum requests_per_minute reached", ErrRateLimitExceeded)
	}

	// Check if token limits are exceeded
	if usage.TokensThisMinute > model.MaxTokensPerMinute {
		return fmt.Errorf("%w: maximum tokens_per_minute reached", ErrRateLimitExceeded)
	}

	if usage.InputTokensThisMinute > model.MaxInputTokensPerMinute {
		return fmt.Errorf("%w: maximum input_tokens_per_minute reached", ErrRateLimitExceeded)
	}

	if usage.OutputTokensThisMinute > model.MaxOutputTokensPerMinute {
		return fmt.Errorf("%w: maximum output_tokens_per_minute reached", ErrRateLimitExceeded)
	}

	if usage.TokensThisDay > model.MaxTokensPerDay {
		return fmt.Errorf("%w: maximum tokens_per_day reached", ErrRateLimitExceeded)
	}

	return nil
}

// SetErrorResponseHeaders sets the appropriate headers for error responses
func SetErrorResponseHeaders(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrRateLimitExceeded) {
		w.Header().Set("Retry-After", "60")
	}
}

// ValidateAccess performs simplified authorization checks for personal use
func ValidateAccess(token *models.LLMToken, modelName string, usage models.ModelUsage) error {
	// Just check rate limits for personal use
	return CheckRateLimit(modelName, usage)
}
