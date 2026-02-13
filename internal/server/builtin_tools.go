package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mark3labs/mcp-go/mcp"
)

const minimalConfigTemplate = `version: "1.0"

# Example tasks - customize these for your project
tasks:
  build:
    description: "Build the project"
    command: "echo 'Add your build command here'"
    type: oneshot

  test:
    description: "Run tests"
    command: "echo 'Add your test command here'"
    type: oneshot

  lint:
    description: "Run linter"
    command: "echo 'Add your lint command here'"
    type: oneshot

# Task groups organize related tasks
task_groups:
  ci:
    description: "CI pipeline tasks"
    tasks:
      - lint
      - test
      - build
`

// registerBuiltInTools registers built-in tools that are always available
func (s *Server) registerBuiltInTools() {
	s.registerInitTool()
}

// registerInitTool registers the init tool for creating config files
func (s *Server) registerInitTool() {
	tool := mcp.Tool{
		Name:        "init",
		Description: "Initialize a new .dev_workflow.yaml configuration file",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Target path for config file (default: ./.dev_workflow.yaml)",
				},
				"overwrite": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether to overwrite existing file (default: false)",
				},
			},
		},
	}

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()

		// Get path parameter (default to ./.dev_workflow.yaml)
		targetPath := "./.dev_workflow.yaml"
		if path, ok := args["path"].(string); ok && path != "" {
			targetPath = path
		}

		// Get overwrite parameter (default to false)
		overwrite := false
		if ow, ok := args["overwrite"].(bool); ok {
			overwrite = ow
		}

		// Convert to absolute path for better error messages
		absPath, err := filepath.Abs(targetPath)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid path: %v", err)), nil
		}

		// Check if file exists
		if _, err := os.Stat(absPath); err == nil && !overwrite {
			return mcp.NewToolResultError(fmt.Sprintf("file already exists at %s (use overwrite=true to replace)", absPath)), nil
		}

		// Create directory if needed
		dir := filepath.Dir(absPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to create directory: %v", err)), nil
		}

		// Write config file
		if err := os.WriteFile(absPath, []byte(minimalConfigTemplate), 0644); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to write config file: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf(`{
  "success": true,
  "path": %q,
  "message": "Successfully created config file. Restart the MCP server to load the new configuration."
}`, absPath)), nil
	}

	s.mcpServer.AddTool(tool, handler)
}
