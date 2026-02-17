package cli

import (
	"testing"
	"time"

	"github.com/jarosser06/runbook/internal/config"
)

func TestParseGlobalFlags(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		wantConfig     string
		wantRemaining  []string
	}{
		{
			name:           "no flags",
			args:           []string{"build"},
			wantConfig:     "",
			wantRemaining:  []string{"build"},
		},
		{
			name:           "config with equals",
			args:           []string{"--config=path/to/config.yaml", "build"},
			wantConfig:     "path/to/config.yaml",
			wantRemaining:  []string{"build"},
		},
		{
			name:           "config with space",
			args:           []string{"--config", "path/to/config.yaml", "build"},
			wantConfig:     "path/to/config.yaml",
			wantRemaining:  []string{"build"},
		},
		{
			name:           "config after task name",
			args:           []string{"build", "--config=path/to/config.yaml"},
			wantConfig:     "path/to/config.yaml",
			wantRemaining:  []string{"build"},
		},
		{
			name:           "config with task params",
			args:           []string{"--config=config.yaml", "build", "--flags=-v"},
			wantConfig:     "config.yaml",
			wantRemaining:  []string{"build", "--flags=-v"},
		},
		{
			name:           "single dash config",
			args:           []string{"-config=config.yaml", "build"},
			wantConfig:     "config.yaml",
			wantRemaining:  []string{"build"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotConfig, gotRemaining := parseGlobalFlags(tt.args)
			if gotConfig != tt.wantConfig {
				t.Errorf("config = %q, want %q", gotConfig, tt.wantConfig)
			}
			if len(gotRemaining) != len(tt.wantRemaining) {
				t.Errorf("remaining = %v (len %d), want %v (len %d)",
					gotRemaining, len(gotRemaining), tt.wantRemaining, len(tt.wantRemaining))
				return
			}
			for i := range gotRemaining {
				if gotRemaining[i] != tt.wantRemaining[i] {
					t.Errorf("remaining[%d] = %q, want %q", i, gotRemaining[i], tt.wantRemaining[i])
				}
			}
		})
	}
}

func TestParseTaskParams(t *testing.T) {
	defaultVal := "default_value"
	taskDef := config.Task{
		Parameters: map[string]config.Param{
			"name": {
				Type:        "string",
				Required:    true,
				Description: "The name",
			},
			"count": {
				Type:        "string",
				Required:    false,
				Description: "The count",
				Default:     &defaultVal,
			},
		},
	}

	tests := []struct {
		name    string
		args    []string
		wantErr bool
		check   func(t *testing.T, params map[string]interface{})
	}{
		{
			name: "all params provided",
			args: []string{"--name=hello", "--count=5"},
			check: func(t *testing.T, params map[string]interface{}) {
				if params["name"] != "hello" {
					t.Errorf("name = %v, want hello", params["name"])
				}
				if params["count"] != "5" {
					t.Errorf("count = %v, want 5", params["count"])
				}
			},
		},
		{
			name: "required only with default",
			args: []string{"--name=hello"},
			check: func(t *testing.T, params map[string]interface{}) {
				if params["name"] != "hello" {
					t.Errorf("name = %v, want hello", params["name"])
				}
				if params["count"] != "default_value" {
					t.Errorf("count = %v, want default_value", params["count"])
				}
			},
		},
		{
			name:    "missing required param",
			args:    []string{},
			wantErr: true,
		},
		{
			name:    "unexpected positional args",
			args:    []string{"--name=hello", "extra"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params, err := parseTaskParams(taskDef, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr = %v", err, tt.wantErr)
				return
			}
			if err == nil && tt.check != nil {
				tt.check(t, params)
			}
		})
	}
}

func TestParseTaskParamsNoParams(t *testing.T) {
	taskDef := config.Task{
		Parameters: nil,
	}

	// No args should be fine
	params, err := parseTaskParams(taskDef, nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(params) != 0 {
		t.Errorf("expected empty params, got %v", params)
	}

	// Args should be rejected
	_, err = parseTaskParams(taskDef, []string{"--foo=bar"})
	if err == nil {
		t.Error("expected error for params on parameterless task")
	}
}

func TestExecuteUnknownCommand(t *testing.T) {
	code := Execute([]string{"unknown"})
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
}

func TestExecuteNoArgs(t *testing.T) {
	code := Execute(nil)
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name string
		ms   int
		want string
	}{
		{"milliseconds", 50, "50ms"},
		{"seconds", 2500, "2.5s"},
		{"minutes", 125000, "2m5s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := toDuration(tt.ms)
			got := formatDuration(d)
			if got != tt.want {
				t.Errorf("formatDuration(%v) = %q, want %q", d, got, tt.want)
			}
		})
	}
}

func toDuration(ms int) time.Duration {
	return time.Duration(ms) * time.Millisecond
}
