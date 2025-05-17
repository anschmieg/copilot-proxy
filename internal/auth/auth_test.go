package auth

import (
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestNewService(t *testing.T) {
	svc := NewService()
	if svc == nil {
		t.Fatal("NewService returned nil")
	}
	if svc.isAuthenticated {
		t.Error("New service should not be authenticated")
	}
	if svc.accessTokens == nil {
		t.Error("Access tokens map should be initialized")
	}
}

func TestServiceAuthentication(t *testing.T) {
	svc := NewService()

	// Test initial state
	if got := svc.GetStatus(); got != "Not Authenticated" {
		t.Errorf("Initial status = %q, want 'Not Authenticated'", got)
	}

	// Test successful authentication
	if err := svc.Authenticate(); err != nil {
		t.Errorf("Authenticate() failed: %v", err)
	}
	if got := svc.GetStatus(); got != "Authenticated" {
		t.Errorf("Status after auth = %q, want 'Authenticated'", got)
	}

	// Test double authentication
	if err := svc.Authenticate(); err == nil {
		t.Error("Second Authenticate() should return error")
	}
}

func TestVerifyAppAPIKey(t *testing.T) {
	tests := []struct {
		name       string
		apiKey     string
		envKeys    string
		disabled   string
		expected   bool
		setupEnv   func()
		cleanupEnv func()
	}{
		{
			name:     "valid key",
			apiKey:   "test-key",
			envKeys:  "test-key",
			expected: true,
			setupEnv: func() {
				os.Setenv("VALID_API_KEYS", "test-key")
				os.Unsetenv("DISABLE_AUTH")
			},
			cleanupEnv: func() {
				os.Unsetenv("VALID_API_KEYS")
			},
		},
		{
			name:     "invalid key",
			apiKey:   "invalid-key",
			envKeys:  "test-key",
			expected: false,
			setupEnv: func() {
				os.Setenv("VALID_API_KEYS", "test-key")
				os.Unsetenv("DISABLE_AUTH")
			},
			cleanupEnv: func() {
				os.Unsetenv("VALID_API_KEYS")
			},
		},
		{
			name:     "disabled auth",
			apiKey:   "any-key",
			disabled: "true",
			expected: true,
			setupEnv: func() {
				os.Setenv("DISABLE_AUTH", "true")
				os.Unsetenv("VALID_API_KEYS")
			},
			cleanupEnv: func() {
				os.Unsetenv("DISABLE_AUTH")
			},
		},
		{
			name:     "multiple valid keys",
			apiKey:   "key2",
			envKeys:  "key1,key2,key3",
			expected: true,
			setupEnv: func() {
				os.Setenv("VALID_API_KEYS", "key1,key2,key3")
				os.Unsetenv("DISABLE_AUTH")
			},
			cleanupEnv: func() {
				os.Unsetenv("VALID_API_KEYS")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupEnv()
			defer tt.cleanupEnv()

			if got := VerifyAppAPIKey(tt.apiKey); got != tt.expected {
				t.Errorf("VerifyAppAPIKey(%q) = %v, want %v", tt.apiKey, got, tt.expected)
			}
		})
	}
}

func TestVerifyCopilotAPIKey(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name     string
		token    string
		expected bool
	}{
		{
			name:     "valid token",
			token:    "tid=test;" + "exp=" + strconv.FormatInt(now.Add(time.Hour).Unix(), 10) + ";sku=pro",
			expected: true,
		},
		{
			name:     "expired token",
			token:    "tid=test;" + "exp=" + strconv.FormatInt(now.Add(-time.Hour).Unix(), 10) + ";sku=pro",
			expected: false,
		},
		{
			name:     "malformed token",
			token:    "invalid-token",
			expected: false,
		},
		{
			name:     "missing expiration",
			token:    "tid=test;sku=pro",
			expected: false,
		},
		{
			name:     "invalid expiration",
			token:    "tid=test;exp=invalid;sku=pro",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := VerifyCopilotAPIKey(tt.token); got != tt.expected {
				t.Errorf("VerifyCopilotAPIKey(%q) = %v, want %v", tt.token, got, tt.expected)
			}
		})
	}
}

func TestAccessTokens(t *testing.T) {
	svc := NewService()
	userID := uint64(123)

	// Generate token
	token, err := svc.GenerateAccessToken(userID)
	if err != nil {
		t.Fatalf("GenerateAccessToken() error = %v", err)
	}
	if token == "" {
		t.Error("GenerateAccessToken() returned empty token")
	}

	// Verify token
	if !svc.VerifyAccessToken(token, userID) {
		t.Error("VerifyAccessToken() failed for valid token")
	}

	// Verify with wrong userID
	if svc.VerifyAccessToken(token, userID+1) {
		t.Error("VerifyAccessToken() succeeded with wrong userID")
	}

	// Verify with wrong token
	if svc.VerifyAccessToken("wrong-token", userID) {
		t.Error("VerifyAccessToken() succeeded with wrong token")
	}
}

func TestKeypairGeneration(t *testing.T) {
	pub, priv, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}
	if pub == nil || pub.Key == nil {
		t.Error("GenerateKeypair() returned nil public key")
	}
	if priv == nil || priv.Key == nil {
		t.Error("GenerateKeypair() returned nil private key")
	}
}

func TestPublicKeyOperations(t *testing.T) {
	// Generate a test keypair
	pub, _, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}

	// Test PEM encoding/decoding
	pemKey := `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA1234567890
-----END PUBLIC KEY-----`

	pubKey := &PublicKey{}
	if err := pubKey.TryFrom(pemKey); err == nil {
		// We expect an error since this is not a valid PEM key
		t.Error("TryFrom() should fail with invalid PEM")
	}

	// Test encryption
	testStr := "test message"
	_, err = pub.EncryptString(testStr, EncryptionFormatV1)
	if err != nil {
		t.Errorf("EncryptString() error = %v", err)
	}
}

func TestRandomToken(t *testing.T) {
	token1 := RandomToken()
	token2 := RandomToken()

	if token1 == "" {
		t.Error("RandomToken() returned empty string")
	}
	if token1 == token2 {
		t.Error("RandomToken() returned same token twice")
	}
}

func TestHashAccessToken(t *testing.T) {
	token := "test-token"
	hash := HashAccessToken(token)

	if hash == "" {
		t.Error("HashAccessToken() returned empty string")
	}
	if !strings.HasPrefix(hash, "$sha256$") {
		t.Error("HashAccessToken() hash doesn't have correct prefix")
	}

	// Same input should produce same hash
	hash2 := HashAccessToken(token)
	if hash != hash2 {
		t.Error("HashAccessToken() not deterministic")
	}

	// Different input should produce different hash
	hash3 := HashAccessToken("different-token")
	if hash == hash3 {
		t.Error("HashAccessToken() produced same hash for different input")
	}
}
