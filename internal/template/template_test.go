package template

import (
	"strings"
	"testing"

	"github.com/jarosser06/dev-toolkit-mcp/internal/config"
)

func TestResolvePromptTemplate(t *testing.T) {
	tasks := map[string]config.Task{
		"test": {
			Description: "Run tests",
			Command:     "go test",
			Type:        config.TaskTypeOneShot,
		},
		"dev": {
			Description: "Start dev server",
			Command:     "npm run dev",
			Type:        config.TaskTypeDaemon,
		},
	}

	tests := []struct {
		name      string
		template  string
		want      string
		wantError bool
		errorMsg  string
	}{
		{
			name:     "task run",
			template: "Run tests: {{.Tasks.test.Run}}",
			want:     "Run tests: run_test",
		},
		{
			name:     "task start",
			template: "Start dev: {{.Tasks.dev.Start}}",
			want:     "Start dev: start_dev",
		},
		{
			name:     "task stop",
			template: "Stop dev: {{.Tasks.dev.Stop}}",
			want:     "Stop dev: stop_dev",
		},
		{
			name:     "task status",
			template: "Check status: {{.Tasks.dev.Status}}",
			want:     "Check status: status_dev",
		},
		{
			name:     "task logs",
			template: "View logs: {{.Tasks.dev.Logs}}",
			want:     "View logs: logs_dev",
		},
		{
			name:     "task description",
			template: "Description: {{.Tasks.test.Desc}}",
			want:     "Description: Run tests",
		},
		{
			name:     "multiple tasks",
			template: "Use {{.Tasks.test.Run}} or {{.Tasks.dev.Start}}",
			want:     "Use run_test or start_dev",
		},
		{
			name:     "with whitespace",
			template: "Start: {{ .Tasks.dev.Start }}",
			want:     "Start: start_dev",
		},
		{
			name:     "nonexistent task returns no value",
			template: "{{.Tasks.nonexistent.Run}}",
			want:     "<no value>",
		},
		{
			name:      "invalid syntax",
			template:  "{{.Tasks.test.Run",
			wantError: true,
			errorMsg:  "parse template",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ResolvePromptTemplate(tt.template, tasks)
			if tt.wantError {
				if err == nil {
					t.Errorf("expected error, got nil")
					return
				}
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got: %v", tt.errorMsg, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result != tt.want {
				t.Errorf("expected %q, got %q", tt.want, result)
			}
		})
	}
}

func TestSubstituteParameters(t *testing.T) {
	tests := []struct {
		name      string
		command   string
		params    map[string]interface{}
		want      string
		wantError bool
		errorMsg  string
	}{
		{
			name:    "single parameter",
			command: "npm test -- {{.file}}",
			params:  map[string]interface{}{"file": "src/test.js"},
			want:    "npm test -- src/test.js",
		},
		{
			name:    "multiple parameters",
			command: "docker run -p {{.port}}:{{.port}} {{.image}}",
			params: map[string]interface{}{
				"port":  8080,
				"image": "nginx",
			},
			want: "docker run -p 8080:8080 nginx",
		},
		{
			name:    "string parameter",
			command: "echo {{.message}}",
			params:  map[string]interface{}{"message": "hello world"},
			want:    "echo hello world",
		},
		{
			name:    "boolean parameter",
			command: "run --verbose={{.verbose}}",
			params:  map[string]interface{}{"verbose": true},
			want:    "run --verbose=true",
		},
		{
			name:    "no parameters",
			command: "go test ./...",
			params:  map[string]interface{}{},
			want:    "go test ./...",
		},
		{
			name:      "missing required parameter",
			command:   "npm test -- {{.file}}",
			params:    map[string]interface{}{},
			wantError: true,
			errorMsg:  "execute command template",
		},
		{
			name:      "invalid template syntax",
			command:   "npm test -- {{.file",
			params:    map[string]interface{}{"file": "test.js"},
			wantError: true,
			errorMsg:  "parse command template",
		},
		{
			name:    "whitespace control",
			command: "echo {{- .message -}}",
			params:  map[string]interface{}{"message": "hello"},
			want:    "echohello",
		},
		{
			name:    "nested field",
			command: "docker run {{.config.image}}",
			params: map[string]interface{}{
				"config": map[string]interface{}{
					"image": "nginx",
				},
			},
			want: "docker run nginx",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SubstituteParameters(tt.command, tt.params)
			if tt.wantError {
				if err == nil {
					t.Errorf("expected error, got nil")
					return
				}
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got: %v", tt.errorMsg, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result != tt.want {
				t.Errorf("expected %q, got %q", tt.want, result)
			}
		})
	}
}

func TestTaskWrapper(t *testing.T) {
	wrapper := &TaskWrapper{
		Name:        "test-task",
		Description: "Test description",
		Type:        config.TaskTypeOneShot,
	}

	tests := []struct {
		name   string
		method func() string
		want   string
	}{
		{"Run", wrapper.Run, "run_test-task"},
		{"Start", wrapper.Start, "start_test-task"},
		{"Stop", wrapper.Stop, "stop_test-task"},
		{"Status", wrapper.Status, "status_test-task"},
		{"Logs", wrapper.Logs, "logs_test-task"},
		{"Desc", wrapper.Desc, "Test description"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.method()
			if result != tt.want {
				t.Errorf("expected %q, got %q", tt.want, result)
			}
		})
	}
}

func TestPromptTemplateEdgeCases(t *testing.T) {
	tasks := map[string]config.Task{
		"task_with_special_chars": {
			Description: "Task with special chars: !@#$%",
			Command:     "echo test",
			Type:        config.TaskTypeOneShot,
		},
	}

	tests := []struct {
		name     string
		template string
		want     string
	}{
		{
			name:     "empty template",
			template: "",
			want:     "",
		},
		{
			name:     "no template actions",
			template: "This is a plain string",
			want:     "This is a plain string",
		},
		{
			name:     "special chars in description",
			template: "{{.Tasks.task_with_special_chars.Desc}}",
			want:     "Task with special chars: !@#$%",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ResolvePromptTemplate(tt.template, tasks)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result != tt.want {
				t.Errorf("expected %q, got %q", tt.want, result)
			}
		})
	}
}

func TestParameterSubstitutionEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		command string
		params  map[string]interface{}
		want    string
	}{
		{
			name:    "empty command",
			command: "",
			params:  map[string]interface{}{},
			want:    "",
		},
		{
			name:    "special characters in parameter",
			command: "echo {{.msg}}",
			params:  map[string]interface{}{"msg": "hello!@#$%^&*()"},
			want:    "echo hello!@#$%^&*()",
		},
		{
			name:    "numeric parameter",
			command: "sleep {{.seconds}}",
			params:  map[string]interface{}{"seconds": 10},
			want:    "sleep 10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SubstituteParameters(tt.command, tt.params)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result != tt.want {
				t.Errorf("expected %q, got %q", tt.want, result)
			}
		})
	}
}
