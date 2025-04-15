// Package utils provides utility functions for API interactions, file operations,
// configuration reading, and other helper functionality.
package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// CopilotConfig represents the structure of the GitHub Copilot config file.
// This matches the structure of the apps.json file used by official GitHub Copilot clients.
type CopilotConfig struct {
	// Tokens maps provider IDs to token information
	Tokens map[string]TokenInfo `json:"tokens"`
}

// TokenInfo represents token information in the GitHub Copilot config file.
// Each token contains authentication information and expiration details.
type TokenInfo struct {
	// Token is the actual bearer token for API authentication
	Token string `json:"token"`
	// ExpiresAt is the Unix timestamp when the token expires
	ExpiresAt int64 `json:"expires_at"`
	// ExpiresIn is the number of seconds until token expiration at the time of creation
	ExpiresIn int `json:"expires_in"`
	// ProviderID identifies the authentication provider
	ProviderID string `json:"provider_id"`
}

// GetCopilotToken retrieves the GitHub Copilot access token from the local config file.
// This allows the application to use the same authentication as the official GitHub Copilot client.
//
// The function looks for a config file at the standard location for the current platform:
// - Windows: %APPDATA%\GitHub Copilot\apps.json
// - macOS: ~/.config/github-copilot/apps.json
// - Linux: ~/.config/github-copilot/apps.json
//
// The token retrieved is in the format:
// tid=<token-id>;exp=<expiration-timestamp>;sku=<subscription-type>;proxy-ep=<proxy-endpoint>;st=<status>;
// followed by various feature flags (chat=1;cit=1;etc.)
//
// Returns the token string or an error if the token couldn't be retrieved.
//
// Example:
//
//	token, err := GetCopilotToken()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	// Use token for API authentication
func GetCopilotToken() (string, error) {
	configPath, err := getCopilotConfigPath()
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return "", err
	}

	var config CopilotConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return "", err
	}

	// Find any valid token (typically there's only one)
	for _, tokenInfo := range config.Tokens {
		if tokenInfo.Token != "" {
			return tokenInfo.Token, nil
		}
	}

	return "", errors.New("no valid GitHub Copilot token found in config")
}

// getCopilotConfigPath returns the path to the GitHub Copilot config file based on the OS.
// Internal helper function that determines the correct path for the current platform.
func getCopilotConfigPath() (string, error) {
	var configDir string

	// Determine the config directory based on the operating system
	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			return "", errors.New("APPDATA environment variable not set")
		}
		configDir = filepath.Join(appData, "GitHub Copilot")
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		configDir = filepath.Join(home, ".config", "github-copilot")
	default: // Linux and other Unix-like systems
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		configDir = filepath.Join(home, ".config", "github-copilot")
	}

	return filepath.Join(configDir, "apps.json"), nil
}

// GetCopilotOAuthToken attempts to read a GitHub OAuth token from various sources.
// It checks environment variables (COPILOT_OAUTH_TOKEN or OAUTH_TOKEN) and will
// eventually try to read from the VS Code GitHub Copilot extension configuration.
//
// Returns the OAuth token if found, or an empty string and error if not found.
func GetCopilotOAuthToken() (string, error) {
	// First check environment variables
	oauthToken := os.Getenv("COPILOT_OAUTH_TOKEN")
	if oauthToken != "" {
		// Clean the token if it has quotes (which might happen in .env files)
		oauthToken = strings.Trim(oauthToken, "'\"")
		fmt.Printf("Found COPILOT_OAUTH_TOKEN in environment variables: %s\n", maskToken(oauthToken))
		return oauthToken, nil
	}

	oauthToken = os.Getenv("OAUTH_TOKEN")
	if oauthToken != "" {
		// Clean the token if it has quotes (which might happen in .env files)
		oauthToken = strings.Trim(oauthToken, "'\"")
		fmt.Printf("Found OAUTH_TOKEN in environment variables: %s\n", maskToken(oauthToken))
		return oauthToken, nil
	}

	// TODO: Add logic to extract OAuth token from VS Code Copilot extension
	// This would require reading the configuration files

	return "", errors.New("no OAuth token found in environment variables")
}

// maskToken masks most of a token for safe logging, showing only the first 4 and last 4 characters
func maskToken(token string) string {
	if len(token) <= 8 {
		return "****"
	}
	return token[:4] + "..." + token[len(token)-4:]
}

// MaskToken masks a token for display by showing only the first and last few characters
// This is used for debugging purposes to show token format without revealing the entire token
func MaskToken(token string) string {
	if len(token) < 10 {
		return "***" // Too short to safely show anything
	}

	// For tokens with "tid=" prefix, keep that visible
	if len(token) >= 4 && token[:4] == "tid=" {
		// Show tid= prefix, first few chars of the ID, and last few chars
		parts := strings.Split(token, ";")
		if len(parts) > 0 {
			tidPart := parts[0]
			if len(tidPart) > 12 {
				return tidPart[:8] + "..." + tidPart[len(tidPart)-4:] + ";***"
			}
		}
	}

	// For standard tokens, show first/last few chars
	return token[:4] + "..." + token[len(token)-4:]
}

// ValidateCopilotToken checks if a Copilot token is valid.
// It parses the token format and checks if it's expired.
//
// Parameters:
//   - token: The Copilot token string
//
// Returns true if the token is valid and not expired, false otherwise.
func ValidateCopilotToken(token string) bool {
	// Check basic format: should start with "tid="
	if !strings.HasPrefix(token, "tid=") {
		return false
	}

	// Extract expiration time from the token
	// The token has format: tid=<token-id>;exp=<expiration-timestamp>;sku=<subscription-type>;...
	expIndex := strings.Index(token, ";exp=")
	if expIndex == -1 {
		return false
	}

	// Find the end of the expiration timestamp
	expStart := expIndex + 5 // length of ";exp="
	expEnd := strings.Index(token[expStart:], ";")
	if expEnd == -1 {
		return false
	}
	expEnd += expStart

	// Parse the expiration timestamp
	expStr := token[expStart:expEnd]
	expTimestamp, err := parseInt64(expStr)
	if err != nil {
		return false
	}

	// Check if token is expired
	currentTime := time.Now().Unix()
	if currentTime > expTimestamp {
		return false
	}

	return true
}

// ParseCopilotToken parses a Copilot token into its components
func ParseCopilotToken(token string) (map[string]string, error) {
	result := make(map[string]string)

	// Split by semicolons
	parts := strings.Split(token, ";")
	for _, part := range parts {
		// Skip empty parts
		if part == "" {
			continue
		}

		// Split by equals sign
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("invalid token part: %s", part)
		}

		// Add to result map
		result[kv[0]] = kv[1]
	}

	// Validate required parts
	if _, ok := result["tid"]; !ok {
		return nil, fmt.Errorf("missing tid in token")
	}

	return result, nil
}

// parseInt64 converts a string to int64, with proper error handling
func parseInt64(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}
