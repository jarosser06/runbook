package server

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jarosser06/dev-workflow-mcp/internal/config"
	"github.com/mark3labs/mcp-go/mcp"
)

// registerWorkflowTools registers all workflows as MCP tools
func (s *Server) registerWorkflowTools() {
	for workflowName, workflow := range s.manifest.Workflows {
		s.registerWorkflowTool(workflowName, workflow)
	}
}

// registerWorkflowTool registers a single workflow as an MCP tool
func (s *Server) registerWorkflowTool(workflowName string, workflow config.Workflow) {
	toolName := "run_workflow_" + workflowName

	// Build description with step names
	stepNames := make([]string, len(workflow.Steps))
	for i, step := range workflow.Steps {
		stepNames[i] = step.Task
	}
	description := fmt.Sprintf("%s (steps: %s)", workflow.Description, strings.Join(stepNames, " -> "))

	// Build input schema from workflow parameters
	inputSchema := mcp.ToolInputSchema{
		Type:       "object",
		Properties: make(map[string]interface{}),
		Required:   []string{},
	}

	for paramName, param := range workflow.Parameters {
		paramSchema := map[string]interface{}{
			"type":        param.Type,
			"description": param.Description,
		}
		inputSchema.Properties[paramName] = paramSchema
		if param.Required {
			inputSchema.Required = append(inputSchema.Required, paramName)
		}
	}

	tool := mcp.Tool{
		Name:        toolName,
		Description: description,
		InputSchema: inputSchema,
	}

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		params := req.GetArguments()

		result, err := s.manager.ExecuteWorkflow(workflowName, params)
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
