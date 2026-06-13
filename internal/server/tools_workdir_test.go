package server

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"runbookmcp.dev/internal/config"
	"runbookmcp.dev/internal/dirs"
	"runbookmcp.dev/internal/task"
	"github.com/mark3labs/mcp-go/mcp"
)

func emptyManifest() *config.Manifest {
	return &config.Manifest{Version: "1.0", Tasks: map[string]config.Task{}}
}

func TestSwitchWorkingDirectoryLoadsNewConfig(t *testing.T) {
	// newTestServer chdirs into a fresh temp dir (with cleanup) that has no config.
	s := newTestServer(t, emptyManifest())

	// Build a second directory that has a .runbook/ config defining a task.
	projectDir := t.TempDir()
	cfgDir := filepath.Join(projectDir, dirs.ConfigDir)
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatal(err)
	}
	cfg := `version: "1.0"
tasks:
  greet:
    description: "say hi"
    command: "echo hi"
    type: oneshot
`
	if err := os.WriteFile(filepath.Join(cfgDir, "tasks.yaml"), []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}

	loaded, err := s.SwitchWorkingDirectory(projectDir)
	if err != nil {
		t.Fatalf("SwitchWorkingDirectory: %v", err)
	}
	if !loaded {
		t.Fatalf("expected config to be loaded from %s", projectDir)
	}

	// CWD must now be the project dir (resolve symlinks; macOS /tmp is a symlink).
	cwd, _ := os.Getwd()
	gotReal, _ := filepath.EvalSymlinks(cwd)
	wantReal, _ := filepath.EvalSymlinks(projectDir)
	if gotReal != wantReal {
		t.Errorf("cwd = %q, want %q", gotReal, wantReal)
	}

	// The new task must be present in the reloaded manifest.
	if _, ok := s.manifest.Tasks["greet"]; !ok {
		t.Errorf("expected task 'greet' after switch; tasks = %v", s.manifest.Tasks)
	}
}

func TestSwitchWorkingDirectoryToDirWithoutConfig(t *testing.T) {
	s := newTestServer(t, emptyManifest())

	emptyDir := t.TempDir()
	loaded, err := s.SwitchWorkingDirectory(emptyDir)
	if err != nil {
		t.Fatalf("SwitchWorkingDirectory: %v", err)
	}
	if loaded {
		t.Errorf("expected no config loaded in dir without %s/", dirs.ConfigDir)
	}
}

// chdirToTemp moves into a fresh temp dir for the duration of the test and
// initializes the log/state structure there (NewServer expects it).
func chdirToTemp(t *testing.T) string {
	t.Helper()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	return dir
}

// TestSetWorkingDirToolGatedToLocalMode verifies that NewServer (the shared
// bootstrap for both stdio and HTTP modes) does NOT expose set_working_directory.
// HTTP mode (ServeHTTP) leaves it that way; only the stdio Serve() path registers
// it via registerSetWorkingDirTool. A process-global os.Chdir must never be
// reachable by clients of a shared HTTP server.
func TestSetWorkingDirToolGatedToLocalMode(t *testing.T) {
	chdirToTemp(t)
	manifest := emptyManifest()
	s := NewServer(manifest, task.NewManager(manifest, nil), nil, false, "test", "")

	if _, ok := s.mcpServer.ListTools()["set_working_directory"]; ok {
		t.Fatalf("set_working_directory must NOT be registered by NewServer (HTTP-mode contract)")
	}
	// refresh_config, by contrast, is safe in both modes and must be present.
	if _, ok := s.mcpServer.ListTools()["refresh_config"]; !ok {
		t.Fatalf("refresh_config should be registered in all modes")
	}

	// Simulate the stdio Serve() path, which is the only place the tool is added.
	s.registerSetWorkingDirTool()
	if _, ok := s.mcpServer.ListTools()["set_working_directory"]; !ok {
		t.Fatalf("set_working_directory must be registered after the stdio Serve() path")
	}
}

// TestSetWorkingDirNotDeletedByRefresh guards that the tool, once registered in
// local mode, is not swept away by a config refresh (it is intentionally absent
// from collectToolNames, which drives DeleteTools on refresh).
func TestSetWorkingDirNotDeletedByRefresh(t *testing.T) {
	manifest := &config.Manifest{
		Tasks:     map[string]config.Task{"build": {Type: config.TaskTypeOneShot, Command: "go build"}},
		Workflows: map[string]config.Workflow{},
		Prompts:   map[string]config.Prompt{},
	}
	s := &Server{manifest: manifest}
	for _, name := range s.collectToolNames() {
		if name == "set_working_directory" {
			t.Fatalf("collectToolNames() must not include set_working_directory (it would be deleted on refresh)")
		}
	}
}

// TestSetWorkingDirToolHandlerEndToEnd drives the actual registered MCP tool
// handler (not just the SwitchWorkingDirectory method) and verifies it switches
// the directory, reports success, and reloads the target config.
func TestSetWorkingDirToolHandlerEndToEnd(t *testing.T) {
	chdirToTemp(t)
	manifest := emptyManifest()
	s := NewServer(manifest, task.NewManager(manifest, nil), nil, false, "test", "")
	s.registerSetWorkingDirTool()

	projectDir := t.TempDir()
	cfgDir := filepath.Join(projectDir, dirs.ConfigDir)
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatal(err)
	}
	cfg := `version: "1.0"
tasks:
  greet:
    description: "say hi"
    command: "echo hi"
    type: oneshot
`
	if err := os.WriteFile(filepath.Join(cfgDir, "tasks.yaml"), []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}

	tool := s.mcpServer.GetTool("set_working_directory")
	if tool == nil || tool.Handler == nil {
		t.Fatal("set_working_directory handler not found")
	}

	var req mcp.CallToolRequest
	req.Params.Name = "set_working_directory"
	req.Params.Arguments = map[string]any{"directory": projectDir}

	res, err := tool.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if res.IsError {
		t.Fatalf("handler reported tool error: %+v", res.Content)
	}

	// Parse the JSON result the handler returns.
	text, ok := mcp.AsTextContent(res.Content[0])
	if !ok {
		t.Fatalf("expected text content, got %T", res.Content[0])
	}
	var payload struct {
		Success      bool   `json:"success"`
		ConfigLoaded bool   `json:"config_loaded"`
		Tasks        int    `json:"tasks"`
		WorkingDir   string `json:"working_directory"`
	}
	if err := json.Unmarshal([]byte(text.Text), &payload); err != nil {
		t.Fatalf("unmarshal result %q: %v", text.Text, err)
	}
	if !payload.Success || !payload.ConfigLoaded {
		t.Fatalf("expected success+config_loaded, got %+v", payload)
	}
	if payload.Tasks != 1 {
		t.Errorf("expected 1 task from new config, got %d", payload.Tasks)
	}

	// CWD actually moved (resolve symlinks; macOS /tmp is symlinked).
	cwd, _ := os.Getwd()
	gotReal, _ := filepath.EvalSymlinks(cwd)
	wantReal, _ := filepath.EvalSymlinks(projectDir)
	if gotReal != wantReal {
		t.Errorf("cwd = %q, want %q", gotReal, wantReal)
	}

	// The newly registered tool from the target config is now live on the server.
	if _, ok := s.mcpServer.ListTools()["run_greet"]; !ok {
		t.Errorf("expected run_greet tool after directory switch")
	}
}

func TestSwitchWorkingDirectoryRejectsMissingPath(t *testing.T) {
	s := newTestServer(t, emptyManifest())

	if _, err := s.SwitchWorkingDirectory(filepath.Join(t.TempDir(), "does-not-exist")); err == nil {
		t.Errorf("expected error for nonexistent directory")
	}
}
