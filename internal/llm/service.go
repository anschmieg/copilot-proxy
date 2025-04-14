package llm

import (
	"bytes"
	"copilot-proxy/pkg/models"
	"copilot-proxy/pkg/utils"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	// CopilotChatCompletionURL is the endpoint for GitHub Copilot chat completions.
	CopilotChatCompletionURL = "https://api.githubcopilot.com/chat/completions"

	// OpenAIChatCompletionURL is the endpoint for OpenAI chat completions.
	OpenAIChatCompletionURL = "https://api.openai.com/v1/chat/completions"

	// AnthropicCompletionURL is the endpoint for Anthropic API access.
	AnthropicCompletionURL = "https://api.anthropic.com/v1/messages"

	// GoogleAICompletionURL is the endpoint for Google AI API access.
	GoogleAICompletionURL = "https://generativelanguage.googleapis.com/v1/models"

	// MinAccountAgeDays is the minimal account age required for using LLM features.
	MinAccountAgeDays = 7
)

var (
	// ErrProviderNotSupported is returned when the requested provider is not supported.
	ErrProviderNotSupported = errors.New("provider not supported")

	// ErrAccountTooYoung is returned when a user's account is too new to use LLM features.
	ErrAccountTooYoung = errors.New("account must be older than 7 days to use LLM features")
)

// Service manages LLM API interactions
type Service struct {
	config      *Config
	httpClient  *http.Client
	usageLock   sync.RWMutex
	userUsage   map[uint64]models.ModelUsage
	activeUsers map[string]models.ActiveUserCount // key is provider:model
}

// NewService creates a new LLM service
func NewService() *Service {
	return &Service{
		config:      GetConfig(),
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		userUsage:   make(map[uint64]models.ModelUsage),
		activeUsers: make(map[string]models.ActiveUserCount),
	}
}

// CompletionRequest contains the data needed for a completion request
type CompletionRequest struct {
	Provider        models.LanguageModelProvider
	Model           string
	ProviderRequest string // JSON payload for the provider
	Token           *models.LLMToken
	CountryCode     *string
	CurrentSpending uint32
}

// GetActiveUserCount returns active users for a model
func (s *Service) GetActiveUserCount(provider models.LanguageModelProvider, model string) models.ActiveUserCount {
	key := fmt.Sprintf("%s:%s", provider, model)
	s.usageLock.RLock()
	defer s.usageLock.RUnlock()

	count, exists := s.activeUsers[key]
	if !exists {
		return models.ActiveUserCount{
			UsersInRecentMinutes: 1,
			UsersInRecentDays:    1,
		}
	}
	return count
}

// RecordUsage records token usage for a user and model
func (s *Service) RecordUsage(userID uint64, provider models.LanguageModelProvider, model string, usage models.TokenUsage) {
	s.usageLock.Lock()
	defer s.usageLock.Unlock()

	existing, exists := s.userUsage[userID]

	if !exists {
		existing = models.ModelUsage{
			UserID:                 userID,
			Provider:               provider,
			Model:                  model,
			RequestsThisMinute:     1,
			TokensThisMinute:       usage.Input + usage.Output,
			InputTokensThisMinute:  usage.Input,
			OutputTokensThisMinute: usage.Output,
			TokensThisDay:          usage.Input + usage.Output,
		}
	} else {
		existing.RequestsThisMinute++
		existing.TokensThisMinute += usage.Input + usage.Output
		existing.InputTokensThisMinute += usage.Input
		existing.OutputTokensThisMinute += usage.Output
		existing.TokensThisDay += usage.Input + usage.Output
	}

	s.userUsage[userID] = existing

	// Update active user counts
	modelKey := fmt.Sprintf("%s:%s", provider, model)
	activeCount, exists := s.activeUsers[modelKey]
	if !exists {
		activeCount = models.ActiveUserCount{
			UsersInRecentMinutes: 1,
			UsersInRecentDays:    1,
		}
	}
	// In a real implementation, we would track unique users over time
	s.activeUsers[modelKey] = activeCount
}

// GetModelUsage returns the current usage for a user and model
func (s *Service) GetModelUsage(userID uint64, provider models.LanguageModelProvider, model string) models.ModelUsage {
	s.usageLock.RLock()
	defer s.usageLock.RUnlock()

	existing, exists := s.userUsage[userID]
	if !exists {
		return models.ModelUsage{
			UserID:   userID,
			Provider: provider,
			Model:    model,
		}
	}
	return existing
}

// PerformCompletion handles an LLM completion request
func (s *Service) PerformCompletion(req CompletionRequest) (*http.Response, error) {
	// Check account age
	accountAgeInDays := time.Since(req.Token.AccountCreatedAt).Hours() / 24
	if accountAgeInDays < MinAccountAgeDays && !req.Token.IsStaff && !req.Token.HasLLMSubscription {
		return nil, ErrAccountTooYoung
	}

	// Normalize model name
	model := normalizeModelName(req.Provider, req.Model)

	// Get current usage
	usage := s.GetModelUsage(req.Token.UserID, req.Provider, model)
	activeUsers := s.GetActiveUserCount(req.Provider, model)

	// Validate access
	if err := ValidateAccess(req.Token, req.CountryCode, req.Provider, model,
		usage, activeUsers, req.CurrentSpending); err != nil {
		return nil, err
	}

	// Route to appropriate provider
	var resp *http.Response
	var err error

	switch req.Provider {
	case models.ProviderCopilot:
		resp, err = s.callCopilotAPI(req.ProviderRequest)
	case models.ProviderOpenAI:
		resp, err = s.callOpenAIAPI(req.ProviderRequest)
	case models.ProviderAnthropic:
		resp, err = s.callAnthropicAPI(req.ProviderRequest, req.Token.IsStaff)
	case models.ProviderGoogle:
		resp, err = s.callGoogleAIAPI(req.ProviderRequest)
	default:
		return nil, ErrProviderNotSupported
	}

	return resp, err
}

// normalizeModelName ensures we use the correct model name for the provider
func normalizeModelName(provider models.LanguageModelProvider, name string) string {
	models := DefaultModels()
	var bestMatch string
	var bestMatchLength int

	for _, model := range models {
		if model.Provider == provider && strings.HasPrefix(name, model.Name) {
			if len(model.Name) > bestMatchLength {
				bestMatch = model.Name
				bestMatchLength = len(model.Name)
			}
		}
	}

	if bestMatch != "" {
		return bestMatch
	}

	return name
}

// callCopilotAPI calls the GitHub Copilot API for chat completions.
//
// This method sends a request to the GitHub Copilot chat completions endpoint
// using the provided API key (which should be obtained via GetAPIKey or from
// the local configuration).
//
// Parameters:
//   - providerRequest: A JSON string containing the completion request parameters
//
// The GitHub Copilot chat API expects requests in a similar format to OpenAI's
// chat completion API, with a model name and an array of messages.
//
// Authorization is done using a "Bearer" token format with the Copilot API key.
func (s *Service) callCopilotAPI(providerRequest string) (*http.Response, error) {
	apiKey := s.config.CopilotAPIKey
	if apiKey == "" {
		return nil, errors.New("Copilot API key not configured")
	}

	var requestData map[string]interface{}
	if err := json.Unmarshal([]byte(providerRequest), &requestData); err != nil {
		return nil, err
	}

	return utils.CallAPIWithBody(CopilotChatCompletionURL, "application/json", apiKey, requestData)
}

// callOpenAIAPI calls the OpenAI API
func (s *Service) callOpenAIAPI(providerRequest string) (*http.Response, error) {
	apiKey := s.config.OpenAIAPIKey
	if apiKey == "" {
		return nil, errors.New("OpenAI API key not configured")
	}

	var requestData map[string]interface{}
	if err := json.Unmarshal([]byte(providerRequest), &requestData); err != nil {
		return nil, err
	}

	return utils.CallAPIWithBody(OpenAIChatCompletionURL, "application/json", apiKey, requestData)
}

// callAnthropicAPI calls the Anthropic API
func (s *Service) callAnthropicAPI(providerRequest string, isStaff bool) (*http.Response, error) {
	var apiKey string
	if isStaff && s.config.AnthropicStaffAPIKey != "" {
		apiKey = s.config.AnthropicStaffAPIKey
	} else {
		apiKey = s.config.AnthropicAPIKey
	}

	if apiKey == "" {
		return nil, errors.New("Anthropic API key not configured")
	}

	var requestData map[string]interface{}
	if err := json.Unmarshal([]byte(providerRequest), &requestData); err != nil {
		return nil, err
	}

	// Modify model name if needed to use the latest version
	if model, ok := requestData["model"].(string); ok {
		switch model {
		case "claude-3-5-sonnet":
			requestData["model"] = "claude-3-5-sonnet-20240620"
		case "claude-3-7-sonnet":
			requestData["model"] = "claude-3-7-sonnet-20240307"
		case "claude-3-opus":
			requestData["model"] = "claude-3-opus-20240229"
		case "claude-3-haiku":
			requestData["model"] = "claude-3-haiku-20240307"
		case "claude-3-sonnet":
			requestData["model"] = "claude-3-sonnet-20240229"
		}
	}

	body, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", AnthropicCompletionURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	return s.httpClient.Do(req)
}

// callGoogleAIAPI calls the Google AI API
func (s *Service) callGoogleAIAPI(providerRequest string) (*http.Response, error) {
	apiKey := s.config.GoogleAIAPIKey
	if apiKey == "" {
		return nil, errors.New("Google AI API key not configured")
	}

	var requestData map[string]interface{}
	if err := json.Unmarshal([]byte(providerRequest), &requestData); err != nil {
		return nil, err
	}

	// Extract model name
	model, ok := requestData["model"].(string)
	if !ok {
		return nil, errors.New("missing model in request")
	}

	// Construct Google AI API URL with model name
	url := fmt.Sprintf("%s/%s:generateContent?key=%s", GoogleAICompletionURL, model, apiKey)

	body, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	return s.httpClient.Do(req)
}

// ProcessStreamingResponse processes a streaming response from any provider
func (s *Service) ProcessStreamingResponse(resp *http.Response, userID uint64, provider models.LanguageModelProvider, model string) (io.ReadCloser, error) {
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("provider returned error: %s", string(body))
	}

	// In a full implementation, this would track token usage from the streaming response
	// For simplicity, we'll just return the response body as-is

	return resp.Body, nil
}

// SubmitTestPrompt sends a test prompt to the Copilot API and returns the response.
func (s *Service) SubmitTestPrompt(prompt string) (string, error) {
	log.Println("Preparing test prompt request...")

	// Construct the provider request payload
	requestPayload := map[string]interface{}{
		"model": "copilot-chat",
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}

	// Serialize the payload to JSON
	requestBody, err := json.Marshal(requestPayload)
	if err != nil {
		return "", fmt.Errorf("failed to serialize request payload: %w", err)
	}

	// Call the Copilot API
	response, err := s.callCopilotAPI(string(requestBody))
	if err != nil {
		return "", fmt.Errorf("failed to call Copilot API: %w", err)
	}
	defer response.Body.Close()

	// Read and return the response body
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	return string(responseBody), nil
}
