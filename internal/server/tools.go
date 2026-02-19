package server

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"runbookmcp.dev/internal/config"
	"github.com/mark3labs/mcp-go/mcp"
)

// oneShotResponse is the MCP response for one-shot task execution.
// Stdout and Stderr are truncated to the last mcpOutputMaxLines lines.
type oneShotResponse struct {
	TaskName         string `json:"task_name,omitempty"`
	SessionID        string `json:"session_id,omitempty"`
	LogPath          string `json:"log_path,omitempty"`
	Success          bool   `json:"success"`
	ExitCode         int    `json:"exit_code"`
	Duration         string `json:"duration"`
	Error            string `json:"error,omitempty"`
	TimedOut         bool   `json:"timed_out,omitempty"`
	Stdout           string `json:"stdout,omitempty"`
	StdoutLines      int    `json:"stdout_lines,omitempty"`
	StdoutTotalLines int    `json:"stdout_total_lines,omitempty"`
	StdoutTruncated  bool   `json:"stdout_truncated,omitempty"`
	Stderr           string `json:"stderr,omitempty"`
	StderrLines      int    `json:"stderr_lines,omitempty"`
	StderrTotalLines int    `json:"stderr_total_lines,omitempty"`
	StderrTruncated  bool   `json:"stderr_truncated,omitempty"`
}

// mcpOutputMaxLines is the maximum number of output lines returned in MCP responses.
const mcpOutputMaxLines = 100

// calcHasMore reports whether there are older lines beyond what was returned.
// When lines == 0 (all lines requested), there is nothing more to page through.
func calcHasMore(totalLines, lines, offset int) bool {
	return lines > 0 && totalLines > lines+offset
}

// truncateToLines splits s into lines, returns the last max lines (or all if max<=0),
// along with the number of lines shown and the total line count.
// A trailing newline does not count as an extra empty line.
func truncateToLines(s string, max int) (result string, shown int, total int) {
	if s == "" {
		return s, 0, 0
	}
	lines := strings.Split(s, "\n")
	// Don't count a trailing empty string produced by a final newline
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	total = len(lines)
	if max > 0 && total > max {
		lines = lines[total-max:]
	}
	return strings.Join(lines, "\n"), len(lines), total
}

// registerTools registers all tasks as MCP tools
func (s *Server) registerTools() {
	// Register session management tools
	s.registerSessionManagementTools()

	// Register task-specific tools
	for taskName, taskDef := range s.manifest.Tasks {
		if taskDef.DisableMCP {
			continue
		}
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

	// Add max_output_lines parameter for clients that want unlimited output
	inputSchema.Properties["max_output_lines"] = map[string]interface{}{
		"type":        "number",
		"description": "Maximum output lines to return per stream (default 100, 0=unlimited). For CLI use.",
	}

	tool := mcp.Tool{
		Name:        toolName,
		Description: task.Description,
		InputSchema: inputSchema,
	}

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		params := req.GetArguments()

		// Read and remove max_output_lines before passing to task executor
		maxLines := mcpOutputMaxLines
		if v, ok := params["max_output_lines"].(float64); ok {
			maxLines = int(v)
			delete(params, "max_output_lines")
		}

		result, err := s.manager.ExecuteOneShot(taskName, params)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		stdout, stdoutShown, stdoutTotal := truncateToLines(result.Stdout, maxLines)
		stderr, stderrShown, stderrTotal := truncateToLines(result.Stderr, maxLines)

		resp := oneShotResponse{
			TaskName:         result.TaskName,
			SessionID:        result.SessionID,
			LogPath:          result.LogPath,
			Success:          result.Success,
			ExitCode:         result.ExitCode,
			Duration:         result.Duration.String(),
			Error:            result.Error,
			TimedOut:         result.TimedOut,
			Stdout:           stdout,
			StdoutLines:      stdoutShown,
			StdoutTotalLines: stdoutTotal,
			StdoutTruncated:  stdoutTotal > stdoutShown,
			Stderr:           stderr,
			StderrLines:      stderrShown,
			StderrTotalLines: stderrTotal,
			StderrTruncated:  stderrTotal > stderrShown,
		}

		resultJSON, err := json.Marshal(resp)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to marshal result: %v", err)), nil
		}

		return mcp.NewToolResultText(string(resultJSON)), nil
	}

	s.mcpServer.AddTool(tool, handler)
}
