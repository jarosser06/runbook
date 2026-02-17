package server

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jarosser06/runbook/internal/config"
	"github.com/jarosser06/runbook/internal/task"
	"github.com/mark3labs/mcp-go/mcp"
)

// registerRefreshConfigTool registers the refresh_config tool that reloads
// configuration from disk while the server is running.
func (s *Server) registerRefreshConfigTool() {
	tool := mcp.Tool{
		Name:        "refresh_config",
		Description: "Reload all configuration from disk. Re-registers tools, resources, and prompts without restarting the server.",
		InputSchema: mcp.ToolInputSchema{
			Type:       "object",
			Properties: make(map[string]interface{}),
		},
	}

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := s.Refresh(); err != nil {
			result := map[string]interface{}{
				"success": false,
				"error":   err.Error(),
			}
			resultJSON, _ := json.Marshal(result)
			return mcp.NewToolResultError(string(resultJSON)), nil
		}

		result := map[string]interface{}{
			"success":   true,
			"message":   "Configuration reloaded successfully",
			"tasks":     len(s.manifest.Tasks),
			"prompts":   len(s.manifest.Prompts),
			"workflows": len(s.manifest.Workflows),
		}
		resultJSON, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(resultJSON)), nil
	}

	s.mcpServer.AddTool(tool, handler)
}

// Refresh reloads configuration from disk, creates a new task manager,
// and re-registers all tools, resources, and prompts on the MCP server.
// Running daemons are not disrupted.
func (s *Server) Refresh() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Reload config from the same path used at startup
	manifest, loaded, err := config.LoadManifest(s.configPath)
	if err != nil {
		return fmt.Errorf("failed to reload config: %w", err)
	}

	if !loaded {
		return fmt.Errorf("no config found at startup path %q", s.configPath)
	}

	// Collect current tool names to remove them
	oldToolNames := s.collectToolNames()

	// Update server state
	s.manifest = manifest
	s.configLoaded = loaded
	s.manager = task.NewManager(manifest, s.processManager)

	// Remove old tools (except built-in ones we'll re-register)
	if len(oldToolNames) > 0 {
		s.mcpServer.DeleteTools(oldToolNames...)
	}

	// Re-register built-in tools if needed
	if !s.configLoaded {
		s.registerBuiltInTools()
	}

	// Re-register config-derived tools, resources, and prompts
	s.registerTools()
	s.registerResources()
	s.registerPrompts()

	return nil
}

// collectToolNames returns the names of all currently registered task-derived tools.
// This is used during refresh to know which tools to remove before re-registering.
func (s *Server) collectToolNames() []string {
	var names []string

	// Session management tools
	names = append(names, "list_sessions", "read_session_metadata", "read_session_log")

	// Task-derived tools
	for taskName, taskDef := range s.manifest.Tasks {
		switch taskDef.Type {
		case config.TaskTypeOneShot:
			names = append(names, "run_"+taskName)
		case config.TaskTypeDaemon:
			names = append(names, "start_"+taskName, "stop_"+taskName, "status_"+taskName, "logs_"+taskName)
		}
	}

	// Workflow-derived tools
	for workflowName := range s.manifest.Workflows {
		names = append(names, "run_workflow_"+workflowName)
	}

	// Built-in tools
	names = append(names, "init")

	return names
}
