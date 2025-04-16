package llm

import (
	"copilot-proxy/pkg/models"
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

// TokenClaims struct for JWT token claims (simplified for personal use)
type TokenClaims struct {
	jwt.RegisteredClaims
	UserID          uint64 `json:"user_id"`
	GithubUserLogin string `json:"github_user_login"`
}

// CreateLLMToken generates a JWT token for LLM API access
func CreateLLMToken(userID uint64, githubLogin string, secret string) (string, error) {
	now := time.Now()

	claims := TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(TokenLifetime * time.Second)),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        now.Format(time.RFC3339Nano), // Simple ID based on timestamp
		},
		UserID:          userID,
		GithubUserLogin: githubLogin,
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

	// Create a simplified token with default values for unused fields
	return &models.LLMToken{
		Iat:                    claims.IssuedAt.Unix(),
		Exp:                    claims.ExpiresAt.Unix(),
		Jti:                    claims.ID,
		UserID:                 claims.UserID,
		GithubUserLogin:        claims.GithubUserLogin,
		AccountCreatedAt:       time.Now().AddDate(-1, 0, 0), // Default to 1 year ago
		IsStaff:                true,                         // Default to true for personal use
		HasLLMSubscription:     true,                         // Default to true for personal use
		MaxMonthlySpendInCents: 10000,                        // Default high limit for personal use
	}, nil
}
