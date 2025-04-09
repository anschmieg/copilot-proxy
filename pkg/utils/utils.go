package utils

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
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
	// Ensure payload adheres to OpenAI schema
	if _, ok := payload["model"]; !ok {
		return nil, errors.New("payload must include 'model'")
	}
	if _, ok := payload["messages"]; !ok {
		return nil, errors.New("payload must include 'messages'")
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", CopilotChatCompletionURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("failed to call OpenAI endpoint: " + resp.Status)
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
//
// Returns the HTTP response or an error if the request failed.
func CallAPIWithBody(url string, contentType string, apiKey string, payload interface{}) (*http.Response, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", contentType)

	client := &http.Client{}
	return client.Do(req)
}
