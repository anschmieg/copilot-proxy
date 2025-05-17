package app

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestNewApp(t *testing.T) {
	app := NewApp()
	if app == nil {
		t.Fatal("NewApp returned nil")
	}
	if app.Router == nil {
		t.Error("Router not initialized")
	}
	if app.Auth == nil {
		t.Error("Auth service not initialized")
	}
}

func TestHandleStatus(t *testing.T) {
	app := NewApp()
	req := httptest.NewRequest("GET", "/status", nil)
	w := httptest.NewRecorder()

	app.handleStatus(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var respBody map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if _, ok := respBody["status"]; !ok {
		t.Error("Response missing status field")
	}
}

func TestHandleAuthenticate(t *testing.T) {
	app := NewApp()
	req := httptest.NewRequest("POST", "/authenticate", nil)
	w := httptest.NewRecorder()

	app.handleAuthenticate(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	// Test double authentication (should fail)
	w = httptest.NewRecorder()
	app.handleAuthenticate(w, req)
	resp = w.Result()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected status code %d for double auth, got %d", http.StatusUnauthorized, resp.StatusCode)
	}
}

func TestHandleStream(t *testing.T) {
	app := NewApp()
	req := httptest.NewRequest("GET", "/stream", nil)
	w := httptest.NewRecorder()

	app.handleStream(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	if resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Errorf("Expected Content-Type %s, got %s", "text/event-stream", resp.Header.Get("Content-Type"))
	}

	body, _ := io.ReadAll(resp.Body)
	responses := strings.Split(string(body), "\n\n")
	responses = responses[:len(responses)-1] // Remove last empty element
	if len(responses) != 5 {
		t.Errorf("Expected 5 stream responses, got %d", len(responses))
	}
}

func TestHandleCopilot(t *testing.T) {
	tests := []struct {
		name           string
		authHeader     string
		body           map[string]interface{}
		setupEnv       func()
		cleanupEnv     func()
		expectedStatus int
	}{
		{
			name:       "missing auth header",
			authHeader: "",
			body: map[string]interface{}{
				"model":    "copilot-chat",
				"messages": []map[string]string{{"role": "user", "content": "Hello"}},
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:       "invalid auth token",
			authHeader: "Bearer invalid-token",
			body: map[string]interface{}{
				"model":    "copilot-chat",
				"messages": []map[string]string{{"role": "user", "content": "Hello"}},
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:       "valid token with disabled auth",
			authHeader: "Bearer any-token",
			body: map[string]interface{}{
				"model":    "copilot-chat",
				"messages": []map[string]string{{"role": "user", "content": "Hello"}},
			},
			setupEnv: func() {
				os.Setenv("DISABLE_AUTH", "true")
			},
			cleanupEnv: func() {
				os.Unsetenv("DISABLE_AUTH")
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:       "invalid request body",
			authHeader: "Bearer valid-token",
			body:       map[string]interface{}{},
			setupEnv: func() {
				os.Setenv("DISABLE_AUTH", "true")
			},
			cleanupEnv: func() {
				os.Unsetenv("DISABLE_AUTH")
			},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupEnv != nil {
				tt.setupEnv()
			}
			defer func() {
				if tt.cleanupEnv != nil {
					tt.cleanupEnv()
				}
			}()

			app := NewApp()
			bodyBytes, _ := json.Marshal(tt.body)
			req := httptest.NewRequest("POST", "/copilot", bytes.NewBuffer(bodyBytes))
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			w := httptest.NewRecorder()

			app.handleCopilot(w, req)

			resp := w.Result()
			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("Expected status code %d, got %d", tt.expectedStatus, resp.StatusCode)
			}
		})
	}
}

func TestGetAPIKey(t *testing.T) {
	tests := []struct {
		name          string
		oauthToken    string
		expectedError bool
	}{
		{
			name:          "empty oauth token",
			oauthToken:    "",
			expectedError: true,
		},
		{
			name:          "invalid oauth token",
			oauthToken:    "invalid-token",
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := NewApp()
			_, err := app.GetAPIKey(tt.oauthToken)
			if (err != nil) != tt.expectedError {
				t.Errorf("GetAPIKey() error = %v, expectedError %v", err, tt.expectedError)
			}
		})
	}
}

func TestGetCopilotAPIKey(t *testing.T) {
	tests := []struct {
		name           string
		setupEnv       func()
		cleanupEnv     func()
		expectedError  bool
		expectedOutput string
	}{
		{
			name: "valid key in env",
			setupEnv: func() {
				os.Setenv("COPILOT_API_KEY", "tid=test;exp="+string(rune(time.Now().Add(time.Hour).Unix()))+";sku=pro")
			},
			cleanupEnv: func() {
				os.Unsetenv("COPILOT_API_KEY")
			},
			expectedError: false,
		},
		{
			name: "expired key in env",
			setupEnv: func() {
				os.Setenv("COPILOT_API_KEY", "tid=test;exp="+string(rune(time.Now().Add(-time.Hour).Unix()))+";sku=pro")
			},
			cleanupEnv: func() {
				os.Unsetenv("COPILOT_API_KEY")
			},
			expectedError: true,
		},
		{
			name:          "no key in env",
			setupEnv:      func() {},
			cleanupEnv:    func() {},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupEnv()
			defer tt.cleanupEnv()

			app := NewApp()
			result, err := app.GetCopilotAPIKey()
			if (err != nil) != tt.expectedError {
				t.Errorf("GetCopilotAPIKey() error = %v, expectedError %v", err, tt.expectedError)
			}
			if !tt.expectedError && result == "" {
				t.Error("GetCopilotAPIKey() returned empty string but no error")
			}
		})
	}
}

func TestTestAPI(t *testing.T) {
	app := NewApp()
	testPayload := "test message"

	response, err := app.TestAPI(testPayload)
	if err != nil {
		t.Errorf("TestAPI() error = %v", err)
	}

	expected := "Test call successful: " + testPayload
	if response != expected {
		t.Errorf("TestAPI() = %v, want %v", response, expected)
	}
}
