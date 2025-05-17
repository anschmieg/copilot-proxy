package llm

import (
	"copilot-proxy/pkg/models"
	"errors"
	"net/http/httptest"
	"testing"
)

func TestAuthorizeAccessToModel(t *testing.T) {
	token := &models.LLMToken{
		UserID:          1,
		GithubUserLogin: "testuser",
	}

	tests := []struct {
		name      string
		token     *models.LLMToken
		provider  models.LanguageModelProvider
		modelName string
		wantErr   bool
	}{
		{
			name:      "valid access",
			token:     token,
			provider:  models.ProviderCopilot,
			modelName: "copilot-chat",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := AuthorizeAccessToModel(tt.token, tt.provider, tt.modelName)
			if (err != nil) != tt.wantErr {
				t.Errorf("AuthorizeAccessToModel() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAuthorizeAccessForCountry(t *testing.T) {
	tests := []struct {
		name        string
		countryCode *string
		provider    models.LanguageModelProvider
		wantErr     error
	}{
		{
			name:        "nil country code",
			countryCode: nil,
			provider:    models.ProviderCopilot,
			wantErr:     nil,
		},
		{
			name:        "unknown country",
			countryCode: strPtr("XX"),
			provider:    models.ProviderCopilot,
			wantErr:     nil,
		},
		{
			name:        "allowed country",
			countryCode: strPtr("US"),
			provider:    models.ProviderCopilot,
			wantErr:     nil,
		},
		{
			name:        "restricted country",
			countryCode: strPtr("IR"),
			provider:    models.ProviderCopilot,
			wantErr:     ErrRestrictedRegion,
		},
		{
			name:        "TOR network",
			countryCode: strPtr("T1"),
			provider:    models.ProviderCopilot,
			wantErr:     ErrTorNetwork,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := AuthorizeAccessForCountry(tt.countryCode, tt.provider)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("AuthorizeAccessForCountry() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCheckRateLimit(t *testing.T) {
	tests := []struct {
		name      string
		modelName string
		usage     models.ModelUsage
		wantErr   error
	}{
		{
			name:      "under limits",
			modelName: "copilot-chat",
			usage: models.ModelUsage{
				RequestsThisMinute:     10,
				TokensThisMinute:       1000,
				InputTokensThisMinute:  500,
				OutputTokensThisMinute: 500,
				TokensThisDay:          5000,
			},
			wantErr: nil,
		},
		{
			name:      "unknown model",
			modelName: "nonexistent-model",
			usage:     models.ModelUsage{},
			wantErr:   errors.New("unknown model: nonexistent-model"),
		},
		{
			name:      "requests exceeded",
			modelName: "copilot-chat",
			usage: models.ModelUsage{
				RequestsThisMinute: 30, // Default limit is 25
			},
			wantErr: ErrRateLimitExceeded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckRateLimit(tt.modelName, tt.usage)
			if tt.wantErr == nil && err != nil {
				t.Errorf("CheckRateLimit() unexpected error = %v", err)
			} else if tt.wantErr != nil && err == nil {
				t.Errorf("CheckRateLimit() expected error = %v, got nil", tt.wantErr)
			} else if tt.wantErr != nil && err != nil && tt.wantErr.Error() != err.Error() {
				t.Errorf("CheckRateLimit() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSetErrorResponseHeaders(t *testing.T) {
	tests := []struct {
		name          string
		err           error
		wantHeader    string
		wantHasHeader bool
	}{
		{
			name:          "rate limit error",
			err:           ErrRateLimitExceeded,
			wantHeader:    "60",
			wantHasHeader: true,
		},
		{
			name:          "other error",
			err:           errors.New("random error"),
			wantHasHeader: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			SetErrorResponseHeaders(w, tt.err)

			retryAfter := w.Header().Get("Retry-After")
			hasHeader := retryAfter != ""

			if hasHeader != tt.wantHasHeader {
				t.Errorf("SetErrorResponseHeaders() header present = %v, want %v", hasHeader, tt.wantHasHeader)
			}
			if tt.wantHasHeader && retryAfter != tt.wantHeader {
				t.Errorf("SetErrorResponseHeaders() header = %v, want %v", retryAfter, tt.wantHeader)
			}
		})
	}
}

func TestValidateAccess(t *testing.T) {
	token := &models.LLMToken{
		UserID:             1,
		GithubUserLogin:    "testuser",
		IsStaff:            false,
		HasLLMSubscription: true,
	}

	tests := []struct {
		name      string
		modelName string
		usage     models.ModelUsage
		wantErr   bool
	}{
		{
			name:      "default validation",
			modelName: "copilot-chat",
			usage: models.ModelUsage{
				RequestsThisMinute: 100,
				TokensThisMinute:   10000,
				TokensThisDay:      200000,
			},
			wantErr: false, // Personal use always allows access
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAccess(token, tt.modelName, tt.usage)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAccess() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Helper function to create string pointer
func strPtr(s string) *string {
	return &s
}
