package llm

import (
	"bytes"
	"copilot-proxy/pkg/models"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
)

// ServerState holds the state for the Copilot LLM server
type ServerState struct {
	Service *Service
	Secret  string
}

// NewLLMServerState creates a new LLM server state
func NewLLMServerState(secret string) *ServerState {
	return &ServerState{
		Service: NewService(),
		Secret:  secret,
	}
}

// ListModelsResponse is the response for the list models endpoint
type ListModelsResponse struct {
	Models []models.LanguageModel `json:"models"`
}

// CompletionParams are the parameters for a completion request
type CompletionParams struct {
	Model           string `json:"model"`
	ProviderRequest string `json:"provider_request"` // Raw JSON payload
}

// validateToken extracts and validates the LLM token from a request
func (s *ServerState) validateToken(r *http.Request) (*models.LLMToken, error) {
	// Check if auth is disabled globally
	if disableAuth := os.Getenv("DISABLE_AUTH"); disableAuth == "true" || disableAuth == "1" {
		// Return a default admin token when auth is disabled
		return &models.LLMToken{
			UserID:                 1,
			GithubUserLogin:        "disabled-auth-user",
			IsStaff:                true,
			HasLLMSubscription:     true,
			MaxMonthlySpendInCents: 10000,
		}, nil
	}

	auth := r.Header.Get("Authorization")
	if auth == "" || len(auth) < 7 || auth[:7] != "Bearer " {
		return nil, errors.New("invalid or missing authorization header")
	}

	token, err := ValidateLLMToken(auth[7:], s.Secret)
	if err != nil {
		return nil, err
	}

	return token, nil
}

// getCountryCode extracts country code from a request header
func getCountryCode(r *http.Request) *string {
	country := r.Header.Get("CF-IPCountry")
	if country == "" || country == "XX" {
		return nil
	}

	return &country
}

// HandleListModels handles the list models endpoint
func (s *ServerState) HandleListModels(w http.ResponseWriter, r *http.Request) {
	token, err := s.validateToken(r)
	if err != nil {
		if errors.Is(err, ErrTokenExpired) {
			w.Header().Set("X-LLM-Token-Expired", "true")
			http.Error(w, "token expired", http.StatusUnauthorized)
		} else {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		}
		return
	}

	countryCode := getCountryCode(r)

	availableModels := DefaultModels()
	var accessibleModels []models.LanguageModel

	for _, model := range availableModels {
		// Check if model is accessible from this country code
		if err := AuthorizeAccessForCountry(countryCode, models.ProviderCopilot); err == nil {
			// Check if model is available in the user's plan
			if err := AuthorizeAccessToModel(token, models.ProviderCopilot, model.Name); err == nil {
				accessibleModels = append(accessibleModels, model)
			}
		}
	}

	response := ListModelsResponse{
		Models: accessibleModels,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleCompletion handles the completion endpoint
func (s *ServerState) HandleCompletion(w http.ResponseWriter, r *http.Request) {
	token, err := s.validateToken(r)
	if err != nil {
		if errors.Is(err, ErrTokenExpired) {
			w.Header().Set("X-LLM-Token-Expired", "true")
			http.Error(w, "token expired", http.StatusUnauthorized)
		} else {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		}
		return
	}

	// Read the request body
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "error reading request body", http.StatusBadRequest)
		return
	}
	r.Body.Close()

	// Create a new reader for the request body for later use
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	// First, try to parse as standard CompletionParams
	var params CompletionParams
	if err := json.Unmarshal(bodyBytes, &params); err != nil || params.ProviderRequest == "" {
		// If that fails, the request might be in OpenAI format
		// Convert the OpenAI format to our internal format
		var openAIRequest map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &openAIRequest); err != nil {
			http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Set the model if present, otherwise use a default
		model, _ := openAIRequest["model"].(string)
		if model == "" {
			model = "copilot-chat" // Default model
		}

		// Set provider to copilot if not specified
		if _, ok := openAIRequest["provider"]; !ok {
			openAIRequest["provider"] = "copilot"
		}

		// Convert the request to a string for our internal format
		providerRequestBytes, err := json.Marshal(openAIRequest)
		if err != nil {
			http.Error(w, "error formatting request: "+err.Error(), http.StatusInternalServerError)
			return
		}

		params = CompletionParams{
			Model:           model,
			ProviderRequest: string(providerRequestBytes),
		}
	}

	countryCode := getCountryCode(r)

	// In a real implementation, we would fetch the current spending from a database
	// Here we'll use a placeholder value
	currentSpending := uint32(0)

	req := CompletionRequest{
		Model:           params.Model,
		ProviderRequest: params.ProviderRequest,
		Token:           token,
		CountryCode:     countryCode,
		CurrentSpending: currentSpending,
	}

	resp, err := s.Service.PerformCompletion(req)
	if err != nil {
		SetErrorResponseHeaders(w, err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	defer resp.Body.Close()

	// Set up streaming response
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Process and stream the response
	reader, err := s.Service.ProcessStreamingResponse(resp, token.UserID, params.Model)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	defer reader.Close()

	// Copy the reader to the response writer
	_, err = io.Copy(w, reader)
	if err != nil {
		// Connection likely closed by client, just log it
		return
	}
}

// RegisterHandlers registers the LLM handlers with a router
func (s *ServerState) RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/models", s.HandleListModels)
	mux.HandleFunc("/completion", s.HandleCompletion)

	// Add OpenAI-compatible paths
	mux.HandleFunc("/openai", s.HandleCompletion)
	mux.HandleFunc("/v1/chat/completions", s.HandleCompletion)
}
