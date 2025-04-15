package main

import (
	"copilot-proxy/internal/llm"
	"copilot-proxy/pkg/utils"
	"fmt"
	"strings"
)

// MaskToken replaces most of a token with asterisks for secure display
// It preserves the first and last 5 characters to help with debugging
func MaskToken(token string) string {
	if token == "" {
		return "[empty token]"
	}

	if len(token) <= 10 {
		return "***" + token[len(token)-3:]
	}

	// For tokens that start with tid=, preserve the format
	if strings.HasPrefix(token, "tid=") {
		parts := strings.Split(token, ";")
		maskedParts := make([]string, len(parts))

		for i, part := range parts {
			if strings.HasPrefix(part, "tid=") {
				// Mask the token ID but keep the prefix
				tidValue := strings.TrimPrefix(part, "tid=")
				if len(tidValue) <= 8 {
					maskedParts[i] = "tid=" + strings.Repeat("*", len(tidValue))
				} else {
					maskedParts[i] = "tid=" + tidValue[:4] + "..." + tidValue[len(tidValue)-4:]
				}
			} else if strings.HasPrefix(part, "exp=") {
				// Keep expiration timestamp visible for debugging
				maskedParts[i] = part
			} else {
				// Mask other parts
				keyValue := strings.SplitN(part, "=", 2)
				if len(keyValue) == 2 {
					maskedParts[i] = keyValue[0] + "=***"
				} else {
					maskedParts[i] = "***"
				}
			}
		}

		return strings.Join(maskedParts, ";")
	}

	// For normal tokens, just show beginning and end
	return token[:5] + "..." + token[len(token)-5:]
}

// AnalyzeToken checks if a token is in the correct format for the GitHub Copilot API
func AnalyzeToken(token string) string {
	if token == "" {
		return "ERROR: Token is empty"
	}

	result := fmt.Sprintf("Token length: %d\n", len(token))

	if strings.HasPrefix(token, "Bearer ") {
		result += "WARNING: Token starts with 'Bearer ' prefix, which should be added by the code\n"
		token = strings.TrimPrefix(token, "Bearer ")
	}

	if strings.HasPrefix(token, "tid=") {
		result += "✓ Token has correct 'tid=' prefix format\n"

		parts := strings.Split(token, ";")
		result += fmt.Sprintf("✓ Token has %d parts\n", len(parts))

		hasExp := false
		for _, part := range parts {
			if strings.HasPrefix(part, "exp=") {
				hasExp = true
				break
			}
		}

		if hasExp {
			result += "✓ Token has 'exp=' timestamp\n"
		} else {
			result += "WARNING: Token is missing 'exp=' timestamp\n"
		}
	} else {
		result += "WARNING: Token does not have 'tid=' prefix, may not be in the correct GitHub Copilot format\n"
	}

	return result
}

// DisplayTokenAnalysis provides detailed analysis of the token format
func DisplayTokenAnalysis() {
	// Get the token from the standard sources
	token, err := utils.GetCopilotToken()
	if err != nil {
		fmt.Printf("Error retrieving Copilot token: %v\n", err)
		return
	}

	// Print token information
	fmt.Println("🔍 Token Analysis")
	fmt.Println("----------------------------")

	// Print the token with masking for security
	fmt.Printf("Token format: %s\n", MaskToken(token))

	// Check if token starts with "tid="
	if len(token) >= 4 {
		fmt.Printf("Starts with 'tid=': %v\n", token[:4] == "tid=")
	} else {
		fmt.Println("Token is too short to check for 'tid=' prefix")
	}

	// Parse the token to get more details
	tokenParts, err := utils.ParseCopilotToken(token)
	if err != nil {
		fmt.Printf("Error parsing token: %v\n", err)
	} else {
		fmt.Println("\nToken Components:")
		fmt.Printf("- Token ID: %s\n", tokenParts["tid"])
		fmt.Printf("- Expiration: %s\n", tokenParts["exp"])
		if sku, ok := tokenParts["sku"]; ok {
			fmt.Printf("- Subscription: %s\n", sku)
		}

		// Print all remaining components
		fmt.Println("\nAll Token Parts:")
		for k, v := range tokenParts {
			if k != "tid" && k != "exp" && k != "sku" {
				fmt.Printf("- %s: %s\n", k, v)
			}
		}
	}

	// Also check config to see how it's loaded
	fmt.Println("\nConfig Analysis:")
	service := llm.NewService()
	config := service.GetConfig()

	// Print the config's token with masking
	fmt.Printf("Config token format: %s\n", MaskToken(config.CopilotAPIKey))
	fmt.Printf("Config token starts with 'tid=': %v\n", len(config.CopilotAPIKey) >= 4 && config.CopilotAPIKey[:4] == "tid=")

	fmt.Println("----------------------------")
	fmt.Println("Based on GitHub Copilot API documentation, the authorization should be the raw token from Copilot, NOT prefixed with 'Bearer '")
}
