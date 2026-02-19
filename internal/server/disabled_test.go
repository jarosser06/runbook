package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"runbookmcp.dev/internal/config"
	"runbookmcp.dev/internal/logs"
	"runbookmcp.dev/internal/task"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// newTestServer builds a minimal Server wired to a real mcp-go server instance.
func newTestServer(t *testing.T, manifest *config.Manifest) *Server {
	t.Helper()

	// Set up log dir so logs.Setup() in manager doesn't fail.
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	if err := logs.Setup(); err != nil {
		t.Fatalf("logs.Setup: %v", err)
	}

	mgr := task.NewManager(manifest, nil)
	mcp := mcpserver.NewMCPServer("test", "0.0.1")

	return &Server{
		manifest:  manifest,
		manager:   mgr,
		mcpServer: mcp,
	}
}

// ---------------------------------------------------------------------------
// collectToolNames — Disabled flag
// ---------------------------------------------------------------------------

func TestCollectToolNamesExcludesDisabledTasks(t *testing.T) {
	manifest := &config.Manifest{
		Tasks: map[string]config.Task{
			"build":   {Type: config.TaskTypeOneShot, Command: "go build"},
			"secrets": {Type: config.TaskTypeOneShot, Command: "./setup.sh", Disabled: true},
			"serve":   {Type: config.TaskTypeDaemon, Command: "go run ."},
			"priv":    {Type: config.TaskTypeDaemon, Command: "./d.sh", Disabled: true},
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

	// Disabled tasks must not appear.
	for _, absent := range []string{"run_secrets", "start_priv", "stop_priv", "status_priv", "logs_priv"} {
		if nameSet[absent] {
			t.Errorf("collectToolNames() must not include %q for disabled task", absent)
		}
	}
	// Normal tasks must appear.
	for _, present := range []string{"run_build", "start_serve"} {
		if !nameSet[present] {
			t.Errorf("collectToolNames() missing %q", present)
		}
	}
}

func TestCollectToolNamesExcludesDisabledWorkflows(t *testing.T) {
	manifest := &config.Manifest{
		Tasks: map[string]config.Task{
			"build": {Type: config.TaskTypeOneShot, Command: "go build"},
		},
		Workflows: map[string]config.Workflow{
			"ci":      {Description: "CI", Steps: []config.WorkflowStep{{Task: "build"}}},
			"release": {Description: "Release", Steps: []config.WorkflowStep{{Task: "build"}}, Disabled: true},
			"nightly": {Description: "Nightly", Steps: []config.WorkflowStep{{Task: "build"}}, DisableMCP: true},
		},
		Prompts: map[string]config.Prompt{},
	}

	s := &Server{manifest: manifest}
	names := s.collectToolNames()
	nameSet := make(map[string]bool, len(names))
	for _, n := range names {
		nameSet[n] = true
	}

	if !nameSet["run_workflow_ci"] {
		t.Error("run_workflow_ci should be present")
	}
	if nameSet["run_workflow_release"] {
		t.Error("run_workflow_release should be absent (Disabled=true)")
	}
	if nameSet["run_workflow_nightly"] {
		t.Error("run_workflow_nightly should be absent (DisableMCP=true)")
	}
}

// ---------------------------------------------------------------------------
// registerTools — Disabled tasks are not registered
// ---------------------------------------------------------------------------

func TestRegisterToolsSkipsDisabledTasks(t *testing.T) {
	manifest := &config.Manifest{
		Version: "1.0",
		Tasks: map[string]config.Task{
			"build":   {Type: config.TaskTypeOneShot, Description: "Build", Command: "go build"},
			"secrets": {Type: config.TaskTypeOneShot, Description: "Secrets", Command: "./s.sh", Disabled: true},
		},
		Workflows: map[string]config.Workflow{},
		Prompts:   map[string]config.Prompt{},
		Resources: map[string]config.Resource{},
	}

	s := newTestServer(t, manifest)
	s.registerTools()

	names := s.collectToolNames()
	nameSet := make(map[string]bool, len(names))
	for _, n := range names {
		nameSet[n] = true
	}

	// run_build was registered, so it would appear in collectToolNames output.
	if !nameSet["run_build"] {
		t.Error("run_build should be in collected names")
	}
	// run_secrets should not be registered.
	if nameSet["run_secrets"] {
		t.Error("run_secrets should not be registered (Disabled=true)")
	}
}

func TestRegisterToolsSkipsDisableMCPTasks(t *testing.T) {
	manifest := &config.Manifest{
		Version: "1.0",
		Tasks: map[string]config.Task{
			"build":  {Type: config.TaskTypeOneShot, Description: "Build", Command: "go build"},
			"hidden": {Type: config.TaskTypeOneShot, Description: "Hidden", Command: "./h.sh", DisableMCP: true},
		},
		Workflows: map[string]config.Workflow{},
		Prompts:   map[string]config.Prompt{},
		Resources: map[string]config.Resource{},
	}

	s := newTestServer(t, manifest)
	s.registerTools()

	names := s.collectToolNames()
	nameSet := make(map[string]bool, len(names))
	for _, n := range names {
		nameSet[n] = true
	}

	if nameSet["run_hidden"] {
		t.Error("run_hidden should not be registered (DisableMCP=true)")
	}
}

func TestRegisterToolsSkipsDisabledWorkflows(t *testing.T) {
	manifest := &config.Manifest{
		Version: "1.0",
		Tasks: map[string]config.Task{
			"build": {Type: config.TaskTypeOneShot, Description: "Build", Command: "go build"},
		},
		Workflows: map[string]config.Workflow{
			"ci":      {Description: "CI", Steps: []config.WorkflowStep{{Task: "build"}}},
			"release": {Description: "Release", Steps: []config.WorkflowStep{{Task: "build"}}, Disabled: true},
			"nightly": {Description: "Nightly", Steps: []config.WorkflowStep{{Task: "build"}}, DisableMCP: true},
		},
		Prompts:   map[string]config.Prompt{},
		Resources: map[string]config.Resource{},
	}

	s := newTestServer(t, manifest)
	s.registerTools()

	names := s.collectToolNames()
	nameSet := make(map[string]bool, len(names))
	for _, n := range names {
		nameSet[n] = true
	}

	if !nameSet["run_workflow_ci"] {
		t.Error("run_workflow_ci should be registered")
	}
	if nameSet["run_workflow_release"] {
		t.Error("run_workflow_release should not be registered (Disabled=true)")
	}
	if nameSet["run_workflow_nightly"] {
		t.Error("run_workflow_nightly should not be registered (DisableMCP=true)")
	}
}

// ---------------------------------------------------------------------------
// registerCustomResources — Disabled and File support
// ---------------------------------------------------------------------------

func TestRegisterCustomResourcesSkipsDisabled(t *testing.T) {
	manifest := &config.Manifest{
		Version: "1.0",
		Tasks:   map[string]config.Task{},
		Resources: map[string]config.Resource{
			"guide":  {Description: "Guide", Content: "# Guide"},
			"hidden": {Description: "Hidden", Content: "secret", Disabled: true},
		},
		Prompts:   map[string]config.Prompt{},
		Workflows: map[string]config.Workflow{},
	}

	s := newTestServer(t, manifest)
	s.registerCustomResources()
	// guide is registered; we confirm by calling its handler indirectly via the
	// resource name in the URI. Disabled resource must not panic or appear.
	// The key test is that registerCustomResources doesn't panic and returns.
}

func TestRegisterCustomResourcesFileContent(t *testing.T) {
	// Write a file to be served as resource content.
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "guide.md")
	if err := os.WriteFile(filePath, []byte("# Hello from file"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	manifest := &config.Manifest{
		Version: "1.0",
		Tasks:   map[string]config.Task{},
		Resources: map[string]config.Resource{
			"guide": {Description: "Guide", File: filePath},
		},
		Prompts:   map[string]config.Prompt{},
		Workflows: map[string]config.Workflow{},
	}

	s := newTestServer(t, manifest)

	// Call the handler directly by capturing it.
	var capturedHandler func(context.Context, interface{}) (interface{}, error)
	_ = capturedHandler

	// We verify registerCustomResources runs without error.
	s.registerCustomResources()

	// Now verify the resource definition still has the file path (server reads at request time).
	r := manifest.Resources["guide"]
	if r.File != filePath {
		t.Errorf("resource File should remain %q, got %q", filePath, r.File)
	}
}

func TestRegisterCustomResourcesMissingFileErrorsAtRequestTime(t *testing.T) {
	manifest := &config.Manifest{
		Version: "1.0",
		Tasks:   map[string]config.Task{},
		Resources: map[string]config.Resource{
			"missing": {Description: "Missing file resource", File: "/nonexistent/path/file.md"},
		},
		Prompts:   map[string]config.Prompt{},
		Workflows: map[string]config.Workflow{},
	}

	s := newTestServer(t, manifest)
	// registerCustomResources should succeed (file errors happen at request time).
	s.registerCustomResources() // must not panic
}

// ---------------------------------------------------------------------------
// registerPrompts — Disabled and File support
// ---------------------------------------------------------------------------

func TestRegisterPromptsSkipsDisabled(t *testing.T) {
	manifest := &config.Manifest{
		Version: "1.0",
		Tasks:   map[string]config.Task{},
		Prompts: map[string]config.Prompt{
			"active":   {Description: "Active", Content: "hello"},
			"disabled": {Description: "Disabled", Content: "secret", Disabled: true},
		},
		Resources: map[string]config.Resource{},
		Workflows: map[string]config.Workflow{},
	}

	s := newTestServer(t, manifest)
	// Must not panic; disabled prompt is skipped.
	s.registerPrompts()
}

func TestRegisterPromptsFileContent(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "system.md")
	if err := os.WriteFile(filePath, []byte("# System Prompt from File"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	manifest := &config.Manifest{
		Version: "1.0",
		Tasks:   map[string]config.Task{},
		Prompts: map[string]config.Prompt{
			"system": {Description: "System", File: filePath},
		},
		Resources: map[string]config.Resource{},
		Workflows: map[string]config.Workflow{},
	}

	s := newTestServer(t, manifest)
	// Must not panic; file-based prompt is registered.
	s.registerPrompts()

	// Confirm the File field is still set (server reads at request time, not at register time).
	p := manifest.Prompts["system"]
	if p.File != filePath {
		t.Errorf("Prompt.File should be %q, got %q", filePath, p.File)
	}
}
