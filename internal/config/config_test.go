package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseManifest(t *testing.T) {
	tests := []struct {
		name      string
		yaml      string
		wantError bool
		validate  func(*testing.T, *Manifest)
	}{
		{
			name: "valid minimal manifest",
			yaml: `version: "1.0"
tasks:
  test:
    description: "Run tests"
    command: "go test ./..."
    type: oneshot
`,
			wantError: false,
			validate: func(t *testing.T, m *Manifest) {
				if m.Version != "1.0" {
					t.Errorf("expected version 1.0, got %s", m.Version)
				}
				if len(m.Tasks) != 1 {
					t.Errorf("expected 1 task, got %d", len(m.Tasks))
				}
				task := m.Tasks["test"]
				if task.Description != "Run tests" {
					t.Errorf("expected description 'Run tests', got %s", task.Description)
				}
			},
		},
		{
			name: "manifest with defaults",
			yaml: `version: "1.0"
defaults:
  timeout: 300
  shell: "/bin/bash"
  env:
    NODE_ENV: "development"
tasks:
  test:
    description: "Run tests"
    command: "npm test"
    type: oneshot
`,
			wantError: false,
			validate: func(t *testing.T, m *Manifest) {
				task := m.Tasks["test"]
				if task.Timeout != 300 {
					t.Errorf("expected timeout 300, got %d", task.Timeout)
				}
				if task.Shell != "/bin/bash" {
					t.Errorf("expected shell /bin/bash, got %s", task.Shell)
				}
				if task.Env["NODE_ENV"] != "development" {
					t.Errorf("expected NODE_ENV=development, got %s", task.Env["NODE_ENV"])
				}
			},
		},
		{
			name: "task overrides defaults",
			yaml: `version: "1.0"
defaults:
  timeout: 300
  shell: "/bin/bash"
  env:
    NODE_ENV: "development"
tasks:
  test:
    description: "Run tests"
    command: "npm test"
    type: oneshot
    timeout: 600
    shell: "/bin/zsh"
    env:
      NODE_ENV: "production"
`,
			wantError: false,
			validate: func(t *testing.T, m *Manifest) {
				task := m.Tasks["test"]
				if task.Timeout != 600 {
					t.Errorf("expected timeout 600, got %d", task.Timeout)
				}
				if task.Shell != "/bin/zsh" {
					t.Errorf("expected shell /bin/zsh, got %s", task.Shell)
				}
				if task.Env["NODE_ENV"] != "production" {
					t.Errorf("expected NODE_ENV=production, got %s", task.Env["NODE_ENV"])
				}
			},
		},
		{
			name:      "invalid yaml",
			yaml:      `this is not: valid: yaml:`,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, "test-manifest.yaml")
			if err := os.WriteFile(tmpFile, []byte(tt.yaml), 0644); err != nil {
				t.Fatalf("failed to create temp file: %v", err)
			}

			manifest, err := ParseManifest(tmpFile)
			if tt.wantError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.validate != nil {
				tt.validate(t, manifest)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name      string
		manifest  *Manifest
		wantError bool
		errorMsg  string
	}{
		{
			name: "valid manifest",
			manifest: &Manifest{
				Version: "1.0",
				Tasks: map[string]Task{
					"test": {
						Description: "Run tests",
						Command:     "go test",
						Type:        TaskTypeOneShot,
					},
				},
			},
			wantError: false,
		},
		{
			name: "missing version",
			manifest: &Manifest{
				Tasks: map[string]Task{
					"test": {
						Description: "Run tests",
						Command:     "go test",
					},
				},
			},
			wantError: true,
			errorMsg:  "version is required",
		},
		{
			name: "empty tasks map is valid",
			manifest: &Manifest{
				Version: "1.0",
				Tasks:   map[string]Task{},
			},
			wantError: false,
		},
		{
			name: "nil tasks map is invalid",
			manifest: &Manifest{
				Version: "1.0",
				Tasks:   nil,
			},
			wantError: true,
			errorMsg:  "tasks map must be initialized",
		},
		{
			name: "missing task description",
			manifest: &Manifest{
				Version: "1.0",
				Tasks: map[string]Task{
					"test": {
						Command: "go test",
					},
				},
			},
			wantError: true,
			errorMsg:  "description is required",
		},
		{
			name: "missing task command",
			manifest: &Manifest{
				Version: "1.0",
				Tasks: map[string]Task{
					"test": {
						Description: "Run tests",
					},
				},
			},
			wantError: true,
			errorMsg:  "command is required",
		},
		{
			name: "invalid task type",
			manifest: &Manifest{
				Version: "1.0",
				Tasks: map[string]Task{
					"test": {
						Description: "Run tests",
						Command:     "go test",
						Type:        "invalid",
					},
				},
			},
			wantError: true,
			errorMsg:  "invalid type",
		},
		{
			name: "parameter without type",
			manifest: &Manifest{
				Version: "1.0",
				Tasks: map[string]Task{
					"test": {
						Description: "Run tests",
						Command:     "go test {{.file}}",
						Parameters: map[string]Param{
							"file": {
								Description: "File to test",
							},
						},
					},
				},
			},
			wantError: true,
			errorMsg:  "must specify a type",
		},
		{
			name: "invalid dependency",
			manifest: &Manifest{
				Version: "1.0",
				Tasks: map[string]Task{
					"test": {
						Description: "Run tests",
						Command:     "go test",
						DependsOn:   []string{"nonexistent"},
					},
				},
			},
			wantError: true,
			errorMsg:  "dependency 'nonexistent' does not exist",
		},
		{
			name: "task group with invalid task reference",
			manifest: &Manifest{
				Version: "1.0",
				Tasks: map[string]Task{
					"test": {
						Description: "Run tests",
						Command:     "go test",
					},
				},
				TaskGroups: map[string]TaskGroup{
					"ci": {
						Description: "CI tasks",
						Tasks:       []string{"test", "nonexistent"},
					},
				},
			},
			wantError: true,
			errorMsg:  "task 'nonexistent' does not exist",
		},
		{
			name: "prompt without description",
			manifest: &Manifest{
				Version: "1.0",
				Tasks: map[string]Task{
					"test": {
						Description: "Run tests",
						Command:     "go test",
					},
				},
				Prompts: map[string]Prompt{
					"dev": {
						Content: "Start development",
					},
				},
			},
			wantError: true,
			errorMsg:  "description is required",
		},
		{
			name: "resource without description",
			manifest: &Manifest{
				Version: "1.0",
				Tasks: map[string]Task{
					"test": {
						Description: "Run tests",
						Command:     "go test",
					},
				},
				Resources: map[string]Resource{
					"docs": {
						Content: "some content",
					},
				},
			},
			wantError: true,
			errorMsg:  "resource 'docs': description is required",
		},
		{
			name: "resource with neither content nor file",
			manifest: &Manifest{
				Version: "1.0",
				Tasks: map[string]Task{
					"test": {
						Description: "Run tests",
						Command:     "go test",
					},
				},
				Resources: map[string]Resource{
					"docs": {
						Description: "Some docs",
					},
				},
			},
			wantError: true,
			errorMsg:  "either content or file is required",
		},
		{
			name: "resource with both content and file",
			manifest: &Manifest{
				Version: "1.0",
				Tasks: map[string]Task{
					"test": {
						Description: "Run tests",
						Command:     "go test",
					},
				},
				Resources: map[string]Resource{
					"docs": {
						Description: "Some docs",
						Content:     "inline content",
						File:        "./docs.md",
					},
				},
			},
			wantError: true,
			errorMsg:  "content and file are mutually exclusive",
		},
		{
			name: "valid resource with content",
			manifest: &Manifest{
				Version: "1.0",
				Tasks: map[string]Task{
					"test": {
						Description: "Run tests",
						Command:     "go test",
					},
				},
				Resources: map[string]Resource{
					"docs": {
						Description: "API docs",
						Content:     "## Endpoints",
					},
				},
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.manifest)
			if tt.wantError {
				if err == nil {
					t.Errorf("expected error, got nil")
					return
				}
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing '%s', got: %v", tt.errorMsg, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestLoadManifest(t *testing.T) {
	tests := []struct {
		name         string
		setupFiles   map[string]string // path -> content
		customPath   string
		wantError    bool
		wantLoaded   bool
		errorMsg     string
		validateFunc func(*testing.T, *Manifest)
	}{
		{
			name: "load from default location",
			setupFiles: map[string]string{
				".dev_workflow.yaml": `version: "1.0"
tasks:
  test:
    description: "Run tests"
    command: "go test"
`,
			},
			wantError:  false,
			wantLoaded: true,
		},
		{
			name: "load from custom path",
			setupFiles: map[string]string{
				"custom/tasks.yaml": `version: "1.0"
tasks:
  test:
    description: "Run tests"
    command: "go test"
`,
			},
			customPath: "custom/tasks.yaml",
			wantError:  false,
			wantLoaded: true,
		},
		{
			name:       "no manifest found - returns empty config",
			wantError:  false,
			wantLoaded: false,
			validateFunc: func(t *testing.T, m *Manifest) {
				if m == nil {
					t.Error("expected non-nil manifest")
					return
				}
				if m.Version != "1.0" {
					t.Errorf("expected version 1.0, got %s", m.Version)
				}
				if m.Tasks == nil {
					t.Error("expected non-nil tasks map")
				}
				if len(m.Tasks) != 0 {
					t.Errorf("expected empty tasks map, got %d tasks", len(m.Tasks))
				}
			},
		},
		{
			name: "invalid manifest",
			setupFiles: map[string]string{
				".dev_workflow.yaml": `version: "1.0"
tasks:
  test:
    command: "go test"
`,
			},
			wantError:  true,
			wantLoaded: false,
			errorMsg:   "description is required",
		},
		{
			name: "load from .dev_workflow directory",
			setupFiles: map[string]string{
				".dev_workflow/tasks.yaml": `version: "1.0"
tasks:
  test:
    description: "Run tests"
    command: "go test"
`,
				".dev_workflow/build.yaml": `version: "1.0"
tasks:
  build:
    description: "Build project"
    command: "go build"
`,
			},
			wantError:  false,
			wantLoaded: true,
			validateFunc: func(t *testing.T, m *Manifest) {
				if len(m.Tasks) != 2 {
					t.Errorf("expected 2 tasks, got %d", len(m.Tasks))
				}
				if _, ok := m.Tasks["test"]; !ok {
					t.Error("expected 'test' task to exist")
				}
				if _, ok := m.Tasks["build"]; !ok {
					t.Error("expected 'build' task to exist")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory
			tmpDir := t.TempDir()
			oldWd, err := os.Getwd()
			if err != nil {
				t.Fatalf("failed to get working directory: %v", err)
			}
			defer func() {
				if err := os.Chdir(oldWd); err != nil {
					t.Errorf("failed to restore working directory: %v", err)
				}
			}()

			if err := os.Chdir(tmpDir); err != nil {
				t.Fatalf("failed to change directory: %v", err)
			}

			// Setup files
			for path, content := range tt.setupFiles {
				dir := filepath.Dir(path)
				if dir != "." {
					if err := os.MkdirAll(dir, 0755); err != nil {
						t.Fatalf("failed to create directory %s: %v", dir, err)
					}
				}
				if err := os.WriteFile(path, []byte(content), 0644); err != nil {
					t.Fatalf("failed to create file %s: %v", path, err)
				}
			}

			manifest, loaded, err := LoadManifest(tt.customPath)
			if tt.wantError {
				if err == nil {
					t.Errorf("expected error, got nil")
					return
				}
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing '%s', got: %v", tt.errorMsg, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if manifest == nil {
				t.Errorf("expected manifest, got nil")
				return
			}

			if loaded != tt.wantLoaded {
				t.Errorf("expected loaded=%v, got %v", tt.wantLoaded, loaded)
			}

			if tt.validateFunc != nil {
				tt.validateFunc(t, manifest)
			}
		})
	}
}

func TestParseManifestWithImports(t *testing.T) {
	tests := []struct {
		name      string
		files     map[string]string // filename -> content
		mainFile  string
		wantError bool
		errorMsg  string
		validate  func(*testing.T, *Manifest)
	}{
		{
			name:     "backward compatibility - no imports",
			mainFile: "main.yaml",
			files: map[string]string{
				"main.yaml": `version: "1.0"
tasks:
  test:
    description: "Run tests"
    command: "go test"
`,
			},
			wantError: false,
			validate: func(t *testing.T, m *Manifest) {
				if len(m.Tasks) != 1 {
					t.Errorf("expected 1 task, got %d", len(m.Tasks))
				}
				if _, ok := m.Tasks["test"]; !ok {
					t.Error("expected 'test' task to exist")
				}
			},
		},
		{
			name:     "simple import",
			mainFile: "main.yaml",
			files: map[string]string{
				"main.yaml": `version: "1.0"
imports:
  - "./tasks.yaml"
defaults:
  timeout: 300
tasks:
  test:
    description: "Run tests"
    command: "go test"
`,
				"tasks.yaml": `version: "1.0"
tasks:
  build:
    description: "Build project"
    command: "go build"
`,
			},
			wantError: false,
			validate: func(t *testing.T, m *Manifest) {
				if len(m.Tasks) != 2 {
					t.Errorf("expected 2 tasks, got %d", len(m.Tasks))
				}
				if _, ok := m.Tasks["test"]; !ok {
					t.Error("expected 'test' task to exist")
				}
				if _, ok := m.Tasks["build"]; !ok {
					t.Error("expected 'build' task to exist")
				}
				// Check defaults are preserved
				if m.Defaults.Timeout != 300 {
					t.Errorf("expected default timeout 300, got %d", m.Defaults.Timeout)
				}
			},
		},
		{
			name:     "glob pattern import",
			mainFile: "main.yaml",
			files: map[string]string{
				"main.yaml": `version: "1.0"
imports:
  - "./tasks/*.yaml"
tasks:
  main_task:
    description: "Main task"
    command: "echo main"
`,
				"tasks/build.yaml": `version: "1.0"
tasks:
  build:
    description: "Build project"
    command: "go build"
`,
				"tasks/test.yaml": `version: "1.0"
tasks:
  test:
    description: "Run tests"
    command: "go test"
`,
			},
			wantError: false,
			validate: func(t *testing.T, m *Manifest) {
				if len(m.Tasks) != 3 {
					t.Errorf("expected 3 tasks, got %d", len(m.Tasks))
				}
				if _, ok := m.Tasks["main_task"]; !ok {
					t.Error("expected 'main_task' to exist")
				}
				if _, ok := m.Tasks["build"]; !ok {
					t.Error("expected 'build' task to exist")
				}
				if _, ok := m.Tasks["test"]; !ok {
					t.Error("expected 'test' task to exist")
				}
			},
		},
		{
			name:     "nested imports",
			mainFile: "main.yaml",
			files: map[string]string{
				"main.yaml": `version: "1.0"
imports:
  - "./level1.yaml"
tasks:
  main:
    description: "Main task"
    command: "echo main"
`,
				"level1.yaml": `version: "1.0"
imports:
  - "./level2.yaml"
tasks:
  level1:
    description: "Level 1 task"
    command: "echo level1"
`,
				"level2.yaml": `version: "1.0"
tasks:
  level2:
    description: "Level 2 task"
    command: "echo level2"
`,
			},
			wantError: false,
			validate: func(t *testing.T, m *Manifest) {
				if len(m.Tasks) != 3 {
					t.Errorf("expected 3 tasks, got %d", len(m.Tasks))
				}
				if _, ok := m.Tasks["main"]; !ok {
					t.Error("expected 'main' task to exist")
				}
				if _, ok := m.Tasks["level1"]; !ok {
					t.Error("expected 'level1' task to exist")
				}
				if _, ok := m.Tasks["level2"]; !ok {
					t.Error("expected 'level2' task to exist")
				}
			},
		},
		{
			name:     "duplicate task names",
			mainFile: "main.yaml",
			files: map[string]string{
				"main.yaml": `version: "1.0"
imports:
  - "./tasks.yaml"
tasks:
  test:
    description: "Main test"
    command: "go test"
`,
				"tasks.yaml": `version: "1.0"
tasks:
  test:
    description: "Imported test"
    command: "npm test"
`,
			},
			wantError: true,
			errorMsg:  "duplicate task name 'test'",
		},
		{
			name:     "circular dependency",
			mainFile: "a.yaml",
			files: map[string]string{
				"a.yaml": `version: "1.0"
imports:
  - "./b.yaml"
tasks:
  a:
    description: "Task A"
    command: "echo a"
`,
				"b.yaml": `version: "1.0"
imports:
  - "./a.yaml"
tasks:
  b:
    description: "Task B"
    command: "echo b"
`,
			},
			wantError: true,
			errorMsg:  "circular import detected",
		},
		{
			name:     "import file not found",
			mainFile: "main.yaml",
			files: map[string]string{
				"main.yaml": `version: "1.0"
imports:
  - "./nonexistent.yaml"
tasks:
  test:
    description: "Test task"
    command: "echo test"
`,
			},
			wantError: true,
			errorMsg:  "matched no files",
		},
		{
			name:     "merge task groups",
			mainFile: "main.yaml",
			files: map[string]string{
				"main.yaml": `version: "1.0"
imports:
  - "./groups.yaml"
tasks:
  test:
    description: "Run tests"
    command: "go test"
task_groups:
  main_group:
    description: "Main group"
    tasks:
      - test
`,
				"groups.yaml": `version: "1.0"
tasks:
  build:
    description: "Build project"
    command: "go build"
task_groups:
  ci:
    description: "CI pipeline"
    tasks:
      - test
      - build
`,
			},
			wantError: false,
			validate: func(t *testing.T, m *Manifest) {
				if len(m.TaskGroups) != 2 {
					t.Errorf("expected 2 task groups, got %d", len(m.TaskGroups))
				}
				if _, ok := m.TaskGroups["main_group"]; !ok {
					t.Error("expected 'main_group' to exist")
				}
				if _, ok := m.TaskGroups["ci"]; !ok {
					t.Error("expected 'ci' group to exist")
				}
			},
		},
		{
			name:     "merge resources",
			mainFile: "main.yaml",
			files: map[string]string{
				"main.yaml": `version: "1.0"
imports:
  - "./resources.yaml"
tasks:
  test:
    description: "Run tests"
    command: "go test"
resources:
  api_docs:
    description: "API docs"
    content: "## Endpoints"
`,
				"resources.yaml": `version: "1.0"
resources:
  architecture:
    description: "System architecture"
    content: "## Architecture overview"
`,
			},
			wantError: false,
			validate: func(t *testing.T, m *Manifest) {
				if len(m.Resources) != 2 {
					t.Errorf("expected 2 resources, got %d", len(m.Resources))
				}
				if _, ok := m.Resources["api_docs"]; !ok {
					t.Error("expected 'api_docs' resource to exist")
				}
				if _, ok := m.Resources["architecture"]; !ok {
					t.Error("expected 'architecture' resource to exist")
				}
			},
		},
		{
			name:     "duplicate resource names",
			mainFile: "main.yaml",
			files: map[string]string{
				"main.yaml": `version: "1.0"
imports:
  - "./resources.yaml"
tasks:
  test:
    description: "Run tests"
    command: "go test"
resources:
  docs:
    description: "Docs"
    content: "main docs"
`,
				"resources.yaml": `version: "1.0"
resources:
  docs:
    description: "Docs"
    content: "imported docs"
`,
			},
			wantError: true,
			errorMsg:  "duplicate resource name 'docs'",
		},
		{
			name:     "resource with file reference",
			mainFile: "main.yaml",
			files: map[string]string{
				"main.yaml": `version: "1.0"
tasks:
  test:
    description: "Run tests"
    command: "go test"
resources:
  architecture:
    description: "System architecture"
    file: "./docs/architecture.md"
`,
				"docs/architecture.md": "# Architecture\n\nThis is the architecture doc.\n",
			},
			wantError: false,
			validate: func(t *testing.T, m *Manifest) {
				r, ok := m.Resources["architecture"]
				if !ok {
					t.Fatal("expected 'architecture' resource to exist")
				}
				if r.File != "" {
					t.Errorf("expected File to be cleared after resolution, got %s", r.File)
				}
				expected := "# Architecture\n\nThis is the architecture doc.\n"
				if r.Content != expected {
					t.Errorf("expected content %q, got %q", expected, r.Content)
				}
			},
		},
		{
			name:     "resource with missing file",
			mainFile: "main.yaml",
			files: map[string]string{
				"main.yaml": `version: "1.0"
tasks:
  test:
    description: "Run tests"
    command: "go test"
resources:
  docs:
    description: "Docs"
    file: "./nonexistent.md"
`,
			},
			wantError: true,
			errorMsg:  "failed to read file",
		},
		{
			name:     "merge prompts",
			mainFile: "main.yaml",
			files: map[string]string{
				"main.yaml": `version: "1.0"
imports:
  - "./prompts.yaml"
tasks:
  test:
    description: "Run tests"
    command: "go test"
prompts:
  main_prompt:
    name: "Main"
    description: "Main prompt"
    content: "This is main"
`,
				"prompts.yaml": `version: "1.0"
prompts:
  dev:
    name: "Dev"
    description: "Development prompt"
    content: "Start development"
`,
			},
			wantError: false,
			validate: func(t *testing.T, m *Manifest) {
				if len(m.Prompts) != 2 {
					t.Errorf("expected 2 prompts, got %d", len(m.Prompts))
				}
				if _, ok := m.Prompts["main_prompt"]; !ok {
					t.Error("expected 'main_prompt' to exist")
				}
				if _, ok := m.Prompts["dev"]; !ok {
					t.Error("expected 'dev' prompt to exist")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory
			tmpDir := t.TempDir()

			// Create all test files
			for filename, content := range tt.files {
				path := filepath.Join(tmpDir, filename)
				dir := filepath.Dir(path)
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatalf("failed to create directory %s: %v", dir, err)
				}
				if err := os.WriteFile(path, []byte(content), 0644); err != nil {
					t.Fatalf("failed to write file %s: %v", path, err)
				}
			}

			// Parse the main manifest
			mainPath := filepath.Join(tmpDir, tt.mainFile)
			manifest, err := ParseManifest(mainPath)

			if tt.wantError {
				if err == nil {
					t.Errorf("expected error, got nil")
					return
				}
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing '%s', got: %v", tt.errorMsg, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.validate != nil {
				tt.validate(t, manifest)
			}
		})
	}
}

func TestMergeManifests(t *testing.T) {
	tests := []struct {
		name      string
		base      *Manifest
		imports   []*Manifest
		wantError bool
		errorMsg  string
		validate  func(*testing.T, *Manifest)
	}{
		{
			name: "merge simple tasks",
			base: &Manifest{
				Version: "1.0",
				Tasks: map[string]Task{
					"test": {
						Description: "Run tests",
						Command:     "go test",
					},
				},
			},
			imports: []*Manifest{
				{
					Tasks: map[string]Task{
						"build": {
							Description: "Build project",
							Command:     "go build",
						},
					},
				},
			},
			wantError: false,
			validate: func(t *testing.T, m *Manifest) {
				if len(m.Tasks) != 2 {
					t.Errorf("expected 2 tasks, got %d", len(m.Tasks))
				}
			},
		},
		{
			name: "duplicate task error",
			base: &Manifest{
				Version: "1.0",
				Tasks: map[string]Task{
					"test": {
						Description: "Run tests",
						Command:     "go test",
					},
				},
			},
			imports: []*Manifest{
				{
					Tasks: map[string]Task{
						"test": {
							Description: "Another test",
							Command:     "npm test",
						},
					},
				},
			},
			wantError: true,
			errorMsg:  "duplicate task name 'test'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := mergeManifests(tt.base, tt.imports)

			if tt.wantError {
				if err == nil {
					t.Errorf("expected error, got nil")
					return
				}
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing '%s', got: %v", tt.errorMsg, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}
