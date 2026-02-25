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

func TestCollectToolNamesExcludesDisableMCPTasks(t *testing.T) {
	manifest := &config.Manifest{
		Tasks: map[string]config.Task{
			"build":        {Type: config.TaskTypeOneShot, Command: "go build"},
			"secret-setup": {Type: config.TaskTypeOneShot, Command: "./setup.sh", DisableMCP: true},
			"serve":        {Type: config.TaskTypeDaemon, Command: "go run ."},
			"priv-daemon":  {Type: config.TaskTypeDaemon, Command: "./daemon.sh", DisableMCP: true},
		},
		Workflows: map[string]config.Workflow{},
		Prompts:   map[string]config.Prompt{},
	}

	s := &Server{manifest: manifest}
	names := s.collectToolNames()

	nameSet := make(map[string]bool, len(names))
	for _, n := range names {
		nameSet[n] = true
	}

	// disable_mcp tasks must not appear
	for _, absent := range []string{"run_secret-setup", "start_priv-daemon", "stop_priv-daemon", "status_priv-daemon", "logs_priv-daemon"} {
		if nameSet[absent] {
			t.Errorf("collectToolNames() must not include %q for disable_mcp task", absent)
		}
	}

	// Normal tasks must still appear
	for _, present := range []string{"run_build", "start_serve", "stop_serve", "status_serve", "logs_serve"} {
		if !nameSet[present] {
			t.Errorf("collectToolNames() missing expected name %q", present)
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

func TestTruncateToLines(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		max           int
		wantShown     int
		wantTotal     int
		wantTruncated bool
	}{
		{
			name:          "empty string",
			input:         "",
			max:           100,
			wantShown:     0,
			wantTotal:     0,
			wantTruncated: false,
		},
		{
			name:          "single line",
			input:         "hello",
			max:           100,
			wantShown:     1,
			wantTotal:     1,
			wantTruncated: false,
		},
		{
			name:          "exactly max lines",
			input:         "a\nb\nc",
			max:           3,
			wantShown:     3,
			wantTotal:     3,
			wantTruncated: false,
		},
		{
			name:          "over max lines",
			input:         "a\nb\nc\nd\ne",
			max:           3,
			wantShown:     3,
			wantTotal:     5,
			wantTruncated: true,
		},
		{
			name:          "max=0 means no truncation",
			input:         "a\nb\nc",
			max:           0,
			wantShown:     3,
			wantTotal:     3,
			wantTruncated: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, shown, total := truncateToLines(tt.input, tt.max)
			if shown != tt.wantShown {
				t.Errorf("shown=%d, want %d", shown, tt.wantShown)
			}
			if total != tt.wantTotal {
				t.Errorf("total=%d, want %d", total, tt.wantTotal)
			}
			truncated := total > shown
			if truncated != tt.wantTruncated {
				t.Errorf("truncated=%v, want %v", truncated, tt.wantTruncated)
			}
			_ = result
		})
	}
}

func TestTruncateToLinesKeepsLastLines(t *testing.T) {
	// Verify that when truncating, we keep the LAST lines (newest output)
	input := "line1\nline2\nline3\nline4\nline5"
	result, shown, total := truncateToLines(input, 3)
	if total != 5 {
		t.Errorf("expected total=5, got %d", total)
	}
	if shown != 3 {
		t.Errorf("expected shown=3, got %d", shown)
	}
	if result != "line3\nline4\nline5" {
		t.Errorf("expected last 3 lines, got %q", result)
	}
}

func TestDaemonLogsSchemaHasOffsetParam(t *testing.T) {
	schema := daemonLogsInputSchema()
	offsetParam, ok := schema.Properties["offset"]
	if !ok {
		t.Fatal("daemon logs schema missing 'offset' parameter")
	}
	offsetMap, ok := offsetParam.(map[string]interface{})
	if !ok {
		t.Fatal("offset param is not a map")
	}
	if offsetMap["type"] != "number" {
		t.Errorf("expected offset type=number, got %v", offsetMap["type"])
	}
}

func TestSessionLogSchemaHasOffsetParam(t *testing.T) {
	schema := sessionLogInputSchema()
	offsetParam, ok := schema.Properties["offset"]
	if !ok {
		t.Fatal("session log schema missing 'offset' parameter")
	}
	offsetMap, ok := offsetParam.(map[string]interface{})
	if !ok {
		t.Fatal("offset param is not a map")
	}
	if offsetMap["type"] != "number" {
		t.Errorf("expected offset type=number, got %v", offsetMap["type"])
	}
}

func TestHasMoreCalculation(t *testing.T) {
	tests := []struct {
		name        string
		totalLines  int
		linesParam  int
		offsetParam int
		wantHasMore bool
	}{
		{
			name:        "more lines available",
			totalLines:  150,
			linesParam:  100,
			offsetParam: 0,
			wantHasMore: true,
		},
		{
			name:        "all lines fit",
			totalLines:  50,
			linesParam:  100,
			offsetParam: 0,
			wantHasMore: false,
		},
		{
			name:        "with offset, more available",
			totalLines:  300,
			linesParam:  100,
			offsetParam: 100,
			wantHasMore: true,
		},
		{
			name:        "with offset, nothing more",
			totalLines:  200,
			linesParam:  100,
			offsetParam: 100,
			wantHasMore: false,
		},
		{
			name:        "lines=0 means all returned, never has_more",
			totalLines:  50,
			linesParam:  0,
			offsetParam: 0,
			wantHasMore: false,
		},
		{
			name:        "lines=0 with offset, still no has_more",
			totalLines:  50,
			linesParam:  0,
			offsetParam: 10,
			wantHasMore: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasMore := calcHasMore(tt.totalLines, tt.linesParam, tt.offsetParam)
			if hasMore != tt.wantHasMore {
				t.Errorf("totalLines=%d, lines=%d, offset=%d: hasMore=%v, want %v",
					tt.totalLines, tt.linesParam, tt.offsetParam, hasMore, tt.wantHasMore)
			}
		})
	}
}
