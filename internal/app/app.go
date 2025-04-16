package app

import (
	"copilot-proxy/internal/auth"
	"copilot-proxy/pkg/utils"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// App represents the main application with its router and authentication service.
type App struct {
	Router *http.ServeMux
	Auth   *auth.Service
}

// NewApp creates and initializes a new instance of the App struct.
func NewApp() *App {
	app := &App{
		Router: http.NewServeMux(),
		Auth:   auth.NewService(),
	}

	app.initializeRoutes()
	return app
}

func (a *App) initializeRoutes() {
	a.Router.HandleFunc("/status", a.handleStatus)
	a.Router.HandleFunc("/authenticate", a.handleAuthenticate)
	a.Router.HandleFunc("/stream", a.handleStream)
	a.Router.HandleFunc("/copilot", a.handleCopilot)
}

func (a *App) handleStatus(w http.ResponseWriter, r *http.Request) {
	status := a.Auth.GetStatus()
	json.NewEncoder(w).Encode(map[string]string{"status": status})
}

func (a *App) handleAuthenticate(w http.ResponseWriter, r *http.Request) {
	err := a.Auth.Authenticate()
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Authenticated successfully"))
}

func (a *App) handleStream(w http.ResponseWriter, r *http.Request) {
	limiter := utils.NewRateLimiter()
	// Define a custom rate limit for stream requests
	rateLimit := utils.NewBasicRateLimit(4, time.Minute, "stream-requests")
	// Pass the rate limit and a default userID (1 for system)
	if !limiter.Check(rateLimit, 1) {
		http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)

	for i := 0; i < 5; i++ {
		w.Write([]byte("data: Streaming response...\n\n"))
		w.(http.Flusher).Flush()
	}
}

func (a *App) handleCopilot(w http.ResponseWriter, r *http.Request) {
	// Extract API key from the Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, "Missing API key", http.StatusUnauthorized)
		return
	}

	// Handle different Authorization header formats
	var apiKey string
	if strings.HasPrefix(authHeader, "Bearer ") {
		apiKey = strings.TrimPrefix(authHeader, "Bearer ")
	} else if strings.HasPrefix(authHeader, "Bearer: ") {
		apiKey = strings.TrimPrefix(authHeader, "Bearer: ")
	} else {
		// Assume the entire header value is the API key
		apiKey = authHeader
	}

	fmt.Printf("Extracted API key: %s\n", apiKey)

	// Verify that this is a valid app API key
	if !auth.VerifyAppAPIKey(apiKey) {
		http.Error(w, "Invalid API key", http.StatusUnauthorized)
		return
	}

	var payload map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	// Format the payload for OpenAI-compatible request if needed
	providerRequest := payload
	if _, ok := payload["messages"]; ok {
		// If payload contains messages directly, wrap it in a provider_request
		providerRequest = map[string]interface{}{
			"model":            payload["model"],
			"provider_request": payload,
		}
	}

	// Get a valid Copilot API key using our prioritized logic
	copilotKey, err := a.GetCopilotAPIKey()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// For debugging
	fmt.Printf("Using Copilot API key: %s\n", copilotKey)

	// Make the request to the Copilot API
	response, err := utils.CallCopilotAPI(copilotKey, providerRequest)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetAPIKey retrieves an API key using the provided GitHub OAuth token.
// It makes a request to the GitHub Copilot API endpoint to obtain a token
// that can be used for subsequent API calls.
//
// Parameters:
//   - oauthToken: A GitHub OAuth token with the required scopes for Copilot access
//
// Returns:
//   - string: The Copilot API token if successful
//   - error: An error if the request fails
//
// The returned token contains multiple components including:
//   - tid: Token ID
//   - exp: Expiration timestamp
//   - sku: Subscription type (e.g., free_educational)
//   - proxy-ep: Proxy endpoint for API calls
//   - Various feature flags (chat, cit, malfil, etc.)
func (a *App) GetAPIKey(oauthToken string) (string, error) {
	// GitHub Copilot API endpoint for getting a token
	copilotTokenURL := "https://api.github.com/copilot_internal/v2/token"

	req, err := http.NewRequest("GET", copilotTokenURL, nil)
	if err != nil {
		return "", err
	}

	// Add the OAuth token to the Authorization header
	req.Header.Set("Authorization", "token "+oauthToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "copilot-proxy")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to retrieve API key: %s - %s", resp.Status, string(bodyBytes))
	}

	var response struct {
		Token     string      `json:"token"`
		ExpiresAt json.Number `json:"expires_at"` // Using json.Number to handle both string and numeric formats
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", err
	}

	return response.Token, nil
}

// GetCopilotAPIKey retrieves a valid GitHub Copilot API key following a priority order:
// 1. First check for direct API key in environment variables
// 2. Then try to use OAuth token from environment to get an API key
// 3. Finally try to read OAuth token from Copilot config and use it to get an API key
//
// Returns the Copilot API key if successful or an error if all methods fail.
func (a *App) GetCopilotAPIKey() (string, error) {
	// Step 1: Check if we already have a Copilot API key in environment variables
	apiKey := os.Getenv("COPILOT_API_KEY")
	if apiKey != "" {
		// Verify the token hasn't expired
		if auth.VerifyCopilotAPIKey(apiKey) {
			return apiKey, nil
		}
		// If token has expired, continue to try other methods
		fmt.Println("Copilot API key from environment variables has expired, trying OAuth token...")
	}

	// Step 2: Try to get an OAuth token from environment variables
	oauthToken, err := utils.GetCopilotOAuthToken()
	if err == nil && oauthToken != "" {
		fmt.Println("Found OAuth token in environment variables, attempting to get Copilot API key...")
		apiKey, err := a.GetAPIKey(oauthToken)
		if err == nil {
			// Cache the API key for future use
			os.Setenv("COPILOT_API_KEY", apiKey)
			return apiKey, nil
		}
		fmt.Printf("Failed to get Copilot API key using OAuth token: %v\n", err)
	}

	// Step 3: Attempt to use the local Copilot token from config
	apiKey, err = utils.GetCopilotToken()
	if err == nil {
		return apiKey, nil
	}

	return "", errors.New("failed to retrieve Copilot API key: no valid source found. Set COPILOT_API_KEY or COPILOT_OAUTH_TOKEN environment variables")
}

// TestAPI makes a test call to verify the API is working.
func (a *App) TestAPI(payload string) (string, error) {
	// Example implementation: Echo the payload back as a response
	return fmt.Sprintf("Test call successful: %s", payload), nil
}
