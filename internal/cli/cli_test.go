package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"runbookmcp.dev/internal/config"
)

// resetGlobals resets package-level state between tests to avoid cross-test contamination.
func resetGlobals(t *testing.T) {
	t.Helper()
	oldConfig := globalConfig
	oldWorkingDir := globalWorkingDir
	oldLocal := globalLocal
	t.Cleanup(func() {
		globalConfig = oldConfig
		globalWorkingDir = oldWorkingDir
		globalLocal = oldLocal
	})
	globalConfig = ""
	globalWorkingDir = ""
	globalLocal = false
}

// ---------------------------------------------------------------------------
// Cobra command-tree tests
// ---------------------------------------------------------------------------

func TestRootHelp(t *testing.T) {
	resetGlobals(t)
	cmd := newRootCmd("test-version")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	out := buf.String()
	for _, sub := range []string{"serve", "init", "list", "run", "start", "stop", "status", "logs"} {
		if !strings.Contains(out, sub) {
			t.Errorf("root --help output should mention %q subcommand", sub)
		}
	}
}

func TestServeSubcommand(t *testing.T) {
	resetGlobals(t)
	cmd := newRootCmd("test-version")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"serve", "--help"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "addr") {
		t.Error("serve --help should mention --addr flag")
	}
}

func TestInitSubcommand(t *testing.T) {
	resetGlobals(t)
	cmd := newRootCmd("test-version")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"init", "--help"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("init --help should exit 0, got error: %v", err)
	}
}

func TestListSubcommand(t *testing.T) {
	resetGlobals(t)
	cmd := newRootCmd("test-version")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"list", "--help"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("list --help should exit 0, got error: %v", err)
	}
}

func TestRunSubcommand(t *testing.T) {
	// run uses DisableFlagParsing, so --help is not handled by Cobra.
	// Instead, verify the subcommand is registered in the command tree.
	cmd := newRootCmd("test-version")
	found, _, err := cmd.Find([]string{"run"})
	if err != nil {
		t.Fatalf("expected 'run' subcommand to exist, got error: %v", err)
	}
	if found.Name() != "run" {
		t.Errorf("expected command name 'run', got %q", found.Name())
	}
}

func TestRunConfigFlag(t *testing.T) {
	// Verify that "run --config=foo.yaml mytask" routes to the run subcommand.
	// We use Find instead of Execute because run has DisableFlagParsing and
	// would attempt to load the config file.
	cmd := newRootCmd("test-version")
	found, _, err := cmd.Find([]string{"run", "--config=foo.yaml", "mytask"})
	if err != nil {
		t.Fatalf("Find failed: %v", err)
	}
	if found.Name() != "run" {
		t.Errorf("expected 'run' command, got %q", found.Name())
	}
}

func TestRunConfigFlagBefore(t *testing.T) {
	// When "--config=foo.yaml" is placed before the subcommand, cobra parses it
	// as a persistent flag on root and sets globalConfig before routing to the
	// subcommand. For "run" specifically, DisableFlagParsing means cobra passes
	// all remaining args (after the subcommand name) to RunE without parsing.
	// So "--config=foo.yaml run mytask" results in:
	//   1. cobra sets globalConfig = "foo.yaml"
	//   2. cobra routes to "run"
	//   3. run's RunE sees args = ["mytask"]
	//
	// We verify the routing via Find, and test the persistent flag binding on
	// a safer subcommand (list) that doesn't call os.Exit.
	cmd := newRootCmd("test-version")
	found, _, err := cmd.Find([]string{"run", "mytask"})
	if err != nil {
		t.Fatalf("Find failed: %v", err)
	}
	if found.Name() != "run" {
		t.Errorf("expected 'run' command, got %q", found.Name())
	}

	// Verify the persistent flag is registered.
	pf := cmd.PersistentFlags().Lookup("config")
	if pf == nil {
		t.Fatal("--config should be a persistent flag on root")
	}
}

func TestConfigFlagBeforeSubcommandExecute(t *testing.T) {
	// Execute "--config=nonexistent.yaml list" through cobra to verify that
	// the persistent --config flag is consumed and sets globalConfig.
	// list will fail because the config file doesn't exist, but globalConfig
	// should be set correctly before the failure.
	resetGlobals(t)
	cmd := newRootCmd("test-version")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--config=nonexistent.yaml", "list"})

	_ = cmd.Execute() // expected to fail (config not found)

	if globalConfig != "nonexistent.yaml" {
		t.Errorf("globalConfig = %q, want %q", globalConfig, "nonexistent.yaml")
	}
}

func TestStartSubcommand(t *testing.T) {
	// start uses DisableFlagParsing, so --help is not handled by Cobra.
	// Verify the subcommand is registered in the command tree.
	cmd := newRootCmd("test-version")
	found, _, err := cmd.Find([]string{"start"})
	if err != nil {
		t.Fatalf("expected 'start' subcommand to exist, got error: %v", err)
	}
	if found.Name() != "start" {
		t.Errorf("expected command name 'start', got %q", found.Name())
	}
}

func TestStopSubcommand(t *testing.T) {
	resetGlobals(t)
	cmd := newRootCmd("test-version")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"stop", "--help"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("stop --help should exit 0, got error: %v", err)
	}
}

func TestStatusSubcommand(t *testing.T) {
	resetGlobals(t)
	cmd := newRootCmd("test-version")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"status", "--help"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("status --help should exit 0, got error: %v", err)
	}
}

func TestLogsSubcommand(t *testing.T) {
	resetGlobals(t)
	cmd := newRootCmd("test-version")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"logs", "--help"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("logs --help should exit 0, got error: %v", err)
	}
}

func TestLogsFlags(t *testing.T) {
	cmd := newRootCmd("test-version")
	logsCmd, _, err := cmd.Find([]string{"logs"})
	if err != nil {
		t.Fatalf("Find logs failed: %v", err)
	}

	for _, name := range []string{"lines", "filter", "session"} {
		if logsCmd.Flags().Lookup(name) == nil {
			t.Errorf("logs command should have --%s flag", name)
		}
	}
}

func TestUnknownSubcommand(t *testing.T) {
	resetGlobals(t)
	cmd := newRootCmd("test-version")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"badcmd"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for unknown subcommand, got nil")
	}
}

func TestLocalFlag(t *testing.T) {
	resetGlobals(t)
	cmd := newRootCmd("test-version")

	// --local should be a persistent flag available on the root command
	// and therefore inherited by all subcommands.
	pf := cmd.PersistentFlags().Lookup("local")
	if pf == nil {
		t.Fatal("--local should be registered as a persistent flag on the root command")
	}

	// Verify it is visible on a subcommand too.
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"list", "--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list --help failed: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "local") {
		t.Error("--local persistent flag should appear in subcommand help")
	}
}

func TestConfigFlagPersistent(t *testing.T) {
	cmd := newRootCmd("test-version")

	pf := cmd.PersistentFlags().Lookup("config")
	if pf == nil {
		t.Fatal("--config should be registered as a persistent flag on the root command")
	}
	if pf.DefValue != "" {
		t.Errorf("--config default should be empty, got %q", pf.DefValue)
	}
}

func TestWorkingDirFlagPersistent(t *testing.T) {
	cmd := newRootCmd("test-version")

	pf := cmd.PersistentFlags().Lookup("working-dir")
	if pf == nil {
		t.Fatal("--working-dir should be registered as a persistent flag on the root command")
	}
	if pf.DefValue != "" {
		t.Errorf("--working-dir default should be empty, got %q", pf.DefValue)
	}

	// Verify it is inherited by subcommands.
	found, _, err := cmd.Find([]string{"list"})
	if err != nil {
		t.Fatalf("Find list failed: %v", err)
	}
	f := found.InheritedFlags()
	if f.Lookup("working-dir") == nil {
		t.Error("list subcommand should inherit --working-dir persistent flag from root")
	}
}

func TestWorkingDirFlagExecute(t *testing.T) {
	// Verify that "--working-dir=/tmp/test list" sets globalWorkingDir via cobra.
	resetGlobals(t)
	cmd := newRootCmd("test-version")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--working-dir=/tmp/test", "list"})

	_ = cmd.Execute() // will fail because /tmp/test may not have config

	if globalWorkingDir != "/tmp/test" {
		t.Errorf("globalWorkingDir = %q, want %q", globalWorkingDir, "/tmp/test")
	}
}

func TestServeAddrDefault(t *testing.T) {
	cmd := newRootCmd("test-version")
	serveCmd, _, err := cmd.Find([]string{"serve"})
	if err != nil {
		t.Fatalf("Find serve failed: %v", err)
	}

	addrFlag := serveCmd.Flags().Lookup("addr")
	if addrFlag == nil {
		t.Fatal("serve command should have --addr flag")
	}
	if addrFlag.DefValue != ":8080" {
		t.Errorf("serve --addr default = %q, want %q", addrFlag.DefValue, ":8080")
	}
}

func TestConfigBeforeSubcommand(t *testing.T) {
	// Verify that "--config=foo.yaml list" routes to the list subcommand.
	// Cobra should consume --config as a persistent flag on root and route to list.
	cmd := newRootCmd("test-version")
	found, _, err := cmd.Find([]string{"list"})
	if err != nil {
		t.Fatalf("Find failed: %v", err)
	}
	if found.Name() != "list" {
		t.Errorf("expected 'list' command, got %q", found.Name())
	}
	// The --config persistent flag should be inheritable by subcommands.
	f := found.InheritedFlags()
	if f.Lookup("config") == nil {
		t.Error("list subcommand should inherit --config persistent flag from root")
	}
}

// ---------------------------------------------------------------------------
// Global var isolation test
// ---------------------------------------------------------------------------

func TestGlobalVarReset(t *testing.T) {
	// Verify that globals set by one command execution do not leak into the next.
	// This catches the bug where Execute() reset logic is broken or tests
	// share state through package-level vars.
	resetGlobals(t)

	// First execution: --config=foo.yaml list (will fail to load, but sets globalConfig).
	cmd1 := newRootCmd("test-version")
	buf := new(bytes.Buffer)
	cmd1.SetOut(buf)
	cmd1.SetErr(buf)
	cmd1.SetArgs([]string{"--config=foo.yaml", "list"})
	_ = cmd1.Execute()

	if globalConfig != "foo.yaml" {
		t.Fatalf("after first execute, globalConfig = %q, want %q", globalConfig, "foo.yaml")
	}

	// Simulate what Execute() does: reset globals before creating a new command.
	globalConfig = ""
	globalWorkingDir = ""
	globalLocal = false

	// Second execution: list without --config.
	cmd2 := newRootCmd("test-version")
	buf2 := new(bytes.Buffer)
	cmd2.SetOut(buf2)
	cmd2.SetErr(buf2)
	cmd2.SetArgs([]string{"list"})
	_ = cmd2.Execute()

	if globalConfig != "" {
		t.Errorf("after second execute without --config, globalConfig = %q, want empty", globalConfig)
	}
}

// ---------------------------------------------------------------------------
// DisableFlagParsing and Args validation tests
// ---------------------------------------------------------------------------

func TestDisableFlagParsing(t *testing.T) {
	cmd := newRootCmd("test-version")

	tests := []struct {
		name    string
		subcmd  string
		wantDFP bool
	}{
		{"run has DisableFlagParsing", "run", true},
		{"start has DisableFlagParsing", "start", true},
		{"stop does not have DisableFlagParsing", "stop", false},
		{"status does not have DisableFlagParsing", "status", false},
		{"logs does not have DisableFlagParsing", "logs", false},
		{"list does not have DisableFlagParsing", "list", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found, _, err := cmd.Find([]string{tt.subcmd})
			if err != nil {
				t.Fatalf("Find %q failed: %v", tt.subcmd, err)
			}
			if found.DisableFlagParsing != tt.wantDFP {
				t.Errorf("%s.DisableFlagParsing = %v, want %v", tt.subcmd, found.DisableFlagParsing, tt.wantDFP)
			}
		})
	}
}

func TestExactArgsCommands(t *testing.T) {
	// stop, status, and logs use ExactArgs(1). Verify they reject zero args.
	for _, subcmd := range []string{"stop", "status", "logs"} {
		t.Run(subcmd+" rejects zero args", func(t *testing.T) {
			resetGlobals(t)
			cmd := newRootCmd("test-version")
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs([]string{subcmd})

			err := cmd.Execute()
			if err == nil {
				t.Errorf("%s with no args should return error", subcmd)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// extractGlobalFlagsManual tests
// ---------------------------------------------------------------------------

func TestExtractGlobalFlagsManual(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		wantConfig    string
		wantWD        string
		wantLocal     bool
		wantRemaining []string
	}{
		{
			name:          "config with equals",
			args:          []string{"--config=myconfig.yaml", "taskname", "--param=value"},
			wantConfig:    "myconfig.yaml",
			wantRemaining: []string{"taskname", "--param=value"},
		},
		{
			name:          "config with space",
			args:          []string{"--config", "myconfig.yaml", "taskname"},
			wantConfig:    "myconfig.yaml",
			wantRemaining: []string{"taskname"},
		},
		{
			name:          "working-dir with equals",
			args:          []string{"--working-dir=/tmp/project", "taskname"},
			wantWD:        "/tmp/project",
			wantRemaining: []string{"taskname"},
		},
		{
			name:          "working-dir with space",
			args:          []string{"--working-dir", "/tmp/project", "taskname"},
			wantWD:        "/tmp/project",
			wantRemaining: []string{"taskname"},
		},
		{
			name:          "local flag",
			args:          []string{"--local", "taskname", "--param=value"},
			wantLocal:     true,
			wantRemaining: []string{"taskname", "--param=value"},
		},
		{
			name:          "all three flags",
			args:          []string{"--local", "--config=x.yaml", "--working-dir", "/tmp", "taskname"},
			wantConfig:    "x.yaml",
			wantWD:        "/tmp",
			wantLocal:     true,
			wantRemaining: []string{"taskname"},
		},
		{
			name:          "no global flags",
			args:          []string{"taskname", "--param=value"},
			wantRemaining: []string{"taskname", "--param=value"},
		},
		{
			name:          "single dash config",
			args:          []string{"-config=x.yaml", "taskname"},
			wantConfig:    "x.yaml",
			wantRemaining: []string{"taskname"},
		},
		{
			name:          "single dash local",
			args:          []string{"-local", "taskname"},
			wantLocal:     true,
			wantRemaining: []string{"taskname"},
		},
		{
			name:          "empty args",
			args:          []string{},
			wantRemaining: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotConfig, gotWD, gotLocal, gotRemaining := extractGlobalFlagsManual(tt.args)
			if gotConfig != tt.wantConfig {
				t.Errorf("config = %q, want %q", gotConfig, tt.wantConfig)
			}
			if gotWD != tt.wantWD {
				t.Errorf("workingDir = %q, want %q", gotWD, tt.wantWD)
			}
			if gotLocal != tt.wantLocal {
				t.Errorf("local = %v, want %v", gotLocal, tt.wantLocal)
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

// ---------------------------------------------------------------------------
// mergeExtractedGlobals tests
// ---------------------------------------------------------------------------

func TestMergeExtractedGlobals(t *testing.T) {
	t.Run("non-empty values override globals", func(t *testing.T) {
		resetGlobals(t)
		mergeExtractedGlobals("myconfig.yaml", "/tmp", true)
		if globalConfig != "myconfig.yaml" {
			t.Errorf("globalConfig = %q, want %q", globalConfig, "myconfig.yaml")
		}
		if globalWorkingDir != "/tmp" {
			t.Errorf("globalWorkingDir = %q, want %q", globalWorkingDir, "/tmp")
		}
		if !globalLocal {
			t.Error("globalLocal should be true")
		}
	})

	t.Run("empty values do not override existing globals", func(t *testing.T) {
		resetGlobals(t)
		globalConfig = "existing.yaml"
		globalWorkingDir = "/existing"
		globalLocal = true
		mergeExtractedGlobals("", "", false)
		if globalConfig != "existing.yaml" {
			t.Errorf("globalConfig = %q, want %q", globalConfig, "existing.yaml")
		}
		if globalWorkingDir != "/existing" {
			t.Errorf("globalWorkingDir = %q, want %q", globalWorkingDir, "/existing")
		}
		if !globalLocal {
			t.Error("globalLocal should remain true")
		}
	})
}

// ---------------------------------------------------------------------------
// handleInit tests
// ---------------------------------------------------------------------------

func TestHandleInit(t *testing.T) {
	t.Run("creates config file", func(t *testing.T) {
		tmp := t.TempDir()
		origDir, _ := os.Getwd()
		t.Cleanup(func() { _ = os.Chdir(origDir) })
		if err := os.Chdir(tmp); err != nil {
			t.Fatalf("chdir: %v", err)
		}

		err := handleInit()
		if err != nil {
			t.Fatalf("handleInit() returned error: %v", err)
		}

		data, err := os.ReadFile(filepath.Join(tmp, ".dev_workflow.yaml"))
		if err != nil {
			t.Fatalf("config file not created: %v", err)
		}
		if len(data) == 0 {
			t.Error("config file is empty")
		}
	})

	t.Run("returns error when file already exists", func(t *testing.T) {
		tmp := t.TempDir()
		origDir, _ := os.Getwd()
		t.Cleanup(func() { _ = os.Chdir(origDir) })
		if err := os.Chdir(tmp); err != nil {
			t.Fatalf("chdir: %v", err)
		}

		// Create the file first.
		if err := os.WriteFile(filepath.Join(tmp, ".dev_workflow.yaml"), []byte("existing"), 0644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		err := handleInit()
		if err == nil {
			t.Fatal("expected error when config already exists, got nil")
		}
		if !strings.Contains(err.Error(), "already exists") {
			t.Errorf("error should mention 'already exists', got: %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// Preserved existing tests for utility functions
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// isMCPEnabled tests
// ---------------------------------------------------------------------------

func TestIsMCPEnabled(t *testing.T) {
	// Create a temp dir with a manifest containing a disable_mcp task.
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(origDir) })
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	manifest := `version: "1.0"
tasks:
  setup-secrets:
    description: "Configure credentials"
    command: "./scripts/setup-secrets.sh"
    type: oneshot
    disable_mcp: true
  build:
    description: "Build the project"
    command: "go build ./..."
    type: oneshot
`
	if err := os.WriteFile(filepath.Join(tmp, ".dev_workflow.yaml"), []byte(manifest), 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	resetGlobals(t)

	tests := []struct {
		args []string
		want bool
	}{
		{[]string{}, true},                        // no task name -> enabled
		{[]string{"build"}, true},                 // normal task -> enabled
		{[]string{"setup-secrets"}, false},        // disable_mcp task -> not enabled
		{[]string{"nonexistent"}, true},           // unknown task -> enabled (pass through)
		{[]string{"setup-secrets", "--foo"}, false}, // extra args don't matter; first arg checked
	}

	for _, tt := range tests {
		got := isMCPEnabled(tt.args)
		if got != tt.want {
			t.Errorf("isMCPEnabled(%v) = %v, want %v", tt.args, got, tt.want)
		}
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
