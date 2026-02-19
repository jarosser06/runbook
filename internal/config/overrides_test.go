package config

import (
	"os"
	"path/filepath"
	"testing"

	"runbookmcp.dev/internal/dirs"
)

// ---------------------------------------------------------------------------
// LoadOverrides
// ---------------------------------------------------------------------------

func TestLoadOverridesFileNotExist(t *testing.T) {
	o, err := LoadOverrides(filepath.Join("/nonexistent/path", dirs.OverridesFile))
	if err != nil {
		t.Fatalf("expected nil error for missing file, got: %v", err)
	}
	if o != nil {
		t.Fatalf("expected nil overrides for missing file, got: %+v", o)
	}
}

func TestLoadOverridesEmpty(t *testing.T) {
	f := writeTempFile(t, "overrides-*.yaml", "")
	o, err := LoadOverrides(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// empty YAML → nil or zero-value struct; either is acceptable
	if o != nil && (len(o.Tasks) > 0 || len(o.Workflows) > 0 || len(o.Resources) > 0 || len(o.Prompts) > 0) {
		t.Fatalf("expected empty overrides, got: %+v", o)
	}
}

func TestLoadOverridesValidYAML(t *testing.T) {
	yaml := `
tasks:
  ts-lint:
    disable_mcp: true
  ts-build:
    disabled: true
workflows:
  ci:
    disabled: true
resources:
  guide:
    disabled: true
prompts:
  setup:
    disable_mcp: true
`
	f := writeTempFile(t, "overrides-*.yaml", yaml)
	o, err := LoadOverrides(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if o == nil {
		t.Fatal("expected non-nil overrides")
	}
	if !o.Tasks["ts-lint"].DisableMCP {
		t.Error("ts-lint: expected DisableMCP=true")
	}
	if !o.Tasks["ts-build"].Disabled {
		t.Error("ts-build: expected Disabled=true")
	}
	if !o.Workflows["ci"].Disabled {
		t.Error("ci workflow: expected Disabled=true")
	}
	if !o.Resources["guide"].Disabled {
		t.Error("guide resource: expected Disabled=true")
	}
	if !o.Prompts["setup"].DisableMCP {
		t.Error("setup prompt: expected DisableMCP=true")
	}
}

func TestLoadOverridesInvalidYAML(t *testing.T) {
	f := writeTempFile(t, "overrides-*.yaml", "tasks: [invalid: yaml: syntax")
	_, err := LoadOverrides(f)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

// ---------------------------------------------------------------------------
// ApplyOverrides — tasks
// ---------------------------------------------------------------------------

func TestApplyOverridesDisablesTask(t *testing.T) {
	manifest := minimalManifestWithTasks(map[string]Task{
		"build": {Description: "Build", Command: "go build", Type: TaskTypeOneShot},
		"test":  {Description: "Test", Command: "go test", Type: TaskTypeOneShot},
	})
	overrides := &Overrides{
		Tasks: map[string]ItemOverride{
			"build": {Disabled: true},
		},
	}
	ApplyOverrides(manifest, overrides)
	if !manifest.Tasks["build"].Disabled {
		t.Error("build: expected Disabled=true")
	}
	if manifest.Tasks["test"].Disabled {
		t.Error("test: expected Disabled=false (unaffected)")
	}
}

func TestApplyOverridesDisableMCPTask(t *testing.T) {
	manifest := minimalManifestWithTasks(map[string]Task{
		"secret-setup": {Description: "Secrets", Command: "./setup.sh", Type: TaskTypeOneShot},
	})
	overrides := &Overrides{
		Tasks: map[string]ItemOverride{
			"secret-setup": {DisableMCP: true},
		},
	}
	ApplyOverrides(manifest, overrides)
	task := manifest.Tasks["secret-setup"]
	if !task.DisableMCP {
		t.Error("secret-setup: expected DisableMCP=true")
	}
	if task.Disabled {
		t.Error("secret-setup: expected Disabled=false (only MCP disabled)")
	}
}

func TestApplyOverridesGlobPattern(t *testing.T) {
	manifest := minimalManifestWithTasks(map[string]Task{
		"ts-lint":       {Description: "Lint", Command: "eslint .", Type: TaskTypeOneShot},
		"ts-test":       {Description: "Test", Command: "jest", Type: TaskTypeOneShot},
		"ts-type-check": {Description: "Types", Command: "tsc", Type: TaskTypeOneShot},
		"go-build":      {Description: "Build", Command: "go build", Type: TaskTypeOneShot},
	})
	overrides := &Overrides{
		Tasks: map[string]ItemOverride{
			"ts-*": {DisableMCP: true},
		},
	}
	ApplyOverrides(manifest, overrides)

	for _, name := range []string{"ts-lint", "ts-test", "ts-type-check"} {
		if !manifest.Tasks[name].DisableMCP {
			t.Errorf("%s: expected DisableMCP=true from glob match", name)
		}
	}
	if manifest.Tasks["go-build"].DisableMCP {
		t.Error("go-build: should NOT be disabled by ts-* glob")
	}
}

func TestApplyOverridesGlobAndExactBothApply(t *testing.T) {
	manifest := minimalManifestWithTasks(map[string]Task{
		"ts-lint": {Description: "Lint", Command: "eslint .", Type: TaskTypeOneShot},
	})
	// Glob disables MCP; exact entry also sets Disabled.
	overrides := &Overrides{
		Tasks: map[string]ItemOverride{
			"ts-*":    {DisableMCP: true},
			"ts-lint": {Disabled: true},
		},
	}
	ApplyOverrides(manifest, overrides)
	task := manifest.Tasks["ts-lint"]
	if !task.DisableMCP {
		t.Error("ts-lint: expected DisableMCP=true from glob")
	}
	if !task.Disabled {
		t.Error("ts-lint: expected Disabled=true from exact match")
	}
}

func TestApplyOverridesPreservesExistingFlags(t *testing.T) {
	// A task that already has DisableMCP=true should keep it even if override doesn't set it.
	manifest := minimalManifestWithTasks(map[string]Task{
		"setup": {Description: "Setup", Command: "./setup.sh", Type: TaskTypeOneShot, DisableMCP: true},
	})
	overrides := &Overrides{
		Tasks: map[string]ItemOverride{
			"setup": {Disabled: true}, // only sets Disabled; should not clear DisableMCP
		},
	}
	ApplyOverrides(manifest, overrides)
	task := manifest.Tasks["setup"]
	if !task.DisableMCP {
		t.Error("setup: DisableMCP should remain true")
	}
	if !task.Disabled {
		t.Error("setup: Disabled should be set to true")
	}
}

// ---------------------------------------------------------------------------
// ApplyOverrides — workflows
// ---------------------------------------------------------------------------

func TestApplyOverridesDisablesWorkflow(t *testing.T) {
	manifest := minimalManifestWithTasks(map[string]Task{
		"build": {Description: "Build", Command: "go build", Type: TaskTypeOneShot},
	})
	manifest.Workflows = map[string]Workflow{
		"ci": {
			Description: "CI pipeline",
			Steps:       []WorkflowStep{{Task: "build"}},
		},
		"release": {
			Description: "Release",
			Steps:       []WorkflowStep{{Task: "build"}},
		},
	}
	overrides := &Overrides{
		Workflows: map[string]ItemOverride{
			"ci": {Disabled: true},
		},
	}
	ApplyOverrides(manifest, overrides)
	if !manifest.Workflows["ci"].Disabled {
		t.Error("ci: expected Disabled=true")
	}
	if manifest.Workflows["release"].Disabled {
		t.Error("release: should not be disabled")
	}
}

func TestApplyOverridesDisableMCPWorkflow(t *testing.T) {
	manifest := minimalManifestWithTasks(map[string]Task{
		"build": {Description: "Build", Command: "go build", Type: TaskTypeOneShot},
	})
	manifest.Workflows = map[string]Workflow{
		"ci": {Description: "CI", Steps: []WorkflowStep{{Task: "build"}}},
	}
	overrides := &Overrides{
		Workflows: map[string]ItemOverride{
			"ci": {DisableMCP: true},
		},
	}
	ApplyOverrides(manifest, overrides)
	wf := manifest.Workflows["ci"]
	if !wf.DisableMCP {
		t.Error("ci: expected DisableMCP=true")
	}
	if wf.Disabled {
		t.Error("ci: should not be fully disabled")
	}
}

// ---------------------------------------------------------------------------
// ApplyOverrides — resources and prompts
// ---------------------------------------------------------------------------

func TestApplyOverridesDisablesResource(t *testing.T) {
	manifest := minimalManifestWithTasks(nil)
	manifest.Resources = map[string]Resource{
		"guide":  {Description: "Guide", Content: "# Guide"},
		"readme": {Description: "Readme", Content: "# Readme"},
	}
	overrides := &Overrides{
		Resources: map[string]ItemOverride{
			"guide": {Disabled: true},
		},
	}
	ApplyOverrides(manifest, overrides)
	if !manifest.Resources["guide"].Disabled {
		t.Error("guide: expected Disabled=true")
	}
	if manifest.Resources["readme"].Disabled {
		t.Error("readme: should not be disabled")
	}
}

func TestApplyOverridesDisableMCPResourceActsAsDisabled(t *testing.T) {
	// Resources are MCP-only; DisableMCP should set Disabled.
	manifest := minimalManifestWithTasks(nil)
	manifest.Resources = map[string]Resource{
		"docs": {Description: "Docs", Content: "# Docs"},
	}
	overrides := &Overrides{
		Resources: map[string]ItemOverride{
			"docs": {DisableMCP: true},
		},
	}
	ApplyOverrides(manifest, overrides)
	if !manifest.Resources["docs"].Disabled {
		t.Error("docs: DisableMCP on resource should set Disabled=true")
	}
}

func TestApplyOverridesDisablesPrompt(t *testing.T) {
	manifest := minimalManifestWithTasks(nil)
	manifest.Prompts = map[string]Prompt{
		"onboarding": {Description: "Onboarding", Content: "Welcome"},
		"review":     {Description: "Review", Content: "Review guide"},
	}
	overrides := &Overrides{
		Prompts: map[string]ItemOverride{
			"onboarding": {Disabled: true},
		},
	}
	ApplyOverrides(manifest, overrides)
	if !manifest.Prompts["onboarding"].Disabled {
		t.Error("onboarding: expected Disabled=true")
	}
	if manifest.Prompts["review"].Disabled {
		t.Error("review: should not be disabled")
	}
}

func TestApplyOverridesGlobOnPrompts(t *testing.T) {
	manifest := minimalManifestWithTasks(nil)
	manifest.Prompts = map[string]Prompt{
		"ts-setup":  {Description: "TS setup", Content: "..."},
		"ts-review": {Description: "TS review", Content: "..."},
		"go-setup":  {Description: "Go setup", Content: "..."},
	}
	overrides := &Overrides{
		Prompts: map[string]ItemOverride{
			"ts-*": {Disabled: true},
		},
	}
	ApplyOverrides(manifest, overrides)
	if !manifest.Prompts["ts-setup"].Disabled {
		t.Error("ts-setup: expected Disabled=true from glob")
	}
	if !manifest.Prompts["ts-review"].Disabled {
		t.Error("ts-review: expected Disabled=true from glob")
	}
	if manifest.Prompts["go-setup"].Disabled {
		t.Error("go-setup: should not be disabled")
	}
}

// ---------------------------------------------------------------------------
// Integration: loader applies overrides
// ---------------------------------------------------------------------------

func TestLoaderAppliesOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	origDir := mustGetwd(t)
	t.Cleanup(func() { mustChdir(t, origDir) })
	mustChdir(t, tmpDir)

	// Write main config.
	if err := os.MkdirAll(dirs.ConfigDir, 0755); err != nil {
		t.Fatal(err)
	}
	mainYAML := `version: "1.0"
tasks:
  build:
    description: "Build"
    command: "go build"
    type: oneshot
  ts-lint:
    description: "Lint"
    command: "eslint ."
    type: oneshot
  ts-test:
    description: "Test"
    command: "jest"
    type: oneshot
`
	if err := os.WriteFile(dirs.ConfigDir + "/tasks.yaml", []byte(mainYAML), 0644); err != nil {
		t.Fatal(err)
	}

	// Write overrides file.
	overridesYAML := `
tasks:
  ts-*:
    disable_mcp: true
  ts-lint:
    disabled: true
`
	if err := os.WriteFile(dirs.OverridesFile, []byte(overridesYAML), 0644); err != nil {
		t.Fatal(err)
	}

	manifest, loaded, err := LoadManifest("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !loaded {
		t.Fatal("expected loaded=true")
	}

	// ts-lint: both disabled and disable_mcp should be set.
	lint := manifest.Tasks["ts-lint"]
	if !lint.Disabled {
		t.Error("ts-lint: expected Disabled=true")
	}
	if !lint.DisableMCP {
		t.Error("ts-lint: expected DisableMCP=true (from glob)")
	}

	// ts-test: only DisableMCP (from glob).
	tsTest := manifest.Tasks["ts-test"]
	if tsTest.Disabled {
		t.Error("ts-test: should not be fully disabled")
	}
	if !tsTest.DisableMCP {
		t.Error("ts-test: expected DisableMCP=true (from glob)")
	}

	// build: unaffected.
	build := manifest.Tasks["build"]
	if build.Disabled || build.DisableMCP {
		t.Error("build: should not be disabled")
	}
}

func TestLoaderOverridesMissingIsOK(t *testing.T) {
	tmpDir := t.TempDir()
	origDir := mustGetwd(t)
	t.Cleanup(func() { mustChdir(t, origDir) })
	mustChdir(t, tmpDir)

	if err := os.MkdirAll(dirs.ConfigDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dirs.ConfigDir + "/tasks.yaml", []byte(`version: "1.0"
tasks:
  build:
    description: "Build"
    command: "go build"
    type: oneshot
`), 0644); err != nil {
		t.Fatal(err)
	}
	// No overrides file — should load without error.
	manifest, loaded, err := LoadManifest("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !loaded {
		t.Fatal("expected loaded=true")
	}
	if manifest.Tasks["build"].Disabled {
		t.Error("build should not be disabled")
	}
}

// ---------------------------------------------------------------------------
// Prompt validation — file field
// ---------------------------------------------------------------------------

func TestValidatePromptFileOrContentRequired(t *testing.T) {
	// Neither content nor file: error.
	err := validatePrompt("p", Prompt{Description: "desc"})
	if err == nil {
		t.Error("expected error when neither content nor file set")
	}
}

func TestValidatePromptContentAndFileMutuallyExclusive(t *testing.T) {
	err := validatePrompt("p", Prompt{Description: "desc", Content: "hi", File: "f.md"})
	if err == nil {
		t.Error("expected error when both content and file set")
	}
}

func TestValidatePromptFileOnly(t *testing.T) {
	err := validatePrompt("p", Prompt{Description: "desc", File: "prompts/guide.md"})
	if err != nil {
		t.Errorf("unexpected error with file-only prompt: %v", err)
	}
}

func TestValidatePromptContentOnly(t *testing.T) {
	err := validatePrompt("p", Prompt{Description: "desc", Content: "hello"})
	if err != nil {
		t.Errorf("unexpected error with content-only prompt: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func minimalManifestWithTasks(tasks map[string]Task) *Manifest {
	if tasks == nil {
		tasks = make(map[string]Task)
	}
	return &Manifest{
		Version:   "1.0",
		Tasks:     tasks,
		Workflows: make(map[string]Workflow),
		Resources: make(map[string]Resource),
		Prompts:   make(map[string]Prompt),
	}
}

func writeTempFile(t *testing.T, pattern, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), pattern)
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	f.Close()
	return f.Name()
}

func mustGetwd(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return wd
}

func mustChdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
}

func TestLoaderOverridesInvalidYAMLIsFatal(t *testing.T) {
	tmpDir := t.TempDir()
	origDir := mustGetwd(t)
	t.Cleanup(func() { mustChdir(t, origDir) })
	mustChdir(t, tmpDir)

	if err := os.MkdirAll(dirs.ConfigDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dirs.ConfigDir + "/tasks.yaml", []byte(`version: "1.0"
tasks:
  build:
    description: "Build"
    command: "go build"
    type: oneshot
`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dirs.OverridesFile, []byte("tasks: [bad yaml: !!"), 0644); err != nil {
		t.Fatal(err)
	}
	_, _, err := LoadManifest("")
	if err == nil {
		t.Error("expected error for invalid overrides YAML, got nil")
	}
}

// TestApplyOverridesNoopOnEmpty confirms ApplyOverrides with empty overrides
// does not mutate the manifest.
func TestApplyOverridesNoopOnEmpty(t *testing.T) {
	manifest := minimalManifestWithTasks(map[string]Task{
		"build": {Description: "Build", Command: "go build", Type: TaskTypeOneShot},
	})
	ApplyOverrides(manifest, &Overrides{})
	if manifest.Tasks["build"].Disabled || manifest.Tasks["build"].DisableMCP {
		t.Error("empty overrides should not modify tasks")
	}
}

// TestApplyOverridesResourceFilePathIsPreserved verifies we're not
// accidentally clobbering struct fields we don't touch.
func TestApplyOverridesResourceFileIsPreserved(t *testing.T) {
	manifest := minimalManifestWithTasks(nil)
	manifest.Resources = map[string]Resource{
		"docs": {Description: "Docs", File: "./docs/guide.md"},
	}
	overrides := &Overrides{
		Resources: map[string]ItemOverride{
			"docs": {Disabled: true},
		},
	}
	ApplyOverrides(manifest, overrides)
	r := manifest.Resources["docs"]
	if !r.Disabled {
		t.Error("docs: expected Disabled=true")
	}
	if r.File != "./docs/guide.md" {
		t.Errorf("docs: File should be preserved, got %q", r.File)
	}
}

// ---------------------------------------------------------------------------
// matchesPattern
// ---------------------------------------------------------------------------

func TestMatchesPattern(t *testing.T) {
	tests := []struct {
		pattern string
		name    string
		want    bool
	}{
		{"ts-lint", "ts-lint", true},
		{"ts-lint", "ts-test", false},
		{"ts-*", "ts-lint", true},
		{"ts-*", "ts-test", true},
		{"ts-*", "go-build", false},
		{"*", "anything", true},
		{"ts-[lt]*", "ts-lint", true},
		{"ts-[lt]*", "ts-test", true},
		{"ts-[lt]*", "ts-type-check", true},  // [lt] matches 't' in type-check
		{"ts-lint", "ts-type-check", false},   // exact match doesn't match
		// invalid pattern returns false, not panic
		{"[invalid", "ts-lint", false},
	}
	for _, tt := range tests {
		got := matchesPattern(tt.pattern, tt.name)
		if got != tt.want {
			t.Errorf("matchesPattern(%q, %q) = %v, want %v", tt.pattern, tt.name, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Server-level: collectToolNames respects Disabled
// ---------------------------------------------------------------------------

// This is tested in the server package; here we confirm types are correct.
func TestDisabledFieldOnTask(t *testing.T) {
	task := Task{Disabled: true, DisableMCP: false}
	if !task.Disabled {
		t.Error("Disabled should be true")
	}
	if task.DisableMCP {
		t.Error("DisableMCP should be false")
	}
}

func TestDisabledFieldOnWorkflow(t *testing.T) {
	wf := Workflow{Disabled: true, DisableMCP: true}
	if !wf.Disabled || !wf.DisableMCP {
		t.Error("Both flags should be true")
	}
}

// ---------------------------------------------------------------------------
// Resource/Prompt disabled type field presence
// ---------------------------------------------------------------------------

func TestResourceDisabledField(t *testing.T) {
	r := Resource{Disabled: true}
	if !r.Disabled {
		t.Error("Resource.Disabled should be settable")
	}
}

func TestPromptDisabledAndFileField(t *testing.T) {
	p := Prompt{Disabled: true, File: "guide.md"}
	if !p.Disabled {
		t.Error("Prompt.Disabled should be settable")
	}
	if p.File != "guide.md" {
		t.Errorf("Prompt.File = %q, want 'guide.md'", p.File)
	}
}

// filepath.Join used to keep import from being unused in this file.
var _ = filepath.Join
