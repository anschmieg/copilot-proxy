package llm

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

func TestCreateLLMToken(t *testing.T) {
	tests := []struct {
		name        string
		userID      uint64
		githubLogin string
		secret      string
		wantErr     bool
	}{
		{
			name:        "valid token creation",
			userID:      123,
			githubLogin: "testuser",
			secret:      "test-secret",
			wantErr:     false,
		},
		{
			name:        "empty secret",
			userID:      123,
			githubLogin: "testuser",
			secret:      "",
			wantErr:     false, // Empty secret is allowed but not recommended
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := CreateLLMToken(tt.userID, tt.githubLogin, tt.secret)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateLLMToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && token == "" {
				t.Error("CreateLLMToken() returned empty token")
			}
		})
	}
}

func TestValidateLLMToken(t *testing.T) {
	secret := "test-secret"
	validUserID := uint64(123)
	validGithubLogin := "testuser"

	// Create a valid token first
	validToken, err := CreateLLMToken(validUserID, validGithubLogin, secret)
	if err != nil {
		t.Fatalf("Failed to create test token: %v", err)
	}

	tests := []struct {
		name      string
		token     string
		secret    string
		wantErr   error
		checkUser bool
	}{
		{
			name:      "valid token",
			token:     validToken,
			secret:    secret,
			wantErr:   nil,
			checkUser: true,
		},
		{
			name:      "empty token",
			token:     "",
			secret:    secret,
			wantErr:   ErrInvalidToken,
			checkUser: false,
		},
		{
			name:      "malformed token",
			token:     "invalid.token.format",
			secret:    secret,
			wantErr:   ErrInvalidToken,
			checkUser: false,
		},
		{
			name:      "wrong secret",
			token:     validToken,
			secret:    "wrong-secret",
			wantErr:   ErrInvalidToken,
			checkUser: false,
		},
		{
			name:      "expired token",
			token:     createExpiredToken(validUserID, validGithubLogin, secret),
			secret:    secret,
			wantErr:   ErrTokenExpired,
			checkUser: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateLLMToken(tt.token, tt.secret)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("ValidateLLMToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.checkUser {
				if got == nil {
					t.Fatal("ValidateLLMToken() returned nil token")
				}
				if got.UserID != validUserID {
					t.Errorf("ValidateLLMToken() UserID = %v, want %v", got.UserID, validUserID)
				}
				if got.GithubUserLogin != validGithubLogin {
					t.Errorf("ValidateLLMToken() GithubUserLogin = %v, want %v", got.GithubUserLogin, validGithubLogin)
				}
			}
		})
	}
}

// Helper function to create an expired token
func createExpiredToken(userID uint64, githubLogin string, secret string) string {
	now := time.Now().Add(-2 * TokenLifetime * time.Second) // Set time to past expiry

	claims := TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(TokenLifetime * time.Second)),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        now.Format(time.RFC3339Nano),
		},
		UserID:          userID,
		GithubUserLogin: githubLogin,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString([]byte(secret))
	return tokenString
}
