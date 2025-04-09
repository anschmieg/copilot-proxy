// Package utils provides utility functions for API interactions, file operations,
// configuration reading, and other helper functionality.
package utils

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
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
// - macOS/Linux: ~/.config/github-copilot/apps.json
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
