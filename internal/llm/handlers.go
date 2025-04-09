package llm

import (
	"copilot-proxy/pkg/models"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

// ServerState holds the state for the LLM server
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
	Provider        models.LanguageModelProvider `json:"provider"`
	Model           string                       `json:"model"`
	ProviderRequest string                       `json:"provider_request"` // Raw JSON payload
}

// validateToken extracts and validates the LLM token from a request
func (s *ServerState) validateToken(r *http.Request) (*models.LLMToken, error) {
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
		if err := AuthorizeAccessForCountry(countryCode, model.Provider); err == nil {
			// Check if model is available in the user's plan
			if err := AuthorizeAccessToModel(token, model.Provider, model.Name); err == nil {
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

	var params CompletionParams
	err = json.NewDecoder(r.Body).Decode(&params)
	if err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	countryCode := getCountryCode(r)

	// In a real implementation, we would fetch the current spending from a database
	// Here we'll use a placeholder value
	currentSpending := uint32(0)

	req := CompletionRequest{
		Provider:        params.Provider,
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
	reader, err := s.Service.ProcessStreamingResponse(resp, token.UserID, params.Provider, params.Model)
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
}
