package server

import (
	"testing"

	"runbookmcp.dev/internal/config"
	"github.com/mark3labs/mcp-go/mcp"
)

func TestCollectToolNamesExcludesRefreshConfig(t *testing.T) {
	manifest := &config.Manifest{
		Tasks: map[string]config.Task{
			"build": {Type: config.TaskTypeOneShot, Command: "go build"},
			"serve": {Type: config.TaskTypeDaemon, Command: "go run ."},
		},
		Workflows: map[string]config.Workflow{},
		Prompts:   map[string]config.Prompt{},
	}

	s := &Server{manifest: manifest}
	names := s.collectToolNames()

	for _, name := range names {
		if name == "refresh_config" {
			t.Errorf("collectToolNames() must not include 'refresh_config' (it is never deleted during refresh)")
		}
	}

	// Sanity-check that expected names are present
	expected := map[string]bool{
		"run_build":    false,
		"start_serve":  false,
		"stop_serve":   false,
		"status_serve": false,
		"logs_serve":   false,
		"init":         false,
	}
	for _, name := range names {
		if _, ok := expected[name]; ok {
			expected[name] = true
		}
	}
	for name, found := range expected {
		if !found {
			t.Errorf("collectToolNames() missing expected name %q", name)
		}
	}
}

// buildOneShotToolSchema tests the schema building logic directly
func buildOneShotToolSchema(task config.Task) mcp.ToolInputSchema {
	inputSchema := mcp.ToolInputSchema{
		Type:       "object",
		Properties: make(map[string]interface{}),
		Required:   []string{},
	}

	for paramName, param := range task.Parameters {
		paramSchema := map[string]interface{}{
			"type":        param.Type,
			"description": param.Description,
		}
		inputSchema.Properties[paramName] = paramSchema
		if param.Required {
			inputSchema.Required = append(inputSchema.Required, paramName)
		}
	}

	// Add working_directory parameter if exposed
	if task.ExposeWorkingDirectory {
		inputSchema.Properties["working_directory"] = map[string]interface{}{
			"type":        "string",
			"description": "Working directory for command execution (overrides static value)",
		}
	}

	return inputSchema
}

func TestToolSchemaWithExposeWorkingDirectory(t *testing.T) {
	tests := []struct {
		name                      string
		task                      config.Task
		shouldHaveWorkingDirParam bool
		expectedParamCount        int
	}{
		{
			name: "oneshot task with expose_working_directory=true",
			task: config.Task{
				Description:            "Test task",
				Command:                "echo test",
				Type:                   config.TaskTypeOneShot,
				WorkingDirectory:       ".",
				ExposeWorkingDirectory: true,
			},
			shouldHaveWorkingDirParam: true,
			expectedParamCount:        1, // Only working_directory
		},
		{
			name: "oneshot task with expose_working_directory=false",
			task: config.Task{
				Description:            "Test task",
				Command:                "echo test",
				Type:                   config.TaskTypeOneShot,
				WorkingDirectory:       ".",
				ExposeWorkingDirectory: false,
			},
			shouldHaveWorkingDirParam: false,
			expectedParamCount:        0, // No parameters
		},
		{
			name: "oneshot task with parameters and expose_working_directory=true",
			task: config.Task{
				Description:            "Test task with params",
				Command:                "echo {{.message}}",
				Type:                   config.TaskTypeOneShot,
				WorkingDirectory:       ".",
				ExposeWorkingDirectory: true,
				Parameters: map[string]config.Param{
					"message": {
						Type:        "string",
						Required:    true,
						Description: "Message to echo",
					},
				},
			},
			shouldHaveWorkingDirParam: true,
			expectedParamCount:        2, // message + working_directory
		},
		{
			name: "task with parameters but no expose_working_directory",
			task: config.Task{
				Description:            "Test task with params",
				Command:                "echo {{.message}}",
				Type:                   config.TaskTypeOneShot,
				WorkingDirectory:       ".",
				ExposeWorkingDirectory: false,
				Parameters: map[string]config.Param{
					"message": {
						Type:        "string",
						Required:    true,
						Description: "Message to echo",
					},
					"count": {
						Type:        "number",
						Required:    false,
						Description: "How many times",
					},
				},
			},
			shouldHaveWorkingDirParam: false,
			expectedParamCount:        2, // message + count only
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build the schema using the same logic as registerOneShotTool
			schema := buildOneShotToolSchema(tt.task)

			// schema.Properties is already map[string]interface{}
			properties := schema.Properties

			// Verify parameter count
			if len(properties) != tt.expectedParamCount {
				t.Errorf("expected %d parameters in schema, got %d", tt.expectedParamCount, len(properties))
			}

			// Check for working_directory parameter
			_, hasWorkingDir := properties["working_directory"]

			if tt.shouldHaveWorkingDirParam && !hasWorkingDir {
				t.Errorf("expected working_directory parameter in schema, but it was not found")
				t.Logf("Schema properties: %v", properties)
			}

			if !tt.shouldHaveWorkingDirParam && hasWorkingDir {
				t.Errorf("expected no working_directory parameter in schema, but it was found")
			}

			// If working_directory should be present, validate its structure
			if tt.shouldHaveWorkingDirParam && hasWorkingDir {
				workingDirParam, ok := properties["working_directory"].(map[string]interface{})
				if !ok {
					t.Fatalf("expected working_directory to be map[string]interface{}, got %T", properties["working_directory"])
				}

				if workingDirParam["type"] != "string" {
					t.Errorf("expected working_directory type to be 'string', got %v", workingDirParam["type"])
				}

				desc, hasDesc := workingDirParam["description"]
				if !hasDesc || desc == "" {
					t.Errorf("expected working_directory to have a description")
				}
			}

			// Verify other parameters are still present
			if tt.task.Parameters != nil {
				for paramName := range tt.task.Parameters {
					if _, exists := properties[paramName]; !exists {
						t.Errorf("expected parameter %q to be in schema", paramName)
					}
				}
			}
		})
	}
}
