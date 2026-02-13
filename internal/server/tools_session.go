package server

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jarosser06/dev-workflow-mcp/internal/logs"
	"github.com/mark3labs/mcp-go/mcp"
)

// registerSessionManagementTools registers global session management tools
func (s *Server) registerSessionManagementTools() {
	s.registerListSessionsTool()
	s.registerReadSessionMetadataTool()
	s.registerReadSessionLogTool()
}

// registerListSessionsTool registers the list_sessions tool
func (s *Server) registerListSessionsTool() {
	inputSchema := mcp.ToolInputSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"task_name": map[string]interface{}{
				"type":        "string",
				"description": "Name of the task to list sessions for",
			},
			"limit": map[string]interface{}{
				"type":        "number",
				"description": "Maximum number of sessions to return (default: 20)",
			},
		},
		Required: []string{"task_name"},
	}

	tool := mcp.Tool{
		Name:        "list_sessions",
		Description: "List recent execution sessions for a task",
		InputSchema: inputSchema,
	}

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()

		taskName, ok := args["task_name"].(string)
		if !ok {
			return mcp.NewToolResultError("task_name is required"), nil
		}

		limit := 20
		if l, ok := args["limit"].(float64); ok {
			limit = int(l)
		}

		sessions, err := logs.ListSessions(taskName, limit)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to list sessions: %v", err)), nil
		}

		resultJSON, _ := json.Marshal(sessions)
		return mcp.NewToolResultText(string(resultJSON)), nil
	}

	s.mcpServer.AddTool(tool, handler)
}

// registerReadSessionMetadataTool registers the read_session_metadata tool
func (s *Server) registerReadSessionMetadataTool() {
	inputSchema := mcp.ToolInputSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"session_id": map[string]interface{}{
				"type":        "string",
				"description": "Session ID to read metadata for",
			},
		},
		Required: []string{"session_id"},
	}

	tool := mcp.Tool{
		Name:        "read_session_metadata",
		Description: "Read metadata for a specific execution session",
		InputSchema: inputSchema,
	}

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()

		sessionID, ok := args["session_id"].(string)
		if !ok {
			return mcp.NewToolResultError("session_id is required"), nil
		}

		metadata, err := logs.ReadSessionMetadata(sessionID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to read session metadata: %v", err)), nil
		}

		resultJSON, _ := json.Marshal(metadata)
		return mcp.NewToolResultText(string(resultJSON)), nil
	}

	s.mcpServer.AddTool(tool, handler)
}

// registerReadSessionLogTool registers the read_session_log tool
func (s *Server) registerReadSessionLogTool() {
	inputSchema := mcp.ToolInputSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"session_id": map[string]interface{}{
				"type":        "string",
				"description": "Session ID to read logs for",
			},
			"lines": map[string]interface{}{
				"type":        "number",
				"description": "Number of lines to tail (0 means all, default: 0)",
			},
			"filter": map[string]interface{}{
				"type":        "string",
				"description": "Regex pattern to filter logs",
			},
		},
		Required: []string{"session_id"},
	}

	tool := mcp.Tool{
		Name:        "read_session_log",
		Description: "Read log output for a specific execution session",
		InputSchema: inputSchema,
	}

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()

		sessionID, ok := args["session_id"].(string)
		if !ok {
			return mcp.NewToolResultError("session_id is required"), nil
		}

		opts := logs.ReadOptions{
			SessionID: sessionID,
			Lines:     0, // Read all lines by default
		}

		if lines, ok := args["lines"].(float64); ok {
			opts.Lines = int(lines)
		}
		if filter, ok := args["filter"].(string); ok {
			opts.Filter = filter
		}

		logLines, err := logs.ReadSessionLog(sessionID, opts)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to read session log: %v", err)), nil
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
