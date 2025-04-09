package llm

import (
	"copilot-proxy/pkg/models"
	"errors"
	"fmt"
	"net/http"
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

// Authorization errors
var (
	ErrNoCountryCode        = errors.New("no country code provided")
	ErrTorNetwork           = errors.New("access via TOR network is not allowed")
	ErrRestrictedRegion     = errors.New("access from this region is restricted")
	ErrModelNotAvailable    = errors.New("this model is not available in your plan")
	ErrRateLimitExceeded    = errors.New("rate limit exceeded")
	ErrSpendingLimitReached = errors.New("monthly spending limit reached")
)

// AuthorizeAccessToModel checks if a user can access a specific model
func AuthorizeAccessToModel(token *models.LLMToken, provider models.LanguageModelProvider, modelName string) error {
	// Staff can access all models
	if token.IsStaff {
		return nil
	}

	// Copilot models are available to all users with valid tokens
	if provider == models.ProviderCopilot {
		return nil
	}

	// Claude 3.5 sonnet is available to all plans
	if provider == models.ProviderAnthropic &&
		(modelName == "claude-3-5-sonnet" || modelName == "claude-3-7-sonnet") {
		return nil
	}

	// Other models require specific access
	config := GetConfig()
	if config.ClosedBetaModelName == modelName {
		// Logic for beta access would go here
		return nil
	}

	return ErrModelNotAvailable
}

// AuthorizeAccessForCountry checks if a model can be accessed from the user's country
func AuthorizeAccessForCountry(countryCode *string, provider models.LanguageModelProvider) error {
	// In development, we may not have country codes
	if countryCode == nil || *countryCode == "XX" {
		return ErrNoCountryCode
	}

	// Block TOR network
	if *countryCode == torNetwork {
		return fmt.Errorf("%w: access to %s models is not available over TOR",
			ErrTorNetwork, provider)
	}

	// Check country restrictions
	if restrictedCountries[*countryCode] {
		return fmt.Errorf("%w: access to %s models is not available in your region (%s)",
			ErrRestrictedRegion, provider, *countryCode)
	}

	return nil
}

// CheckRateLimit verifies the user hasn't exceeded their rate limits
func CheckRateLimit(userID uint64, provider models.LanguageModelProvider, modelName string,
	usage models.ModelUsage, activeUsers models.ActiveUserCount) error {

	availableModels := DefaultModels()
	var model *models.LanguageModel

	// Find the model configuration
	for _, m := range availableModels {
		if m.Provider == provider && m.Name == modelName {
			modelCopy := m // Create a copy to avoid potential issues
			model = &modelCopy
			break
		}
	}

	if model == nil {
		return fmt.Errorf("unknown model: %s from provider %s", modelName, provider)
	}

	// Scale limits based on active users
	usersInRecentMinutes := activeUsers.UsersInRecentMinutes
	if usersInRecentMinutes < 1 {
		usersInRecentMinutes = 1
	}

	usersInRecentDays := activeUsers.UsersInRecentDays
	if usersInRecentDays < 1 {
		usersInRecentDays = 1
	}

	perUserMaxRequestsPerMinute := model.MaxRequestsPerMinute / usersInRecentMinutes
	perUserMaxTokensPerMinute := model.MaxTokensPerMinute / usersInRecentMinutes
	perUserMaxInputTokensPerMinute := model.MaxInputTokensPerMinute / usersInRecentMinutes
	perUserMaxOutputTokensPerMinute := model.MaxOutputTokensPerMinute / usersInRecentMinutes
	perUserMaxTokensPerDay := model.MaxTokensPerDay / usersInRecentDays

	// Check if any limits are exceeded
	if usage.RequestsThisMinute > perUserMaxRequestsPerMinute {
		return fmt.Errorf("%w: maximum requests_per_minute reached", ErrRateLimitExceeded)
	}

	if usage.TokensThisMinute > perUserMaxTokensPerMinute {
		return fmt.Errorf("%w: maximum tokens_per_minute reached", ErrRateLimitExceeded)
	}

	if usage.InputTokensThisMinute > perUserMaxInputTokensPerMinute {
		return fmt.Errorf("%w: maximum input_tokens_per_minute reached", ErrRateLimitExceeded)
	}

	if usage.OutputTokensThisMinute > perUserMaxOutputTokensPerMinute {
		return fmt.Errorf("%w: maximum output_tokens_per_minute reached", ErrRateLimitExceeded)
	}

	if usage.TokensThisDay > perUserMaxTokensPerDay {
		return fmt.Errorf("%w: maximum tokens_per_day reached", ErrRateLimitExceeded)
	}

	return nil
}

// CheckSpendingLimit verifies if the user is within their spending limits
func CheckSpendingLimit(token *models.LLMToken, currentSpending uint32) error {
	// Staff bypass spending limits
	if token.IsStaff {
		return nil
	}

	config := GetConfig()
	freeTier := token.CustomMonthlyAllowanceInCents
	if freeTier == nil {
		defaultFreeTier := config.FreeTierMonthlyAllowance
		freeTier = &defaultFreeTier
	}

	if currentSpending >= *freeTier {
		if !token.HasLLMSubscription {
			return ErrSpendingLimitReached
		}

		// For subscribers, check against their max monthly spend
		monthlySpend := currentSpending - *freeTier
		if monthlySpend >= token.MaxMonthlySpendInCents {
			return ErrSpendingLimitReached
		}
	}

	return nil
}

// SetErrorResponseHeaders sets the appropriate headers for error responses
func SetErrorResponseHeaders(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrSpendingLimitReached):
		w.Header().Set("X-LLM-Monthly-Spend-Reached", "true")
	case errors.Is(err, ErrRateLimitExceeded):
		w.Header().Set("Retry-After", "60")
	}
}

// ValidateAccess performs all authorization checks for an LLM request
func ValidateAccess(token *models.LLMToken, countryCode *string, provider models.LanguageModelProvider,
	modelName string, usage models.ModelUsage, activeUsers models.ActiveUserCount,
	currentSpending uint32) error {

	// Check country restrictions
	if err := AuthorizeAccessForCountry(countryCode, provider); err != nil {
		return err
	}

	// Check if model is available to user's plan
	if err := AuthorizeAccessToModel(token, provider, modelName); err != nil {
		return err
	}

	// Check spending limits
	if err := CheckSpendingLimit(token, currentSpending); err != nil {
		return err
	}

	// Check rate limits
	if err := CheckRateLimit(token.UserID, provider, modelName, usage, activeUsers); err != nil {
		return err
	}

	return nil
}
