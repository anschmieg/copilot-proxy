package llm

import (
	"bufio"
	"bytes"
	"copilot-proxy/pkg/models"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
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

	// Fetch the actual list of models from Copilot API
	availableModels, err := s.Service.FetchModels()
	if err != nil {
		http.Error(w, "failed to fetch models: "+err.Error(), http.StatusBadGateway)
		return
	}

	accessibleModels := []models.LanguageModel{}

	for _, model := range availableModels {
		if err := AuthorizeAccessForCountry(countryCode, model.Provider); err == nil {
			if err := AuthorizeAccessToModel(token, model.Provider, model.Name); err == nil {
				accessibleModels = append(accessibleModels, model)
			}
		}
	}

	response := ListModelsResponse{Models: accessibleModels}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleCompletion handles the completion endpoint
func (s *ServerState) HandleCompletion(w http.ResponseWriter, r *http.Request) {
	// Track if client requested streaming
	var isStream bool

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

	// Remove any 'stream' from incoming payload before processing
	var incoming map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &incoming); err == nil {
		// Determine if streaming was requested
		isStream, _ = incoming["stream"].(bool)
		// Clean out the stream key for internal processing
		delete(incoming, "stream")
		// Re-marshal to remove 'stream' from bodyBytes
		cleanBody, err2 := json.Marshal(incoming)
		if err2 == nil {
			bodyBytes = cleanBody
		}

		// Use this for branching later
		r = r.Clone(r.Context())
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	} else {
		// Fall back if unmarshal fails
		isStream = false
	}

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

	// Always use streaming on the Copilot API side
	resp, err := s.Service.PerformCompletion(req)
	if err != nil {
		SetErrorResponseHeaders(w, err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	defer resp.Body.Close()
	// Process streaming SSE for both modes
	reader, err := s.Service.ProcessStreamingResponse(resp, token.UserID, params.Model)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer reader.Close()
	if !isStream {
		// Accumulate all chunks into one message
		var full strings.Builder
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				break
			}
			var chunk map[string]interface{}
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}
			choices, ok := chunk["choices"].([]interface{})
			if !ok || len(choices) == 0 {
				continue
			}
			choice, _ := choices[0].(map[string]interface{})
			delta, _ := choice["delta"].(map[string]interface{})
			if content, ok := delta["content"].(string); ok {
				full.WriteString(content)
			}
		}
		// Write JSON response
		w.Header().Set("Content-Type", "application/json")
		out := map[string]interface{}{ // minimal OpenAI response
			"choices": []map[string]interface{}{{
				"message":       map[string]string{"role": "assistant", "content": full.String()},
				"finish_reason": "stop", "index": 0,
			}},
		}
		json.NewEncoder(w).Encode(out)
		return
	}
	// Streaming SSE: proxy raw event stream line-by-line with flush
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, _ := w.(http.Flusher)
	bufReader := bufio.NewReader(reader)
	for {
		line, err := bufReader.ReadBytes('\n')
		if len(line) > 0 {
			w.Write(line)
			flusher.Flush()
		}
		if err != nil {
			break
		}
	}
	return
}

// RegisterHandlers registers the LLM handlers with a router
func (s *ServerState) RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/models", s.HandleListModels)
	mux.HandleFunc("/completion", s.HandleCompletion)

	// Add OpenAI-compatible paths
	mux.HandleFunc("/openai", s.HandleCompletion)
	mux.HandleFunc("/v1/chat/completions", s.HandleCompletion)
}
