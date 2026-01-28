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
			name: "no tasks",
			manifest: &Manifest{
				Version: "1.0",
				Tasks:   map[string]Task{},
			},
			wantError: true,
			errorMsg:  "at least one task must be defined",
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
		name       string
		setupFiles map[string]string // path -> content
		customPath string
		wantError  bool
		errorMsg   string
	}{
		{
			name: "load from default location",
			setupFiles: map[string]string{
				"mcp-tasks.yaml": `version: "1.0"
tasks:
  test:
    description: "Run tests"
    command: "go test"
`,
			},
			wantError: false,
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
		},
		{
			name:      "no manifest found",
			wantError: true,
			errorMsg:  "no task manifest found",
		},
		{
			name: "invalid manifest",
			setupFiles: map[string]string{
				"mcp-tasks.yaml": `version: "1.0"
tasks:
  test:
    command: "go test"
`,
			},
			wantError: true,
			errorMsg:  "description is required",
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

			manifest, err := LoadManifest(tt.customPath)
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
			}
		})
	}
}
