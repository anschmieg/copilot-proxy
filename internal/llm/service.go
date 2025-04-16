package llm

import (
	"bufio"
	"bytes"
	"copilot-proxy/pkg/models"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	// CopilotChatCompletionURL is the endpoint for GitHub Copilot chat completions.
	CopilotChatCompletionURL = "https://api.githubcopilot.com/chat/completions"
)

var (
	// ErrCopilotAPIKeyMissing is returned when no Copilot API key is configured
	ErrCopilotAPIKeyMissing = errors.New("Copilot API key not configured")
)

// Service manages GitHub Copilot API interactions
type Service struct {
	config     *Config
	httpClient *http.Client
	usageLock  sync.RWMutex
	userUsage  map[uint64]models.ModelUsage
}

// NewService creates a new LLM service
func NewService() *Service {
	return &Service{
		config:     GetConfig(),
		httpClient: &http.Client{Timeout: 30 * time.Second},
		userUsage:  make(map[uint64]models.ModelUsage),
	}
}

// GetConfig returns the service's configuration
func (s *Service) GetConfig() *Config {
	return s.config
}

// CompletionRequest contains the data needed for a completion request
type CompletionRequest struct {
	Model           string
	ProviderRequest string // JSON payload for the provider
	Token           *models.LLMToken
	CountryCode     *string
	CurrentSpending uint32
}

// RecordUsage records token usage for a user and model
func (s *Service) RecordUsage(userID uint64, model string, usage models.TokenUsage) {
	s.usageLock.Lock()
	defer s.usageLock.Unlock()

	existing, exists := s.userUsage[userID]

	if !exists {
		existing = models.ModelUsage{
			UserID:             userID,
			Model:              model,
			RequestsThisMinute: 1,
			TokensThisMinute:   usage.Input + usage.Output,
		}
	} else {
		existing.RequestsThisMinute++
		existing.TokensThisMinute += usage.Input + usage.Output
	}

	s.userUsage[userID] = existing
}

// GetModelUsage returns the current usage for a user and model
func (s *Service) GetModelUsage(userID uint64, model string) models.ModelUsage {
	s.usageLock.RLock()
	defer s.usageLock.RUnlock()

	existing, exists := s.userUsage[userID]
	if !exists {
		return models.ModelUsage{
			UserID: userID,
			Model:  model,
		}
	}
	return existing
}

// PerformCompletion handles a GitHub Copilot completion request
func (s *Service) PerformCompletion(req CompletionRequest) (*http.Response, error) {
	// Normalize model name
	model := normalizeModelName(req.Model)

	// Get current usage
	usage := s.GetModelUsage(req.Token.UserID, model)

	// Validate access (just rate limiting for personal use)
	if err := ValidateAccess(req.Token, model, usage); err != nil {
		return nil, err
	}

	// Call Copilot API
	return s.callCopilotAPI(req.ProviderRequest)
}

// normalizeModelName ensures we use the correct model name
func normalizeModelName(name string) string {
	models := DefaultModels()
	var bestMatch string
	var bestMatchLength int

	for _, model := range models {
		if strings.HasPrefix(name, model.Name) {
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
func (s *Service) callCopilotAPI(providerRequest string) (*http.Response, error) {
	apiKey := s.config.CopilotAPIKey
	if apiKey == "" {
		return nil, ErrCopilotAPIKeyMissing
	}

	var requestData map[string]interface{}
	if err := json.Unmarshal([]byte(providerRequest), &requestData); err != nil {
		return nil, err
	}

	// Set default values if not specified
	if _, ok := requestData["model"]; !ok {
		requestData["model"] = "gpt-4o"
	}

	if _, ok := requestData["temperature"]; !ok {
		requestData["temperature"] = 0
	}

	if _, ok := requestData["top_p"]; !ok {
		requestData["top_p"] = 1
	}

	if _, ok := requestData["max_tokens"]; !ok {
		requestData["max_tokens"] = 4096
	}

	// Serialize the request body
	body, err := json.Marshal(requestData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", CopilotChatCompletionURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set required headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	// Editor and plugin version
	editorVersion := s.config.EditorVersion
	if editorVersion == "" {
		editorVersion = "vscode/1.99.2" // Default value
	}

	pluginVersion := s.config.EditorPluginVersion
	if pluginVersion == "" {
		pluginVersion = "copilot-chat/0.26.3" // Default value
	}

	integrationID := "vscode-chat"
	userAgent := "GitHubCopilotChat/" + strings.TrimPrefix(pluginVersion, "copilot-chat/")

	// Set all required headers based on mitmproxy logs
	req.Header.Set("Editor-Version", editorVersion)
	req.Header.Set("Editor-Plugin-Version", pluginVersion)
	req.Header.Set("Copilot-Integration-ID", integrationID)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("OpenAI-Intent", "conversation-agent")
	req.Header.Set("X-GitHub-API-Version", "2025-04-01")
	req.Header.Set("X-Initiator", "user")
	req.Header.Set("X-Interaction-Type", "conversation-agent")

	// Generate unique request ID
	requestID := generateRequestID()
	req.Header.Set("X-Request-ID", requestID)

	// If provided, set VS Code specific headers
	if s.config.VSCodeMachineID != "" {
		req.Header.Set("Vscode-Machineid", s.config.VSCodeMachineID)
	}

	if s.config.VSCodeSessionID != "" {
		req.Header.Set("Vscode-Sessionid", s.config.VSCodeSessionID)
	}

	return s.httpClient.Do(req)
}

// generateRequestID creates a unique request ID for Copilot API calls
func generateRequestID() string {
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		time.Now().Unix(),
		rand.Intn(0x10000),
		rand.Intn(0x10000),
		rand.Intn(0x10000),
		rand.Intn(0x1000000))
}

// SubmitTestPrompt sends a test prompt to the GitHub Copilot API and returns the response.
func (s *Service) SubmitTestPrompt(prompt string) (string, error) {
	// Create a simple chat completion request
	requestData := map[string]interface{}{
		"model": "gpt-4o",
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": prompt,
			},
		},
		"temperature": 0.5,
		"max_tokens":  1000,
	}

	// Marshal the request to JSON
	providerRequest, err := json.Marshal(requestData)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Call the Copilot API
	resp, err := s.callCopilotAPI(string(providerRequest))
	if err != nil {
		return "", fmt.Errorf("API call failed: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// If response status is not successful, return the error
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API returned error: %s - %s", resp.Status, string(body))
	}

	// Parse the JSON response
	var responseData map[string]interface{}
	if err := json.Unmarshal(body, &responseData); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	// Extract the assistant's message content
	choices, ok := responseData["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	choice, ok := choices[0].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid choice format")
	}

	message, ok := choice["message"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid message format")
	}

	content, ok := message["content"].(string)
	if !ok {
		return "", fmt.Errorf("invalid content format")
	}

	return content, nil
}

// SubmitStreamingTestPrompt sends a test prompt to the GitHub Copilot API and streams the response to the terminal.
func (s *Service) SubmitStreamingTestPrompt(prompt string) error {
	// Create a simple chat completion request
	requestData := map[string]interface{}{
		"model": "gpt-4o",
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": prompt,
			},
		},
		"temperature": 0.5,
		"max_tokens":  1000,
		"stream":      true, // Enable streaming
	}

	// Marshal the request to JSON
	providerRequest, err := json.Marshal(requestData)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Call the Copilot API
	resp, err := s.callCopilotAPI(string(providerRequest))
	if err != nil {
		return fmt.Errorf("API call failed: %w", err)
	}
	defer resp.Body.Close()

	// If response status is not successful, return the error
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned error: %s - %s", resp.Status, string(body))
	}

	// Process the streaming response
	scanner := bufio.NewScanner(resp.Body)

	fmt.Println("\nStreaming response from Copilot API:")

	// Create a buffer to hold the complete response
	var fullResponse strings.Builder

	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines
		if line == "" {
			continue
		}

		// SSE format starts with "data: "
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		// Remove the "data: " prefix
		data := line[6:]

		// Check for the end of the stream
		if data == "[DONE]" {
			break
		}

		// Parse the JSON chunk
		var chunk map[string]interface{}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue // Skip malformed chunks
		}

		// Extract the delta content from the chunk
		choices, ok := chunk["choices"].([]interface{})
		if !ok || len(choices) == 0 {
			continue
		}

		choice, ok := choices[0].(map[string]interface{})
		if !ok {
			continue
		}

		delta, ok := choice["delta"].(map[string]interface{})
		if !ok {
			continue
		}

		content, ok := delta["content"].(string)
		if !ok || content == "" {
			continue
		}

		// Print the content chunk without a newline to create a stream effect
		fmt.Print(content)
		fullResponse.WriteString(content)
	}

	// Print a final newline
	fmt.Println()

	// Record usage statistics (simplified for CLI usage)
	s.RecordUsage(0, "gpt-4o", models.TokenUsage{
		Input:  100, // Simplified estimation
		Output: 100, // Simplified estimation
	})

	return scanner.Err()
}

// ProcessStreamingResponse processes a streaming response from the Copilot API
func (s *Service) ProcessStreamingResponse(resp *http.Response, userID uint64, model string) (io.ReadCloser, error) {
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("API returned error: %s", string(body))
	}

	// Record basic usage statistics (this is a simplified version)
	s.RecordUsage(userID, model, models.TokenUsage{
		Input:  100, // Simplified estimation
		Output: 100, // Simplified estimation
	})

	return resp.Body, nil
}
