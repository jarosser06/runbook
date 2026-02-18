// Package main integration tests exercise the full binary end-to-end:
// server registry file lifecycle, CLI auto-detection of a running HTTP server,
// --local bypass, stale-file hard error, and graceful shutdown cleanup.
//
// These tests require "go test -run Integration -v" and take a few seconds
// because they spawn real OS processes.
package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// buildBinary compiles the runbook binary and returns the path. The binary
// is removed when the test completes.
// testBinaryPath holds the path to the compiled runbook binary, built once by
// TestMain and shared across all integration tests.
var testBinaryPath string

// TestMain builds the binary once for the entire test run, then executes tests.
// This avoids O(N) recompilation across N integration tests.
func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "runbook-integration-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmp)

	testBinaryPath = filepath.Join(tmp, "runbook")
	out, err := exec.Command("go", "build", "-o", testBinaryPath, ".").CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "go build: %v\n%s\n", err, out)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

func buildBinary(t *testing.T) string {
	t.Helper()
	if testBinaryPath == "" {
		t.Fatal("testBinaryPath not set — TestMain did not run")
	}
	return testBinaryPath
}

// minimalConfig returns a YAML config with one oneshot task and one daemon.
func testConfig() string {
	return `version: "1.0"
tasks:
  echo:
    description: "Echo task"
    command: "echo 'hello from echo'"
    type: oneshot
  sleeper:
    description: "Sleep daemon"
    command: "while true; do sleep 1; done"
    type: daemon
workflows:
  echo-wf:
    description: "Echo workflow"
    steps:
      - task: echo
`
}

// freePort returns an available TCP port.
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("freePort: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}

// serverJSON is the shape of ._dev_tools/server.json.
type serverJSON struct {
	Addr string `json:"addr"`
	PID  int    `json:"pid"`
}

// readServerJSON parses ._dev_tools/server.json from the given dir.
func readServerJSON(t *testing.T, dir string) serverJSON {
	t.Helper()
	path := filepath.Join(dir, "._dev_tools", "server.json")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read server.json: %v", err)
	}
	var d serverJSON
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatalf("parse server.json: %v", err)
	}
	return d
}

// startServer launches the binary in -serve mode and waits until
// ._dev_tools/server.json appears (or the deadline). It returns the process
// so the caller can stop it, plus the port it's listening on.
func startServer(t *testing.T, bin, dir string, port int) *os.Process {
	t.Helper()
	addr := fmt.Sprintf(":%d", port)
	cmd := exec.Command(bin, "serve", "--addr", addr)
	cmd.Dir = dir
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}
	// Register cleanup so the process is killed when the test ends, even if the
	// caller forgets to defer proc.Kill() or if the test panics.
	t.Cleanup(func() { cmd.Process.Kill() }) //nolint:errcheck

	registryPath := filepath.Join(dir, "._dev_tools", "server.json")
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(registryPath); err == nil {
			return cmd.Process
		}
		time.Sleep(100 * time.Millisecond)
	}
	cmd.Process.Kill() //nolint:errcheck
	t.Fatalf("server.json never appeared in %s", dir)
	return nil
}

// runCLI executes the binary with the given working dir and args.
// It returns (stdout+stderr combined, exit code).
func runCLI(t *testing.T, bin, dir string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			t.Fatalf("exec %v: %v", args, err)
		}
	}
	return string(out), code
}

// TestIntegrationServerRegistryWrittenOnStart verifies that starting the HTTP
// server creates ._dev_tools/server.json with the correct address and a valid PID.
func TestIntegrationServerRegistryWrittenOnStart(t *testing.T) {
	bin := buildBinary(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".dev_workflow.yaml"), []byte(testConfig()), 0644); err != nil {
		t.Fatal(err)
	}

	port := freePort(t)
	proc := startServer(t, bin, dir, port)
	defer proc.Kill() //nolint:errcheck

	d := readServerJSON(t, dir)

	wantAddr := fmt.Sprintf("http://localhost:%d", port)
	if d.Addr != wantAddr {
		t.Errorf("server.json addr = %q, want %q", d.Addr, wantAddr)
	}
	if d.PID != proc.Pid {
		t.Errorf("server.json PID = %d, want %d", d.PID, proc.Pid)
	}
}

// TestIntegrationGracefulShutdownCleansRegistry verifies that sending SIGTERM
// to the HTTP server removes ._dev_tools/server.json.
func TestIntegrationGracefulShutdownCleansRegistry(t *testing.T) {
	bin := buildBinary(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".dev_workflow.yaml"), []byte(testConfig()), 0644); err != nil {
		t.Fatal(err)
	}

	port := freePort(t)
	proc := startServer(t, bin, dir, port)

	// Send SIGTERM for graceful shutdown.
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("SIGTERM: %v", err)
	}
	// Wait up to 5 seconds for server.json to disappear.
	registryPath := filepath.Join(dir, "._dev_tools", "server.json")
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(registryPath); os.IsNotExist(err) {
			return // PASS
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Error("server.json still exists after graceful SIGTERM")
}

// TestIntegrationCLIAutoDetectsServer verifies that when a server is running,
// "runbook list" routes through it (no local bootstrap required).
func TestIntegrationCLIAutoDetectsServer(t *testing.T) {
	bin := buildBinary(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".dev_workflow.yaml"), []byte(testConfig()), 0644); err != nil {
		t.Fatal(err)
	}

	port := freePort(t)
	proc := startServer(t, bin, dir, port)
	defer proc.Kill() //nolint:errcheck

	out, code := runCLI(t, bin, dir, "list")
	if code != 0 {
		t.Fatalf("runbook list exit=%d output=%q", code, out)
	}
	if !strings.Contains(out, "echo") {
		t.Errorf("expected 'echo' in list output, got: %q", out)
	}
}

// TestIntegrationCLIRunViaServer verifies that "runbook run <task>" executes
// via the running server and returns output.
func TestIntegrationCLIRunViaServer(t *testing.T) {
	bin := buildBinary(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".dev_workflow.yaml"), []byte(testConfig()), 0644); err != nil {
		t.Fatal(err)
	}

	port := freePort(t)
	proc := startServer(t, bin, dir, port)
	defer proc.Kill() //nolint:errcheck

	out, code := runCLI(t, bin, dir, "run", "echo")
	if code != 0 {
		t.Fatalf("runbook run echo exit=%d output=%q", code, out)
	}
	if !strings.Contains(out, "hello from echo") {
		t.Errorf("expected task output in response, got: %q", out)
	}
}

// TestIntegrationCLIRunWorkflowViaServer verifies that "runbook run <workflow>"
// falls back to the run_workflow_ tool name when run_<name> doesn't exist.
func TestIntegrationCLIRunWorkflowViaServer(t *testing.T) {
	bin := buildBinary(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".dev_workflow.yaml"), []byte(testConfig()), 0644); err != nil {
		t.Fatal(err)
	}

	port := freePort(t)
	proc := startServer(t, bin, dir, port)
	defer proc.Kill() //nolint:errcheck

	out, code := runCLI(t, bin, dir, "run", "echo-wf")
	if code != 0 {
		t.Fatalf("runbook run echo-wf exit=%d output=%q", code, out)
	}
	// Workflow response contains step results
	if !strings.Contains(out, "echo-wf") {
		t.Errorf("expected workflow name in response, got: %q", out)
	}
}

// TestIntegrationCLILocalFlagBypassesServer verifies that --local skips the
// running server and executes locally.
func TestIntegrationCLILocalFlagBypassesServer(t *testing.T) {
	bin := buildBinary(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".dev_workflow.yaml"), []byte(testConfig()), 0644); err != nil {
		t.Fatal(err)
	}

	port := freePort(t)
	proc := startServer(t, bin, dir, port)
	defer proc.Kill() //nolint:errcheck

	// --local should run locally even with a live server.json present.
	out, code := runCLI(t, bin, dir, "--local", "list")
	if code != 0 {
		t.Fatalf("runbook --local list exit=%d output=%q", code, out)
	}
	if !strings.Contains(out, "echo") {
		t.Errorf("expected local list output, got: %q", out)
	}
}

// TestIntegrationStaleServerJSONHardError verifies that when server.json exists
// but the server process is dead, the CLI prints a descriptive error and exits 1.
// This must NOT silently fall through to local mode (by design).
func TestIntegrationStaleServerJSONHardError(t *testing.T) {
	bin := buildBinary(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".dev_workflow.yaml"), []byte(testConfig()), 0644); err != nil {
		t.Fatal(err)
	}

	port := freePort(t)
	proc := startServer(t, bin, dir, port)

	// Kill the server without graceful shutdown so server.json remains.
	if err := proc.Kill(); err != nil {
		t.Fatalf("kill: %v", err)
	}
	proc.Wait() //nolint:errcheck

	// Verify server.json was NOT cleaned up (it was killed, not gracefully shut down).
	registryPath := filepath.Join(dir, "._dev_tools", "server.json")
	if _, err := os.Stat(registryPath); os.IsNotExist(err) {
		t.Skip("server cleaned up server.json on kill (unexpected but not a test failure)")
	}

	out, code := runCLI(t, bin, dir, "list")
	if code == 0 {
		t.Errorf("expected exit code 1 for stale server.json, got 0. output=%q", out)
	}
	if !strings.Contains(out, "server.json") {
		t.Errorf("expected error message mentioning server.json, got: %q", out)
	}
}

// TestIntegrationWorkingDirFlag verifies that -working-dir targets a project
// directory other than the current working directory.
func TestIntegrationWorkingDirFlag(t *testing.T) {
	bin := buildBinary(t)
	projectDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectDir, ".dev_workflow.yaml"), []byte(testConfig()), 0644); err != nil {
		t.Fatal(err)
	}

	port := freePort(t)

	// Start the server with -working-dir pointing to projectDir.
	otherDir := t.TempDir() // CWD for the server process (different from projectDir)
	cmd := exec.Command(bin, "serve", "--addr", fmt.Sprintf(":%d", port), "--working-dir", projectDir)
	cmd.Dir = otherDir
	if err := cmd.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}
	defer cmd.Process.Kill() //nolint:errcheck

	// Wait for server.json to appear in projectDir (not otherDir).
	registryPath := filepath.Join(projectDir, "._dev_tools", "server.json")
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(registryPath); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if _, err := os.Stat(registryPath); err != nil {
		t.Fatalf("server.json not found in projectDir after timeout: %v", err)
	}

	// CLI with -working-dir should route through the server.
	cliDir := t.TempDir() // CLI runs from a completely different directory
	out, code := runCLI(t, bin, cliDir, "--working-dir", projectDir, "list")
	if code != 0 {
		t.Fatalf("runbook -working-dir list exit=%d output=%q", code, out)
	}
	if !strings.Contains(out, "echo") {
		t.Errorf("expected 'echo' in output, got: %q", out)
	}
}

// TestIntegrationDaemonViaServer verifies start/status/stop daemon lifecycle
// when routed through the HTTP server.
func TestIntegrationDaemonViaServer(t *testing.T) {
	bin := buildBinary(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".dev_workflow.yaml"), []byte(testConfig()), 0644); err != nil {
		t.Fatal(err)
	}

	port := freePort(t)
	proc := startServer(t, bin, dir, port)
	defer proc.Kill() //nolint:errcheck

	// Start daemon.
	out, code := runCLI(t, bin, dir, "start", "sleeper")
	if code != 0 {
		t.Fatalf("start sleeper exit=%d output=%q", code, out)
	}
	if !strings.Contains(out, `"success":true`) {
		t.Errorf("expected success in start output, got: %q", out)
	}

	// Check status.
	out, code = runCLI(t, bin, dir, "status", "sleeper")
	if code != 0 {
		t.Fatalf("status sleeper exit=%d output=%q", code, out)
	}
	if !strings.Contains(out, `"running":true`) {
		t.Errorf("expected running=true in status, got: %q", out)
	}

	// Stop daemon.
	out, code = runCLI(t, bin, dir, "stop", "sleeper")
	if code != 0 {
		t.Fatalf("stop sleeper exit=%d output=%q", code, out)
	}
	if !strings.Contains(out, "success") {
		t.Errorf("expected success in stop output, got: %q", out)
	}
}

// TestIntegrationLogsReadLocally verifies that "runbook logs" reads log files
// from disk directly (not through the server), so it works even for oneshot tasks.
func TestIntegrationLogsReadLocally(t *testing.T) {
	bin := buildBinary(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".dev_workflow.yaml"), []byte(testConfig()), 0644); err != nil {
		t.Fatal(err)
	}

	port := freePort(t)
	proc := startServer(t, bin, dir, port)
	defer proc.Kill() //nolint:errcheck

	// Run a task to produce logs.
	_, code := runCLI(t, bin, dir, "run", "echo")
	if code != 0 {
		t.Fatalf("run echo failed")
	}

	// Read the logs — must work without trying logs_echo MCP tool.
	out, code := runCLI(t, bin, dir, "logs", "echo")
	if code != 0 {
		t.Fatalf("logs echo exit=%d output=%q", code, out)
	}
	if !strings.Contains(out, "hello from echo") {
		t.Errorf("expected log content in output, got: %q", out)
	}
}

// TestIntegrationStdioProxyHTTPProbeMismatch verifies that when server.json exists
// with a live PID but the HTTP port is not listening, the binary in stdio proxy
// mode exits with a clear error message referencing server.json — rather than
// silently attempting to proxy and failing with a generic connection error.
func TestIntegrationStdioProxyHTTPProbeMismatch(t *testing.T) {
	bin := buildBinary(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".dev_workflow.yaml"), []byte(testConfig()), 0644); err != nil {
		t.Fatal(err)
	}

	// Write server.json with our own PID (definitely alive) but an HTTP port
	// that has nothing listening on it.
	devToolsDir := filepath.Join(dir, "._dev_tools")
	if err := os.MkdirAll(devToolsDir, 0755); err != nil {
		t.Fatal(err)
	}
	sj := serverJSON{Addr: "http://localhost:19741", PID: os.Getpid()}
	b, err := json.Marshal(sj)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(devToolsDir, "server.json"), b, 0644); err != nil {
		t.Fatal(err)
	}

	// Invoke binary in stdio proxy mode (no subcommand, no -serve).
	// Pass an empty stdin so it doesn't hang waiting for input.
	out, code := runCLI(t, bin, dir)
	if code == 0 {
		t.Errorf("expected non-zero exit for dead HTTP with live PID, got 0. output=%q", out)
	}
	// The error must mention server.json (clear diagnostic), not "Proxy error" (opaque).
	if !strings.Contains(out, "server.json") {
		t.Errorf("expected error message referencing server.json, got: %q", out)
	}
}
