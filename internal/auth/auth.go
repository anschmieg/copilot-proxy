package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Service provides authentication-related functionalities.
type Service struct {
	isAuthenticated bool
	accessTokens    map[string]AccessToken
	mutex           sync.RWMutex
}

// AccessToken represents an authenticated token
type AccessToken struct {
	ID        string
	UserID    uint64
	Hash      string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// EncryptionFormat represents the format used for encryption
type EncryptionFormat int

const (
	// EncryptionFormatV0 is the legacy encryption format
	EncryptionFormatV0 EncryptionFormat = iota
	// EncryptionFormatV1 uses OAEP with SHA-256
	EncryptionFormatV1
)

// NewService creates and returns a new instance of the Service struct.
func NewService() *Service {
	return &Service{
		isAuthenticated: false,
		accessTokens:    make(map[string]AccessToken),
	}
}

// GetStatus returns the authentication status of the service.
func (s *Service) GetStatus() string {
	if s.isAuthenticated {
		return "Authenticated"
	}
	return "Not Authenticated"
}

// Authenticate sets the service's authentication status to true if not already authenticated.
func (s *Service) Authenticate() error {
	if s.isAuthenticated {
		return errors.New("Already authenticated")
	}
	s.isAuthenticated = true
	return nil
}

// VerifyAppAPIKey checks if the provided API key is valid for accessing this app's API.
// This function verifies keys against the VALID_API_KEYS environment variable, which
// should contain a comma-separated list of valid API keys.
//
// If the DISABLE_AUTH environment variable is set to "true" or "1", all authentication
// checks will be bypassed and any API key will be considered valid.
//
// This function is used to authenticate API requests to the proxy application itself,
// not for authenticating with external services like GitHub Copilot.
//
// Parameters:
//   - apiKey: The API key to validate
//
// Returns:
//   - bool: true if the API key is valid or if authentication is disabled, false otherwise
func VerifyAppAPIKey(apiKey string) bool {
	// Check if authorization is disabled globally
	if disableAuth := os.Getenv("DISABLE_AUTH"); disableAuth == "true" || disableAuth == "1" {
		fmt.Println("Authorization is disabled, accepting all API keys")
		return true
	}

	// Check environment variables
	validKeys := os.Getenv("VALID_API_KEYS")
	if validKeys == "" {
		fmt.Println("No valid API keys configured in environment")
		return false
	}

	// Debug output to help diagnose issues
	fmt.Printf("Validating API key: %s against environment keys\n", apiKey)

	keys := strings.Split(validKeys, ",")
	for _, key := range keys {
		trimmedKey := strings.TrimSpace(key)
		if apiKey == trimmedKey {
			return true
		}
	}

	return false
}

// VerifyCopilotAPIKey checks if the provided API key is a valid GitHub Copilot token.
// This function validates tokens in the format "tid=token_id;exp=expiration;..."
// by checking the token's format and expiration timestamp.
//
// The GitHub Copilot token format includes several components:
//   - tid: Token ID
//   - exp: Expiration timestamp (Unix format)
//   - sku: Subscription type
//   - Various feature flags and configuration parameters
//
// Parameters:
//   - apiKey: The GitHub Copilot token to validate
//
// Returns:
//   - bool: true if the token is valid and not expired, false otherwise
func VerifyCopilotAPIKey(apiKey string) bool {
	// Check if it's a GitHub Copilot token
	if strings.HasPrefix(apiKey, "tid=") {
		// Extract the token ID and expiration
		parts := strings.Split(apiKey, ";")
		if len(parts) >= 2 {
			expPart := parts[1]

			// Check if expiration part exists and is properly formatted
			if strings.HasPrefix(expPart, "exp=") {
				expStr := strings.TrimPrefix(expPart, "exp=")
				expInt, err := strconv.ParseInt(expStr, 10, 64)
				if err == nil {
					expTime := time.Unix(expInt, 0)
					if expTime.After(time.Now()) {
						return true
					}
				}
			}
		}
	}

	return false
}

// VerifyAPIKey checks the provided API key for compatibility with either this app's API
// or the GitHub Copilot API. This is maintained for backward compatibility.
func VerifyAPIKey(apiKey string) bool {
	return VerifyAppAPIKey(apiKey) || VerifyCopilotAPIKey(apiKey)
}

// GenerateAccessToken creates a new access token for a user
func (s *Service) GenerateAccessToken(userID uint64) (string, error) {
	token := RandomToken()
	tokenHash := HashAccessToken(token)

	id := fmt.Sprintf("tok_%s", RandomToken()[:10])

	s.mutex.Lock()
	s.accessTokens[id] = AccessToken{
		ID:        id,
		UserID:    userID,
		Hash:      tokenHash,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour), // 30 days
	}
	s.mutex.Unlock()

	return token, nil
}

// VerifyAccessToken checks if an access token is valid
func (s *Service) VerifyAccessToken(token string, userID uint64) bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	tokenHash := HashAccessToken(token)

	for _, storedToken := range s.accessTokens {
		if storedToken.UserID == userID && storedToken.Hash == tokenHash {
			if time.Now().After(storedToken.ExpiresAt) {
				return false // Token expired
			}
			return true
		}
	}

	return false
}

// RandomToken generates a random token for authentication
func RandomToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

// HashAccessToken hashes an access token using SHA-256
func HashAccessToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return "$sha256$" + base64.URLEncoding.EncodeToString(hash[:])
}

// PublicKey wraps an RSA public key
type PublicKey struct {
	Key *rsa.PublicKey
}

// PrivateKey wraps an RSA private key
type PrivateKey struct {
	Key *rsa.PrivateKey
}

// GenerateKeypair creates a new public/private key pair
func GenerateKeypair() (*PublicKey, *PrivateKey, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	pubKey := &PublicKey{Key: &privateKey.PublicKey}
	privKey := &PrivateKey{Key: privateKey}

	return pubKey, privKey, nil
}

// TryFrom creates a PublicKey from a PEM-encoded string
func (p *PublicKey) TryFrom(pemStr string) error {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return errors.New("failed to decode PEM block")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return err
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return errors.New("not an RSA public key")
	}

	p.Key = rsaPub
	return nil
}

// EncryptString encrypts a string using the public key
func (p *PublicKey) EncryptString(text string, format EncryptionFormat) (string, error) {
	var encryptedBytes []byte
	var err error

	switch format {
	case EncryptionFormatV0:
		encryptedBytes, err = rsa.EncryptPKCS1v15(rand.Reader, p.Key, []byte(text))
	case EncryptionFormatV1:
		encryptedBytes, err = rsa.EncryptOAEP(sha256.New(), rand.Reader, p.Key, []byte(text), nil)
	default:
		return "", errors.New("unsupported encryption format")
	}

	if err != nil {
		return "", err
	}

	return fmt.Sprintf("v%d:%s", format, base64.StdEncoding.EncodeToString(encryptedBytes)), nil
}
