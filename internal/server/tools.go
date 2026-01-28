package server

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jarosser06/dev-toolkit-mcp/internal/config"
	"github.com/jarosser06/dev-toolkit-mcp/internal/logs"
	"github.com/mark3labs/mcp-go/mcp"
)

// registerTools registers all tasks as MCP tools
func (s *Server) registerTools() {
	for taskName, taskDef := range s.manifest.Tasks {
		switch taskDef.Type {
		case config.TaskTypeOneShot:
			s.registerOneShotTool(taskName, taskDef)
		case config.TaskTypeDaemon:
			s.registerDaemonTools(taskName, taskDef)
		}
	}
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

// registerDaemonTools registers daemon task tools
func (s *Server) registerDaemonTools(taskName string, task config.Task) {
	s.registerDaemonStartTool(taskName, task)
	s.registerDaemonStopTool(taskName, task)
	s.registerDaemonStatusTool(taskName, task)
	s.registerDaemonLogsTool(taskName, task)
}

func (s *Server) registerDaemonStartTool(taskName string, task config.Task) {
	toolName := "start_" + taskName

	tool := mcp.Tool{
		Name:        toolName,
		Description: fmt.Sprintf("Start daemon: %s", task.Description),
		InputSchema: mcp.ToolInputSchema{Type: "object", Properties: make(map[string]interface{})},
	}

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		params := req.GetArguments()

		result, err := s.manager.StartDaemon(taskName, params)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		resultJSON, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(resultJSON)), nil
	}

	s.mcpServer.AddTool(tool, handler)
}

func (s *Server) registerDaemonStopTool(taskName string, task config.Task) {
	toolName := "stop_" + taskName

	tool := mcp.Tool{
		Name:        toolName,
		Description: fmt.Sprintf("Stop daemon: %s", task.Description),
		InputSchema: mcp.ToolInputSchema{Type: "object", Properties: make(map[string]interface{})},
	}

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		result, err := s.manager.StopDaemon(taskName)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		resultJSON, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(resultJSON)), nil
	}

	s.mcpServer.AddTool(tool, handler)
}

func (s *Server) registerDaemonStatusTool(taskName string, task config.Task) {
	toolName := "status_" + taskName

	tool := mcp.Tool{
		Name:        toolName,
		Description: fmt.Sprintf("Check status of daemon: %s", task.Description),
		InputSchema: mcp.ToolInputSchema{Type: "object", Properties: make(map[string]interface{})},
	}

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		status, err := s.manager.DaemonStatus(taskName)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		resultJSON, _ := json.Marshal(status)
		return mcp.NewToolResultText(string(resultJSON)), nil
	}

	s.mcpServer.AddTool(tool, handler)
}

func (s *Server) registerDaemonLogsTool(taskName string, task config.Task) {
	toolName := "logs_" + taskName

	inputSchema := mcp.ToolInputSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"lines": map[string]interface{}{
				"type":        "number",
				"description": "Number of lines to tail (default: 100)",
			},
			"filter": map[string]interface{}{
				"type":        "string",
				"description": "Regex pattern to filter logs",
			},
		},
	}

	tool := mcp.Tool{
		Name:        toolName,
		Description: fmt.Sprintf("Read logs for daemon: %s", task.Description),
		InputSchema: inputSchema,
	}

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		opts := logs.ReadOptions{Lines: 100}

		args := req.GetArguments()
		if lines, ok := args["lines"].(float64); ok {
			opts.Lines = int(lines)
		}
		if filter, ok := args["filter"].(string); ok {
			opts.Filter = filter
		}

		logLines, err := logs.ReadLog(taskName, opts)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to read logs: %v", err)), nil
		}

		result := map[string]interface{}{
			"lines": logLines,
			"count": len(logLines),
		}

		resultJSON, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(resultJSON)), nil
	}

	s.mcpServer.AddTool(tool, handler)
}
