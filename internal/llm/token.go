package llm

import (
	"copilot-proxy/pkg/models"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

const (
	// TokenLifetime defines how long tokens are valid
	TokenLifetime = 60 * 60 // 1 hour in seconds
)

var (
	// ErrTokenExpired is returned when the token has expired
	ErrTokenExpired = errors.New("token expired")

	// ErrInvalidToken is returned when the token is invalid for any reason
	ErrInvalidToken = errors.New("invalid token")
)

// TokenClaims struct for JWT token claims
type TokenClaims struct {
	jwt.RegisteredClaims
	UserID                        uint64  `json:"user_id"`
	SystemID                      *string `json:"system_id,omitempty"`
	MetricsID                     string  `json:"metrics_id"`
	GithubUserLogin               string  `json:"github_user_login"`
	AccountCreatedAt              int64   `json:"account_created_at"`
	IsStaff                       bool    `json:"is_staff"`
	HasLLMSubscription            bool    `json:"has_llm_subscription"`
	MaxMonthlySpendInCents        uint32  `json:"max_monthly_spend_in_cents"`
	CustomMonthlyAllowanceInCents *uint32 `json:"custom_llm_monthly_allowance_in_cents,omitempty"`
}

// CreateLLMToken generates a JWT token for LLM API access
func CreateLLMToken(userID uint64, metricsID string, githubLogin string,
	accountCreatedAt time.Time, isStaff bool, hasSubscription bool,
	maxMonthlySpend uint32, customAllowance *uint32, secret string) (string, error) {

	now := time.Now()

	claims := TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(TokenLifetime * time.Second)),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        NewUUID(), // Implement this function to generate a UUID
		},
		UserID:                        userID,
		MetricsID:                     metricsID,
		GithubUserLogin:               githubLogin,
		AccountCreatedAt:              accountCreatedAt.Unix(),
		IsStaff:                       isStaff,
		HasLLMSubscription:            hasSubscription,
		MaxMonthlySpendInCents:        maxMonthlySpend,
		CustomMonthlyAllowanceInCents: customAllowance,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString([]byte(secret))
}

// ValidateLLMToken validates and parses a JWT token
func ValidateLLMToken(tokenString string, secret string) (*models.LLMToken, error) {
	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*TokenClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return &models.LLMToken{
		Iat:                           claims.IssuedAt.Unix(),
		Exp:                           claims.ExpiresAt.Unix(),
		Jti:                           claims.ID,
		UserID:                        claims.UserID,
		MetricsID:                     claims.MetricsID,
		GithubUserLogin:               claims.GithubUserLogin,
		AccountCreatedAt:              time.Unix(claims.AccountCreatedAt, 0),
		IsStaff:                       claims.IsStaff,
		HasLLMSubscription:            claims.HasLLMSubscription,
		MaxMonthlySpendInCents:        claims.MaxMonthlySpendInCents,
		CustomMonthlyAllowanceInCents: claims.CustomMonthlyAllowanceInCents,
	}, nil
}

// EncryptionFormat represents the format used for token encryption
type EncryptionFormat int

const (
	// EncryptionFormatV0 is the legacy encryption format
	EncryptionFormatV0 EncryptionFormat = iota
	// EncryptionFormatV1 uses OAEP with SHA-256
	EncryptionFormatV1
)

// NewUUID generates a new UUID for JWT IDs
func NewUUID() string {
	// This is a simplified version. In a real implementation, use a proper UUID package
	return base64.StdEncoding.EncodeToString([]byte(time.Now().String()))
}

// PublicKeyFromPEM creates a public key from PEM data
func PublicKeyFromPEM(pemStr string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("failed to decode PEM block")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("not an RSA public key")
	}

	return rsaPub, nil
}
