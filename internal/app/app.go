package app

import (
	"copilot-proxy/internal/auth"
	"copilot-proxy/pkg/utils"
	"encoding/json"
	"fmt"
	"net/http"
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
	a.Router.HandleFunc("/openai", a.handleOpenAI)
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

func (a *App) handleOpenAI(w http.ResponseWriter, r *http.Request) {
	apiKey := r.Header.Get("Authorization")
	if apiKey == "" {
		http.Error(w, "Missing API key", http.StatusUnauthorized)
		return
	}
	// Verify API key using the auth module
	if !auth.VerifyAPIKey(apiKey) {
		http.Error(w, "Invalid API key", http.StatusUnauthorized)
		return
	}

	var payload map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	response, err := utils.CallOpenAIEndpoint(apiKey, payload)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetAPIKey retrieves an API key using the provided OAuth token.
func (a *App) GetAPIKey(oauthToken string) (string, error) {
	req, err := http.NewRequest("GET", "https://example.com/api/get_llm_api_token", nil)
	if err != nil {
		return "", err
	}

	// Add the OAuth token to the Authorization header
	req.Header.Set("Authorization", "Bearer "+oauthToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to retrieve API key: %s", resp.Status)
	}

	var response struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", err
	}

	return response.Token, nil
}
