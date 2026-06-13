package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"runbookmcp.dev/internal/dirs"
	"runbookmcp.dev/internal/logs"
	"github.com/mark3labs/mcp-go/mcp"
)

// registerSetWorkingDirTool registers the set_working_directory tool, which
// switches runbook's effective working directory and reloads configuration
// from the new directory's .runbook/ — without restarting the server.
func (s *Server) registerSetWorkingDirTool() {
	tool := mcp.Tool{
		Name: "set_working_directory",
		Description: "Switch runbook's effective working directory to a new path and reload its configuration. " +
			"Re-registers all tools, resources, and prompts from the new directory's " + dirs.ConfigDir + "/ config " +
			"without restarting the server. Use this to point runbook at a different project, e.g. another app in a " +
			"monorepo. Only available in local mode; not exposed when running as a shared HTTP server.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"directory": map[string]interface{}{
					"type":        "string",
					"description": "Path to the new working directory (absolute, or relative to the current working directory).",
				},
			},
			Required: []string{"directory"},
		},
	}

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		dir, _ := args["directory"].(string)
		if dir == "" {
			return mcp.NewToolResultError(`{"success":false,"error":"directory is required"}`), nil
		}

		loaded, err := s.SwitchWorkingDirectory(dir)
		if err != nil {
			result := map[string]interface{}{"success": false, "error": err.Error()}
			b, _ := json.Marshal(result)
			return mcp.NewToolResultError(string(b)), nil
		}

		cwd, _ := os.Getwd()
		result := map[string]interface{}{
			"success":           true,
			"working_directory": cwd,
			"config_loaded":     loaded,
			"tasks":             len(s.manifest.Tasks),
			"prompts":           len(s.manifest.Prompts),
			"workflows":         len(s.manifest.Workflows),
		}
		if loaded {
			result["message"] = "Switched working directory and reloaded configuration."
		} else {
			result["message"] = "Switched working directory, but no " + dirs.ConfigDir + "/ config was found there. Use the init tool to create one."
		}
		b, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(b)), nil
	}

	s.mcpServer.AddTool(tool, handler)
}

// SwitchWorkingDirectory changes the process working directory to dir and
// reloads configuration from that directory's default location. All relative
// runbook paths (the .runbook/ config dir, the ._runbook_state/ state dir, and
// task working directories) resolve against the new directory afterwards.
//
// It returns whether a config file was found and loaded in the new directory.
func (s *Server) SwitchWorkingDirectory(dir string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	abs, err := filepath.Abs(dir)
	if err != nil {
		return false, fmt.Errorf("invalid directory %q: %w", dir, err)
	}

	info, err := os.Stat(abs)
	if err != nil {
		return false, fmt.Errorf("cannot access directory %q: %w", abs, err)
	}
	if !info.IsDir() {
		return false, fmt.Errorf("%q is not a directory", abs)
	}

	if err := os.Chdir(abs); err != nil {
		return false, fmt.Errorf("failed to change directory to %q: %w", abs, err)
	}

	// Re-create the state/log directory structure under the new working dir.
	if err := logs.Setup(); err != nil {
		return false, fmt.Errorf("failed to set up logs in %q: %w", abs, err)
	}

	// Load config from the new directory's default location, ignoring any
	// config path supplied at startup — switching directories means adopting
	// the new directory's runbook configuration.
	s.configPath = ""

	return s.reloadLocked()
}
