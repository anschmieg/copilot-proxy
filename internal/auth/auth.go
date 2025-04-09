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

// VerifyAPIKey checks the provided API key against the comma-separated list in the VALID_API_KEYS environment variable.
func VerifyAPIKey(apiKey string) bool {
	validKeys := os.Getenv("VALID_API_KEYS")
	keys := strings.Split(validKeys, ",")
	for _, key := range keys {
		if apiKey == strings.TrimSpace(key) {
			return true
		}
	}
	return false
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
