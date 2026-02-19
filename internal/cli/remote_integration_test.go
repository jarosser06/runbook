package cli

import (
	"context"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	mcpclient "github.com/mark3labs/mcp-go/client"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/mark3labs/mcp-go/mcp"
	"runbookmcp.dev/internal/config"
	"runbookmcp.dev/internal/logs"
	"runbookmcp.dev/internal/mcputil"
	"runbookmcp.dev/internal/server"
	"runbookmcp.dev/internal/task"
)

// newTestServer creates an httptest server backed by a runbook Server with the given manifest.
func newTestServer(t *testing.T, manifest *config.Manifest) *httptest.Server {
	t.Helper()

	mgr := task.NewManager(manifest, nil)
	srv := server.NewServer(manifest, mgr, nil, true, "test", "")
	ts := httptest.NewServer(mcpserver.NewStreamableHTTPServer(srv.GetMCPServer()))
	t.Cleanup(ts.Close)
	return ts
}

// newTestMCPClient creates an initialized MCP client pointing at the given httptest server.
func newTestMCPClient(t *testing.T, ts *httptest.Server) *mcpclient.Client {
	t.Helper()

	c, err := mcpclient.NewStreamableHttpClient(mcputil.Endpoint(ts.URL))
	if err != nil {
		t.Fatalf("NewStreamableHttpClient: %v", err)
	}
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("client.Start: %v", err)
	}
	if _, err := c.Initialize(context.Background(), mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			ClientInfo:      mcp.Implementation{Name: "test", Version: "0"},
		},
	}); err != nil {
		t.Fatalf("client.Initialize: %v", err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func setupTestLogs(t *testing.T) {
	t.Helper()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Errorf("restore wd: %v", err)
		}
	})
	if err := logs.Setup(); err != nil {
		t.Fatalf("logs.Setup: %v", err)
	}
}

func TestRemoteCallTool_FormattedOutput_NotRawJSON(t *testing.T) {
	setupTestLogs(t)

	manifest := &config.Manifest{
		Version: "1",
		Tasks: map[string]config.Task{
			"hello": {
				Type:        config.TaskTypeOneShot,
				Description: "print hello",
				Command:     "echo hello world",
			},
		},
	}

	ts := newTestServer(t, manifest)
	c := newTestMCPClient(t, ts)

	stdout, stderr := captureOutput(func() {
		params := map[string]any{"max_output_lines": float64(0)}
		code, found := callTool(context.Background(), c, "run_hello", params)
		if !found {
			t.Error("tool not found")
		}
		if code != 0 {
			t.Errorf("exit code = %d, want 0", code)
		}
	})

	// Must contain the task output
	if !strings.Contains(stdout, "hello world") {
		t.Errorf("stdout %q does not contain 'hello world'", stdout)
	}
	// Must contain the formatted status line
	if !strings.Contains(stderr, "[OK]") {
		t.Errorf("stderr %q does not contain '[OK]'", stderr)
	}
	// Must NOT be raw JSON
	combined := stdout + stderr
	if strings.Contains(combined, `"success"`) {
		t.Errorf("output contains raw JSON field 'success': stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestRemoteCallTool_FailureFormatted(t *testing.T) {
	setupTestLogs(t)

	manifest := &config.Manifest{
		Version: "1",
		Tasks: map[string]config.Task{
			"fail": {
				Type:        config.TaskTypeOneShot,
				Description: "exit with failure",
				Command:     "exit 42",
			},
		},
	}

	ts := newTestServer(t, manifest)
	c := newTestMCPClient(t, ts)

	_, stderr := captureOutput(func() {
		params := map[string]any{"max_output_lines": float64(0)}
		callTool(context.Background(), c, "run_fail", params) //nolint:errcheck
	})

	if !strings.Contains(stderr, "[FAIL]") {
		t.Errorf("stderr %q does not contain '[FAIL]'", stderr)
	}
	if !strings.Contains(stderr, "exit code 42") {
		t.Errorf("stderr %q does not contain 'exit code 42'", stderr)
	}
	if strings.Contains(stderr, `"success"`) {
		t.Errorf("stderr contains raw JSON: %q", stderr)
	}
}

func TestRemoteCallTool_UnlimitedOutput(t *testing.T) {
	setupTestLogs(t)

	manifest := &config.Manifest{
		Version: "1",
		Tasks: map[string]config.Task{
			"bigout": {
				Type:        config.TaskTypeOneShot,
				Description: "200 lines",
				Command:     "seq 1 200",
			},
		},
	}

	ts := newTestServer(t, manifest)
	c := newTestMCPClient(t, ts)

	stdout, _ := captureOutput(func() {
		params := map[string]any{"max_output_lines": float64(0)}
		callTool(context.Background(), c, "run_bigout", params) //nolint:errcheck
	})

	// With max_output_lines=0 we should get all 200 lines, not the default 100
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) < 200 {
		t.Errorf("got %d lines, want 200 (truncation not bypassed)", len(lines))
	}
}
