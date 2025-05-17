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
	"os"
	"strings"
	"time"
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

// Helper for OpenAI-style error responses
func writeOpenAIError(w http.ResponseWriter, status int, message, errType string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"message": message,
			"type":    errType,
			"param":   nil,
			"code":    nil,
		},
	})
}

// HandleListModels handles the list models endpoint
func (s *ServerState) HandleListModels(w http.ResponseWriter, r *http.Request) {
	token, err := s.validateToken(r)
	if err != nil {
		if errors.Is(err, ErrTokenExpired) {
			w.Header().Set("X-LLM-Token-Expired", "true")
			writeOpenAIError(w, http.StatusUnauthorized, "token expired", "invalid_request_error")
		} else {
			writeOpenAIError(w, http.StatusUnauthorized, "unauthorized", "invalid_request_error")
		}
		return
	}

	countryCode := getCountryCode(r)

	// --- Directly proxy the upstream Copilot API response, but filter if needed ---
	apiKey := s.Service.config.CopilotAPIKey
	if apiKey == "" {
		writeOpenAIError(w, http.StatusInternalServerError, "missing Copilot API key", "internal_error")
		return
	}
	reqURL := s.Service.getProxyURL(CopilotModelsURL)
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		writeOpenAIError(w, http.StatusBadGateway, "failed to create models request: "+err.Error(), "api_error")
		return
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	editorVersion := s.Service.config.EditorVersion
	if editorVersion == "" {
		editorVersion = "vscode/1.99.2"
	}
	pluginVersion := s.Service.config.EditorPluginVersion
	if pluginVersion == "" {
		pluginVersion = "copilot-chat/0.26.3"
	}
	req.Header.Set("Editor-Version", editorVersion)
	req.Header.Set("Editor-Plugin-Version", pluginVersion)
	req.Header.Set("Copilot-Integration-ID", "vscode-chat")
	req.Header.Set("User-Agent", "GitHubCopilotChat/"+strings.TrimPrefix(pluginVersion, "copilot-chat/"))
	req.Header.Set("OpenAI-Intent", "conversation-agent")
	req.Header.Set("X-GitHub-API-Version", "2025-04-01")

	resp, err := s.Service.httpClient.Do(req)
	if err != nil {
		writeOpenAIError(w, http.StatusBadGateway, "failed to fetch models: "+err.Error(), "api_error")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		writeOpenAIError(w, http.StatusBadGateway, "models API returned "+resp.Status+": "+string(body), "api_error")
		return
	}

	// Read the upstream response as raw JSON
	var upstream struct {
		Object string                   `json:"object"`
		Data   []map[string]interface{} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&upstream); err != nil {
		writeOpenAIError(w, http.StatusBadGateway, "failed to decode models response: "+err.Error(), "api_error")
		return
	}

	// Filter models according to authorization/country if needed
	filtered := make([]map[string]interface{}, 0, len(upstream.Data))
	for _, model := range upstream.Data {
		provider, _ := model["provider"].(string)
		name, _ := model["name"].(string)
		if err := AuthorizeAccessForCountry(countryCode, models.LanguageModelProvider(provider)); err == nil {
			if err := AuthorizeAccessToModel(token, models.LanguageModelProvider(provider), name); err == nil {
				// Ensure "object": "model" is present for OpenAI compatibility
				model["object"] = "model"
				filtered = append(filtered, model)
			}
		}
	}

	out := map[string]interface{}{
		"object": "list",
		"data":   filtered,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

// HandleCompletion handles the completion endpoint
func (s *ServerState) HandleCompletion(w http.ResponseWriter, r *http.Request) {
	// Track if client requested streaming
	var isStream bool

	token, err := s.validateToken(r)
	if err != nil {
		if errors.Is(err, ErrTokenExpired) {
			w.Header().Set("X-LLM-Token-Expired", "true")
			writeOpenAIError(w, http.StatusUnauthorized, "token expired", "invalid_request_error")
		} else {
			writeOpenAIError(w, http.StatusUnauthorized, "unauthorized", "invalid_request_error")
		}
		return
	}

	// Read the request body
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		writeOpenAIError(w, http.StatusBadRequest, "error reading request body", "invalid_request_error")
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
			writeOpenAIError(w, http.StatusBadRequest, "invalid request body: "+err.Error(), "invalid_request_error")
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
			writeOpenAIError(w, http.StatusInternalServerError, "error formatting request: "+err.Error(), "internal_error")
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
		writeOpenAIError(w, http.StatusBadRequest, err.Error(), "invalid_request_error")
		return
	}

	defer resp.Body.Close()
	// Process streaming SSE for both modes
	reader, err := s.Service.ProcessStreamingResponse(resp, token.UserID, params.Model)
	if err != nil {
		writeOpenAIError(w, http.StatusInternalServerError, err.Error(), "internal_error")
		return
	}
	defer reader.Close()
	if !isStream {
		// Accumulate all chunks into one message
		var full strings.Builder
		var usage struct {
			PromptTokens     int
			CompletionTokens int
			TotalTokens      int
		}
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
			// Try to extract usage if present
			if u, ok := chunk["usage"].(map[string]interface{}); ok {
				if v, ok := u["prompt_tokens"].(float64); ok {
					usage.PromptTokens = int(v)
				}
				if v, ok := u["completion_tokens"].(float64); ok {
					usage.CompletionTokens = int(v)
				}
				if v, ok := u["total_tokens"].(float64); ok {
					usage.TotalTokens = int(v)
				}
			}
		}
		// Write OpenAI-compliant response
		w.Header().Set("Content-Type", "application/json")
		now := time.Now().Unix()
		id := fmt.Sprintf("chatcmpl-%d%06d", now, rand.Intn(1000000))
		out := map[string]interface{}{
			"id":      id,
			"object":  "chat.completion",
			"created": now,
			"model":   params.Model,
			"choices": []map[string]interface{}{{
				"message":       map[string]string{"role": "assistant", "content": full.String()},
				"finish_reason": "stop", "index": 0,
			}},
			"usage": map[string]interface{}{
				"prompt_tokens":     usage.PromptTokens,
				"completion_tokens": usage.CompletionTokens,
				"total_tokens":      usage.TotalTokens,
			},
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
	mux.HandleFunc("/v1/models", s.HandleListModels) // OpenAI alias
	mux.HandleFunc("/completion", s.HandleCompletion)
	mux.HandleFunc("/openai", s.HandleCompletion)
	mux.HandleFunc("/v1/chat/completions", s.HandleCompletion)
	// (Optional) Add /v1/completions and /v1/embeddings handlers here if implemented
}
