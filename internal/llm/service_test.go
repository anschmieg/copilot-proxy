package llm

import (
	"copilot-proxy/pkg/models"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewService(t *testing.T) {
	service := NewService()
	if service == nil {
		t.Fatal("NewService() returned nil")
	}
	if service.config == nil {
		t.Error("NewService() returned service with nil config")
	}
	if service.httpClient == nil {
		t.Error("NewService() returned service with nil httpClient")
	}
	if service.userUsage == nil {
		t.Error("NewService() returned service with nil userUsage map")
	}
}

func TestGetProxyEndpoint(t *testing.T) {
	tests := []struct {
		name   string
		apiKey string
		want   string
	}{
		{
			name:   "default endpoint",
			apiKey: "test-key",
			want:   "api.githubcopilot.com",
		},
		{
			name:   "custom endpoint",
			apiKey: "test-key;proxy-ep=custom.endpoint.com",
			want:   "custom.endpoint.com",
		},
		{
			name:   "multiple fields",
			apiKey: "tid=123;proxy-ep=api2.copilot.com;exp=123456",
			want:   "api2.copilot.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Service{
				config: &Config{CopilotAPIKey: tt.apiKey},
			}
			if got := s.getProxyEndpoint(); got != tt.want {
				t.Errorf("getProxyEndpoint() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRecordAndGetModelUsage(t *testing.T) {
	s := NewService()
	userID := uint64(1)
	model := "test-model"
	usage := models.TokenUsage{
		Input:  100,
		Output: 50,
	}

	// Record usage
	s.RecordUsage(userID, model, usage)

	// Get usage
	got := s.GetModelUsage(userID, model)

	if got.TokensThisMinute != usage.Input+usage.Output {
		t.Errorf("GetModelUsage() TokensThisMinute = %v, want %v", got.TokensThisMinute, usage.Input+usage.Output)
	}
	if got.RequestsThisMinute != 1 {
		t.Errorf("GetModelUsage() RequestsThisMinute = %v, want 1", got.RequestsThisMinute)
	}
}

func TestPerformCompletion(t *testing.T) {
	// Create a test server that mimics the Copilot API
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request headers
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("Missing or invalid Authorization header")
		}
		if !strings.Contains(r.Header.Get("Editor-Version"), "vscode") {
			t.Error("Missing or invalid Editor-Version header")
		}
		if !strings.Contains(r.Header.Get("Editor-Plugin-Version"), "copilot-chat") {
			t.Error("Missing or invalid Editor-Plugin-Version header")
		}

		// Return a mock response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "test-completion",
			"choices": []map[string]interface{}{
				{
					"message": map[string]string{
						"role":    "assistant",
						"content": "Test response",
					},
				},
			},
		})
	}))
	defer ts.Close()

	s := &Service{
		config: &Config{
			CopilotAPIKey: "test-key",
		},
		httpClient: ts.Client(),
		modelsCache: []models.LanguageModel{
			{
				ID:       "test-model",
				Name:     "test-model",
				Provider: models.ProviderCopilot,
				Enabled:  true,
			},
		},
	}

	req := CompletionRequest{
		Model:           "test-model",
		ProviderRequest: `{"messages":[{"role":"user","content":"test"}]}`,
		Token: &models.LLMToken{
			UserID: 1,
		},
	}

	resp, err := s.PerformCompletion(req)
	if err != nil {
		t.Fatalf("PerformCompletion() error = %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("PerformCompletion() status = %v, want %v", resp.StatusCode, http.StatusOK)
	}
}

func TestGenerateRequestID(t *testing.T) {
	id1 := generateRequestID()
	id2 := generateRequestID()

	if id1 == "" {
		t.Error("generateRequestID() returned empty string")
	}
	if id1 == id2 {
		t.Error("generateRequestID() returned same ID twice")
	}
	if len(strings.Split(id1, "-")) != 5 {
		t.Errorf("generateRequestID() returned ID with wrong format: %s", id1)
	}
}
