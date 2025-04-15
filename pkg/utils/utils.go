package utils

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"
)

// CopilotChatCompletionURL is the endpoint for GitHub Copilot chat completions.
const CopilotChatCompletionURL = "https://api.githubcopilot.com/chat/completions"

// SomeUtilityFunction performs a specific utility task.
// It simply wraps the input string with a "Processed:" prefix.
// This is a placeholder function for demonstration purposes.
//
// Parameters:
//   - input: The string to process
//
// Returns the processed string.
func SomeUtilityFunction(input string) string {
	// Implement the utility logic here
	return "Processed: " + input
}

// GetEnvWithDefault retrieves an environment variable or returns a default value if not set.
//
// Parameters:
//   - name: The name of the environment variable
//   - defaultValue: The default value to return if the environment variable is not set
//
// Returns the value of the environment variable, or the default value if not set.
func GetEnvWithDefault(name, defaultValue string) string {
	value := os.Getenv(name)
	if value == "" {
		return defaultValue
	}
	return value
}

// CallOpenAIEndpoint sends a request to the OpenAI endpoint and returns the response.
// This function uses the GitHub Copilot endpoint but formats the request and response
// in a way that's compatible with OpenAI's API structure.
//
// Parameters:
//   - apiKey: The API key to use for authentication
//   - payload: The request payload (must include "model" and "messages" fields)
//
// Returns a map containing the response data or an error if the request failed.
//
// Example:
//
//	payload := map[string]interface{}{
//	    "model": "copilot-chat",
//	    "messages": []map[string]interface{}{
//	        {"role": "user", "content": "Hello, how are you?"},
//	    },
//	}
//	response, err := CallOpenAIEndpoint(apiKey, payload)
func CallOpenAIEndpoint(apiKey string, payload map[string]interface{}) (map[string]interface{}, error) {
	// Extract provider_request if it exists
	providerRequest, hasProviderRequest := payload["provider_request"].(map[string]interface{})
	if hasProviderRequest {
		// Use the inner provider_request
		if _, ok := providerRequest["model"]; !ok {
			return nil, errors.New("provider_request must include 'model'")
		}
		if _, ok := providerRequest["messages"]; !ok {
			return nil, errors.New("provider_request must include 'messages'")
		}

		// Use the provider_request for the actual API call
		payload = providerRequest
	} else {
		// Ensure payload adheres to OpenAI schema
		if _, ok := payload["model"]; !ok {
			return nil, errors.New("payload must include 'model'")
		}
		if _, ok := payload["messages"]; !ok {
			return nil, errors.New("payload must include 'messages'")
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	// Get environment variables for headers or use defaults
	editorVersion := GetEnvWithDefault("EDITOR_VERSION", "vscode/1.99.2")
	editorPluginVersion := GetEnvWithDefault("EDITOR_PLUGIN_VERSION", "copilot-chat/0.26.3")
	vscodeMachineID := os.Getenv("VSCODE_MACHINE_ID")
	vscodeSessionID := os.Getenv("VSCODE_SESSION_ID")

	// Generate a unique request ID
	requestID := fmt.Sprintf("%s-%s", time.Now().Format("20060102T150405.000Z"), uuid.New().String()[:8])

	// Create request with all required headers
	req, err := http.NewRequest("POST", CopilotChatCompletionURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	// Set Authorization header
	if strings.HasPrefix(apiKey, "tid=") {
		// This is already a full GitHub Copilot token, use it directly
		req.Header.Set("Authorization", "Bearer "+apiKey)
	} else {
		// For other API keys that might not have the Bearer prefix
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	// Required Copilot headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Editor-Version", editorVersion)
	req.Header.Set("Editor-Plugin-Version", editorPluginVersion)
	req.Header.Set("Copilot-Integration-ID", "vscode-chat")
	req.Header.Set("User-Agent", "GitHubCopilotChat/"+strings.TrimPrefix(editorPluginVersion, "copilot-chat/"))
	req.Header.Set("OpenAI-Intent", "conversation-agent")
	req.Header.Set("X-GitHub-API-Version", "2025-04-01")
	req.Header.Set("X-Initiator", "user")
	req.Header.Set("X-Interaction-Type", "conversation-agent")
	req.Header.Set("X-Request-ID", requestID)

	// Optional VS Code specific headers if available
	if vscodeMachineID != "" {
		req.Header.Set("Vscode-Machineid", vscodeMachineID)
	}
	if vscodeSessionID != "" {
		req.Header.Set("Vscode-Sessionid", vscodeSessionID)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to call OpenAI endpoint: %s - %s", resp.Status, string(bodyBytes))
	}

	var response struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
			Index        int    `json:"index"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	// Convert response to a generic map for flexibility
	responseMap := map[string]interface{}{
		"id":      response.ID,
		"object":  response.Object,
		"created": response.Created,
		"model":   response.Model,
		"choices": response.Choices,
		"usage":   response.Usage,
	}

	return responseMap, nil
}

// CallCopilotEndpoint sends a request to the GitHub Copilot endpoint using the locally stored token.
// This is a convenience wrapper around CallOpenAIEndpoint that automatically fetches and uses
// the local Copilot token.
//
// Parameters:
//   - payload: The request payload (must include "model" and "messages" fields)
//
// Returns a map containing the response data or an error if the request failed.
func CallCopilotEndpoint(payload map[string]interface{}) (map[string]interface{}, error) {
	apiKey, err := GetCopilotToken()
	if err != nil {
		return nil, errors.New("failed to get Copilot token: " + err.Error())
	}

	return CallOpenAIEndpoint(apiKey, payload)
}

// CallAPIWithBody makes an API call with a JSON body and returns the raw response.
// This is a lower-level function that gives more control over the request and response.
//
// Parameters:
//   - url: The API endpoint URL
//   - contentType: The content type header value (e.g., "application/json")
//   - apiKey: The API key to use for authentication
//   - payload: The request payload (will be JSON-serialized)
//   - headers: Optional additional headers to include in the request
//
// Returns the HTTP response or an error if the request failed.
func CallAPIWithBody(url string, contentType string, apiKey string, payload interface{}, headers ...map[string]string) (*http.Response, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	// Set standard headers
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", contentType)

	// Add additional headers if provided
	if len(headers) > 0 {
		for headerKey, headerValue := range headers[0] {
			req.Header.Set(headerKey, headerValue)
		}
	}

	client := &http.Client{}
	return client.Do(req)
}

// DynamicImport dynamically imports a package using reflection.
// This is useful for avoiding import cycles in the codebase.
//
// Parameters:
//   - pkgPath: The import path of the package to load
//
// Returns a Package object that provides access to the package's exported symbols,
// or an error if the package could not be loaded.
func DynamicImport(pkgPath string) (*Package, error) {
	// This implementation would normally use Go's reflect package
	// to dynamically import packages. For simplicity, we're using a stub
	// that returns a mock Package object for the app package.

	// In a real implementation, this would use reflect or plugin to dynamically
	// load the package.
	if pkgPath == "copilot-proxy/internal/app" {
		return &Package{
			path: pkgPath,
			// This would be populated with actual exported symbols
		}, nil
	}

	return nil, fmt.Errorf("package %s not found or not supported", pkgPath)
}

// Package represents a dynamically loaded Go package.
type Package struct {
	path    string
	symbols map[string]interface{}
}

// Lookup finds an exported symbol in the package by name.
// Returns nil if the symbol is not found.
func (p *Package) Lookup(name string) *Symbol {
	// In a real implementation, this would use reflection to look up the symbol
	// This is a simplified version for demonstration purposes
	if name == "NewApp" {
		return &Symbol{
			name: name,
			pkg:  p,
		}
	}
	return nil
}

// Symbol represents an exported symbol from a dynamically loaded package.
type Symbol struct {
	name string
	pkg  *Package
}

// Call invokes the symbol as a function with the given arguments.
// Returns the results of the function call.
func (s *Symbol) Call(args []interface{}) []reflect.Value {
	// In a real implementation, this would use reflection to call the function
	// This is a simplified version for demonstration purposes
	if s.name == "NewApp" {
		// Create a mock App object
		app := &mockApp{}
		return []reflect.Value{reflect.ValueOf(app)}
	}
	return nil
}

// mockApp is a mock implementation of the app.App type used for testing.
type mockApp struct{}

// GetCopilotAPIKey is a mock implementation of app.App.GetCopilotAPIKey.
func (a *mockApp) GetCopilotAPIKey() (string, error) {
	// Try to get OAuth token from environment variables
	oauthToken := os.Getenv("COPILOT_OAUTH_TOKEN")
	if oauthToken == "" {
		oauthToken = os.Getenv("OAUTH_TOKEN")
	}

	if oauthToken != "" {
		// This would normally call out to the GitHub API to get a token
		// For demonstration, we'll use a mock implementation
		return "tid=mock_token_from_oauth;exp=" + fmt.Sprintf("%d", time.Now().Add(24*time.Hour).Unix()) + ";sku=free", nil
	}

	return "", fmt.Errorf("no OAuth token found in environment")
}

// GetMethod gets a method from an object by name using reflection.
// Returns a method value that can be called, or nil if the method doesn't exist.
func GetMethod(obj interface{}, methodName string) *reflect.Value {
	// In a real implementation, this would use reflection to get the method
	// This is a simplified version for demonstration purposes
	if methodName == "GetCopilotAPIKey" {
		if app, ok := obj.(*mockApp); ok {
			method := reflect.ValueOf(app).MethodByName(methodName)
			return &method
		}
	}
	return nil
}
