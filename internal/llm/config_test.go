package llm

import (
	"os"
	"sync"
	"testing"
)

func TestGetConfig(t *testing.T) {
	// Save original env vars
	origKey := os.Getenv("COPILOT_API_KEY")
	origEditorVer := os.Getenv("EDITOR_VERSION")
	origPluginVer := os.Getenv("EDITOR_PLUGIN_VERSION")
	origMachineID := os.Getenv("VSCODE_MACHINE_ID")
	origSessionID := os.Getenv("VSCODE_SESSION_ID")

	defer func() {
		// Restore original env vars
		os.Setenv("COPILOT_API_KEY", origKey)
		os.Setenv("EDITOR_VERSION", origEditorVer)
		os.Setenv("EDITOR_PLUGIN_VERSION", origPluginVer)
		os.Setenv("VSCODE_MACHINE_ID", origMachineID)
		os.Setenv("VSCODE_SESSION_ID", origSessionID)
	}()

	tests := []struct {
		name              string
		envVars           map[string]string
		wantAPIKey        string
		wantEditorVersion string
		wantPluginVersion string
		wantVSCodeMachine string
		wantVSCodeSession string
	}{
		{
			name: "all environment variables set",
			envVars: map[string]string{
				"COPILOT_API_KEY":       "test-key",
				"EDITOR_VERSION":        "vscode/1.99.2",
				"EDITOR_PLUGIN_VERSION": "copilot-chat/0.26.3",
				"VSCODE_MACHINE_ID":     "test-machine",
				"VSCODE_SESSION_ID":     "test-session",
			},
			wantAPIKey:        "test-key",
			wantEditorVersion: "vscode/1.99.2",
			wantPluginVersion: "copilot-chat/0.26.3",
			wantVSCodeMachine: "test-machine",
			wantVSCodeSession: "test-session",
		},
		{
			name:    "no environment variables set",
			envVars: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all relevant env vars first
			os.Unsetenv("COPILOT_API_KEY")
			os.Unsetenv("EDITOR_VERSION")
			os.Unsetenv("EDITOR_PLUGIN_VERSION")
			os.Unsetenv("VSCODE_MACHINE_ID")
			os.Unsetenv("VSCODE_SESSION_ID")

			// Set test env vars
			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}

			// Reset the singleton for each test
			config = nil
			configOnce = sync.Once{}

			got := GetConfig()

			if got.CopilotAPIKey != tt.wantAPIKey {
				t.Errorf("GetConfig().CopilotAPIKey = %v, want %v", got.CopilotAPIKey, tt.wantAPIKey)
			}
			if got.EditorVersion != tt.wantEditorVersion {
				t.Errorf("GetConfig().EditorVersion = %v, want %v", got.EditorVersion, tt.wantEditorVersion)
			}
			if got.EditorPluginVersion != tt.wantPluginVersion {
				t.Errorf("GetConfig().EditorPluginVersion = %v, want %v", got.EditorPluginVersion, tt.wantPluginVersion)
			}
			if got.VSCodeMachineID != tt.wantVSCodeMachine {
				t.Errorf("GetConfig().VSCodeMachineID = %v, want %v", got.VSCodeMachineID, tt.wantVSCodeMachine)
			}
			if got.VSCodeSessionID != tt.wantVSCodeSession {
				t.Errorf("GetConfig().VSCodeSessionID = %v, want %v", got.VSCodeSessionID, tt.wantVSCodeSession)
			}
		})
	}
}

func TestDefaultModels(t *testing.T) {
	models := DefaultModels()

	if len(models) == 0 {
		t.Fatal("DefaultModels() returned empty slice")
	}

	// Test the copilot-chat model which should always be present
	var found bool
	for _, model := range models {
		if model.ID == "copilot-chat" {
			found = true
			if model.Provider != models.ProviderCopilot {
				t.Error("copilot-chat model has wrong provider")
			}
			if !model.Enabled {
				t.Error("copilot-chat model should be enabled")
			}
			if model.MaxRequestsPerMinute <= 0 {
				t.Error("copilot-chat model should have positive MaxRequestsPerMinute")
			}
			if model.MaxTokensPerMinute <= 0 {
				t.Error("copilot-chat model should have positive MaxTokensPerMinute")
			}
			break
		}
	}

	if !found {
		t.Error("copilot-chat model not found in default models")
	}
}
