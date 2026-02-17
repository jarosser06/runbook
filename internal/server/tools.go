package server

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jarosser06/runbook/internal/config"
	"github.com/mark3labs/mcp-go/mcp"
)

// registerTools registers all tasks as MCP tools
func (s *Server) registerTools() {
	// Register session management tools
	s.registerSessionManagementTools()

	// Register task-specific tools
	for taskName, taskDef := range s.manifest.Tasks {
		switch taskDef.Type {
		case config.TaskTypeOneShot:
			s.registerOneShotTool(taskName, taskDef)
		case config.TaskTypeDaemon:
			s.registerDaemonTools(taskName, taskDef)
		}
	}

	// Register workflow tools
	s.registerWorkflowTools()
}

// registerOneShotTool registers a one-shot task as an MCP tool
func (s *Server) registerOneShotTool(taskName string, task config.Task) {
	toolName := "run_" + taskName

	// Build input schema
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

	tool := mcp.Tool{
		Name:        toolName,
		Description: task.Description,
		InputSchema: inputSchema,
	}

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		params := req.GetArguments()

		result, err := s.manager.ExecuteOneShot(taskName, params)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		resultJSON, err := json.Marshal(result)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to marshal result: %v", err)), nil
		}

		return mcp.NewToolResultText(string(resultJSON)), nil
	}

	s.mcpServer.AddTool(tool, handler)
}
