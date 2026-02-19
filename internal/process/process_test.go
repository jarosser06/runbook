package process

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"runbookmcp.dev/internal/logs"
)

func TestManagerStartStop(t *testing.T) {
	// Setup
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

	if err := logs.Setup(); err != nil {
		t.Fatalf("failed to setup logs: %v", err)
	}

	manager := NewManager()
	logPath := logs.GetLogPath("test-daemon")

	// Start daemon
	err = manager.Start("test-daemon", "test-session-id", "sleep 10", nil, "", logPath, "")
	if err != nil {
		t.Fatalf("failed to start daemon: %v", err)
	}

	// Check status
	running, pid, err := manager.Status("test-daemon")
	if err != nil {
		t.Fatalf("failed to get status: %v", err)
	}
	if !running {
		t.Errorf("expected daemon to be running")
	}
	if pid == 0 {
		t.Errorf("expected non-zero PID")
	}

	// Stop daemon
	err = manager.Stop("test-daemon")
	if err != nil {
		t.Fatalf("failed to stop daemon: %v", err)
	}

	// Verify stopped
	running, _, err = manager.Status("test-daemon")
	if err != nil {
		t.Fatalf("failed to get status: %v", err)
	}
	if running {
		t.Errorf("expected daemon to be stopped")
	}
}

func TestManagerDoubleStart(t *testing.T) {
	// Setup
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

	if err := logs.Setup(); err != nil {
		t.Fatalf("failed to setup logs: %v", err)
	}

	manager := NewManager()
	logPath := logs.GetLogPath("test-daemon")

	// Start daemon
	err = manager.Start("test-daemon", "test-session-id", "sleep 10", nil, "", logPath, "")
	if err != nil {
		t.Fatalf("failed to start daemon: %v", err)
	}
	defer func() {
		_ = manager.Stop("test-daemon") // Cleanup, ignore errors
	}()

	// Try to start again
	err = manager.Start("test-daemon", "test-session-id", "sleep 10", nil, "", logPath, "")
	if err == nil {
		t.Errorf("expected error when starting already running daemon")
	}
}

func TestManagerStopNotRunning(t *testing.T) {
	manager := NewManager()

	// Try to stop non-existent daemon
	err := manager.Stop("nonexistent")
	if err == nil {
		t.Errorf("expected error when stopping non-existent daemon")
	}
}

func TestManagerStatusNotRunning(t *testing.T) {
	manager := NewManager()

	// Check status of non-existent daemon
	running, pid, err := manager.Status("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if running {
		t.Errorf("expected daemon to not be running")
	}
	if pid != 0 {
		t.Errorf("expected PID to be 0 for non-existent daemon")
	}
}

func TestManagerStopAll(t *testing.T) {
	// Setup
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

	if err := logs.Setup(); err != nil {
		t.Fatalf("failed to setup logs: %v", err)
	}

	manager := NewManager()

	// Start multiple daemons
	for i := 0; i < 3; i++ {
		taskName := fmt.Sprintf("daemon-%d", i)
		logPath := logs.GetLogPath(taskName)
		err := manager.Start(taskName, "test-session-id", "sleep 10", nil, "", logPath, "")
		if err != nil {
			t.Fatalf("failed to start daemon %s: %v", taskName, err)
		}
	}

	// Stop all
	err = manager.StopAll()
	if err != nil {
		t.Fatalf("failed to stop all daemons: %v", err)
	}

	// Verify all stopped
	for i := 0; i < 3; i++ {
		taskName := fmt.Sprintf("daemon-%d", i)
		running, _, err := manager.Status(taskName)
		if err != nil {
			t.Fatalf("failed to get status for %s: %v", taskName, err)
		}
		if running {
			t.Errorf("expected daemon %s to be stopped", taskName)
		}
	}
}

func TestManagerProcessExit(t *testing.T) {
	// Setup
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

	if err := logs.Setup(); err != nil {
		t.Fatalf("failed to setup logs: %v", err)
	}

	manager := NewManager()
	logPath := logs.GetLogPath("test-daemon")

	// Start daemon that exits quickly
	err = manager.Start("test-daemon", "test-session-id", "echo 'hello'", nil, "", logPath, "")
	if err != nil {
		t.Fatalf("failed to start daemon: %v", err)
	}

	// Wait for process to exit
	time.Sleep(100 * time.Millisecond)

	// Check status (should be not running)
	running, _, err := manager.Status("test-daemon")
	if err != nil {
		t.Fatalf("failed to get status: %v", err)
	}
	if running {
		t.Errorf("expected daemon to be stopped after exit")
	}
}

func TestManagerEnvironmentVariables(t *testing.T) {
	// Setup
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

	if err := logs.Setup(); err != nil {
		t.Fatalf("failed to setup logs: %v", err)
	}

	manager := NewManager()
	logPath := logs.GetLogPath("test-daemon")

	// Start daemon with environment variable
	env := map[string]string{"TEST_VAR": "test_value"}
	err = manager.Start("test-daemon", "test-session-id", "echo $TEST_VAR", env, "", logPath, "")
	if err != nil {
		t.Fatalf("failed to start daemon: %v", err)
	}

	// Wait for process to complete
	time.Sleep(100 * time.Millisecond)

	// Clean up
	_ = manager.Stop("test-daemon") // Ignore error during cleanup

	// Read log file to verify environment variable was set
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	if !strings.Contains(string(content), "test_value") {
		t.Errorf("expected log to contain 'test_value', got: %s", content)
	}
}

func TestManagerWorkingDirectory(t *testing.T) {
	// Setup
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

	if err := logs.Setup(); err != nil {
		t.Fatalf("failed to setup logs: %v", err)
	}

	// Create test directory
	testDir := "testdir"
	if err := os.Mkdir(testDir, 0755); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	manager := NewManager()
	logPath := logs.GetLogPath("test-daemon")

	// Start daemon with working directory
	err = manager.Start("test-daemon", "test-session-id", "pwd", nil, testDir, logPath, "")
	if err != nil {
		t.Fatalf("failed to start daemon: %v", err)
	}

	// Wait for process to complete
	time.Sleep(100 * time.Millisecond)

	// Clean up
	_ = manager.Stop("test-daemon") // Ignore error during cleanup

	// Read log file to verify working directory was set
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	if !strings.Contains(string(content), testDir) {
		t.Errorf("expected log to contain working directory path with '%s', got: %s", testDir, content)
	}
}

func TestManagerGetProcessInfo(t *testing.T) {
	// Setup
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

	if err := logs.Setup(); err != nil {
		t.Fatalf("failed to setup logs: %v", err)
	}

	manager := NewManager()
	logPath := logs.GetLogPath("test-daemon")

	// Start daemon
	err = manager.Start("test-daemon", "test-session-id", "sleep 10", nil, "", logPath, "")
	if err != nil {
		t.Fatalf("failed to start daemon: %v", err)
	}
	defer func() {
		_ = manager.Stop("test-daemon") // Cleanup, ignore errors
	}()

	// Get process info
	info, err := manager.GetProcessInfo("test-daemon")
	if err != nil {
		t.Fatalf("failed to get process info: %v", err)
	}

	if info.PID == 0 {
		t.Errorf("expected non-zero PID")
	}
	if info.LogFile != logPath {
		t.Errorf("expected log file %s, got %s", logPath, info.LogFile)
	}
	if info.StartTime.IsZero() {
		t.Errorf("expected non-zero start time")
	}
}

func TestManagerCustomShell(t *testing.T) {
	// Setup
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

	if err := logs.Setup(); err != nil {
		t.Fatalf("failed to setup logs: %v", err)
	}

	manager := NewManager()
	logPath := logs.GetLogPath("test-daemon")

	// Use sh explicitly; if shell routing works the process runs under sh
	err = manager.Start("test-daemon", "test-session-id", "echo $0", nil, "", logPath, "/bin/sh")
	if err != nil {
		t.Fatalf("failed to start daemon with custom shell: %v", err)
	}

	// Wait for process to complete
	time.Sleep(100 * time.Millisecond)

	// Clean up
	_ = manager.Stop("test-daemon")

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	// $0 in a shell script is the shell binary name or "sh"
	if !strings.Contains(string(content), "sh") {
		t.Errorf("expected log to contain shell name, got: %s", content)
	}
}

func TestManagerStopAllConcurrent(t *testing.T) {
	// Setup
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

	if err := logs.Setup(); err != nil {
		t.Fatalf("failed to setup logs: %v", err)
	}

	manager := NewManager()

	// Start multiple daemons
	for i := 0; i < 5; i++ {
		taskName := fmt.Sprintf("concurrent-daemon-%d", i)
		logPath := logs.GetLogPath(taskName)
		if err := manager.Start(taskName, "test-session-id", "sleep 10", nil, "", logPath, ""); err != nil {
			t.Fatalf("failed to start daemon %s: %v", taskName, err)
		}
	}

	// StopAll must not deadlock or race even with the background monitoring goroutines
	if err := manager.StopAll(); err != nil {
		t.Fatalf("StopAll failed: %v", err)
	}

	// Verify all stopped
	for i := 0; i < 5; i++ {
		taskName := fmt.Sprintf("concurrent-daemon-%d", i)
		running, _, err := manager.Status(taskName)
		if err != nil {
			t.Fatalf("failed to get status for %s: %v", taskName, err)
		}
		if running {
			t.Errorf("expected daemon %s to be stopped after StopAll", taskName)
		}
	}
}

// TestNewManagerPreservesDaemonsAcrossInvocations proves that creating a new
// Manager (what every CLI subcommand does via bootstrap) must NOT kill daemons
// that are still running. Before the fix, NewManager called restoreFromPIDFiles
// which SIGKILLed every process it found in the pids directory, so
// `runbook status api` would kill the daemon it was supposed to report on.
func TestNewManagerPreservesDaemonsAcrossInvocations(t *testing.T) {
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
	if err := logs.Setup(); err != nil {
		t.Fatalf("failed to setup logs: %v", err)
	}

	// m1 simulates `runbook start api`
	m1 := NewManager()
	logPath := logs.GetLogPath("api")
	if err := m1.Start("api", "sess-1", "sleep 30", nil, "", logPath, ""); err != nil {
		t.Fatalf("start: %v", err)
	}
	_, pid, _ := m1.Status("api")
	if pid == 0 {
		t.Fatal("daemon did not start")
	}

	// m2 simulates `runbook status api` — a brand-new process.
	// The daemon must still be alive; m2 must report it as running.
	m2 := NewManager()

	if !isProcessAlive(pid) {
		t.Errorf("BUG: new Manager killed daemon PID %d (was alive before status call)", pid)
	}

	running, pid2, _ := m2.Status("api")
	if !running {
		t.Errorf("BUG: new Manager reports daemon as STOPPED; it should be RUNNING")
	}
	if pid2 != pid {
		t.Errorf("BUG: PID mismatch after restore: got %d, want %d", pid2, pid)
	}

	// m3 simulates `runbook stop api` from yet another standalone invocation.
	// It does not own the daemon, so stop must be rejected.
	m3 := NewManager()
	if err := m3.Stop("api"); err == nil {
		t.Error("non-owner Stop should have returned an error")
	}
	if !isProcessAlive(pid) {
		t.Errorf("daemon PID %d was killed by a non-owner Manager", pid)
	}

	// Only m1 (the owner) can stop it.
	if err := m1.Stop("api"); err != nil {
		t.Errorf("owner Manager failed to stop daemon: %v", err)
	}
	if isProcessAlive(pid) {
		t.Errorf("daemon PID %d still alive after owner stopped it", pid)
	}
}

// TestPIDFileRestoreAcrossManagerInstances verifies that stale PID files for
// dead processes are cleaned up on startup, and that the PID file for a still-
// running daemon is preserved so subsequent Managers can find it.
func TestPIDFileRestoreAcrossManagerInstances(t *testing.T) {
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

	if err := logs.Setup(); err != nil {
		t.Fatalf("failed to setup logs: %v", err)
	}

	// Manager 1: start a daemon.
	m1 := NewManager()
	logPath := logs.GetLogPath("persist-daemon")
	if err := m1.Start("persist-daemon", "test-session-id", "sleep 30", nil, "", logPath, ""); err != nil {
		t.Fatalf("failed to start daemon: %v", err)
	}

	running, pid, err := m1.Status("persist-daemon")
	if err != nil || !running || pid == 0 {
		t.Fatalf("expected running daemon, got running=%v pid=%d err=%v", running, pid, err)
	}
	t.Logf("daemon PID: %d", pid)

	// Manager 2: simulates a new CLI invocation (e.g. `runbook status`).
	// The daemon must still be alive and visible as running.
	m2 := NewManager()

	if !isProcessAlive(pid) {
		t.Errorf("new Manager killed daemon PID %d — it should have been restored", pid)
	}
	running2, _, _ := m2.Status("persist-daemon")
	if !running2 {
		t.Errorf("Manager 2 should see daemon as running after restore")
	}

	// m2 does not own the daemon — stop must be refused.
	if err := m2.Stop("persist-daemon"); err == nil {
		t.Error("non-owner Stop should have returned an error")
	}
	if !isProcessAlive(pid) {
		t.Errorf("daemon PID %d was killed by non-owner m2", pid)
	}

	// Only the owner (m1) can stop it.
	if err := m1.Stop("persist-daemon"); err != nil {
		t.Errorf("owner m1 failed to stop daemon: %v", err)
	}

	// Manager 3: daemon is gone, PID file should be cleaned up.
	m3 := NewManager()
	running3, _, _ := m3.Status("persist-daemon")
	if running3 {
		t.Errorf("Manager 3 should see clean state with no running daemons")
	}
}

func TestIsProcessAlive(t *testing.T) {
	// Test with own PID (should be alive)
	if !isProcessAlive(os.Getpid()) {
		t.Errorf("expected own process to be alive")
	}

	// Test with unlikely PID (should not be alive)
	if isProcessAlive(999999) {
		t.Errorf("expected unlikely PID to not be alive")
	}
}

// TestOrphanedProcesses verifies that child processes spawned by daemons
// become orphaned when the daemon is stopped.
//
// This test PROVES the bug exists. The test is expected to FAIL until
// the orphan process issue is fixed.
//
// Bug: When a daemon spawns child processes and the daemon is stopped,
// the child processes are not terminated and become orphaned (adopted by init).
func TestOrphanedProcesses(t *testing.T) {
	// Skip on Windows - Unix-specific test
	if os.Getenv("GOOS") == "windows" {
		t.Skip("Unix-specific test")
	}

	// Setup
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

	if err := logs.Setup(); err != nil {
		t.Fatalf("failed to setup logs: %v", err)
	}

	manager := NewManager()
	logPath := logs.GetLogPath("orphan-test")

	// Start daemon that spawns child processes
	// The child processes are backgrounded with & so they outlive the parent
	cmd := `
echo "Parent PID: $$"
sleep 30 &
CHILD1=$!
sleep 30 &
CHILD2=$!
echo "Child PIDs: $CHILD1 $CHILD2"
wait
`

	err = manager.Start("orphan-test", "test-session-id", cmd, nil, "", logPath, "")
	if err != nil {
		t.Fatalf("failed to start daemon: %v", err)
	}

	// Get daemon PID
	info, err := manager.GetProcessInfo("orphan-test")
	if err != nil {
		t.Fatalf("failed to get process info: %v", err)
	}
	daemonPID := info.PID

	// Wait for child processes to spawn
	time.Sleep(500 * time.Millisecond)

	// Find child process PIDs using ps
	// Look for sleep processes that are children of our daemon
	childPIDs := findChildProcesses(t, daemonPID)

	if len(childPIDs) == 0 {
		t.Fatal("expected to find child processes, but found none")
	}

	t.Logf("Daemon PID: %d, Child PIDs: %v", daemonPID, childPIDs)

	// Verify child processes are running
	for _, pid := range childPIDs {
		if !isProcessAlive(pid) {
			t.Errorf("expected child process %d to be alive before stop", pid)
		}
	}

	// Stop the daemon
	err = manager.Stop("orphan-test")
	if err != nil {
		t.Fatalf("failed to stop daemon: %v", err)
	}

	// Wait a bit for cleanup
	time.Sleep(200 * time.Millisecond)

	// Check if daemon process is gone
	if isProcessAlive(daemonPID) {
		t.Errorf("expected daemon process %d to be terminated", daemonPID)
	}

	// Check if child processes are still running (THE BUG)
	orphanedPIDs := []int{}
	for _, pid := range childPIDs {
		if isProcessAlive(pid) {
			orphanedPIDs = append(orphanedPIDs, pid)
		}
	}

	// Clean up any orphaned processes before asserting
	defer func() {
		for _, pid := range orphanedPIDs {
			// Force kill any remaining processes
			_ = killProcess(pid)
		}
	}()

	// THIS IS THE BUG: Child processes should be terminated but they're not
	if len(orphanedPIDs) > 0 {
		t.Errorf("BUG CONFIRMED: Found %d orphaned child process(es): %v",
			len(orphanedPIDs), orphanedPIDs)
		t.Errorf("Child processes should be terminated when daemon stops, but they were orphaned")

		// Provide diagnostic information
		for _, pid := range orphanedPIDs {
			ppid := getParentPID(t, pid)
			t.Logf("Orphaned process %d now has PPID: %d (1 = adopted by init)", pid, ppid)
		}
	}
}

// findChildProcesses returns PIDs of child processes of the given parent PID
func findChildProcesses(t *testing.T, parentPID int) []int {
	t.Helper()

	// Use ps to find child processes
	// Format: PID PPID COMMAND
	output, err := execCommand("ps", "-eo", "pid,ppid,command")
	if err != nil {
		t.Fatalf("failed to run ps command: %v", err)
	}

	var childPIDs []int
	lines := strings.Split(string(output), "\n")

	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		// Parse PID and PPID
		var pid, ppid int
		_, err1 := fmt.Sscanf(fields[0], "%d", &pid)
		_, err2 := fmt.Sscanf(fields[1], "%d", &ppid)

		if err1 != nil || err2 != nil {
			continue
		}

		// Check if this is a child of our daemon and is a sleep process
		if ppid == parentPID && strings.Contains(line, "sleep") {
			childPIDs = append(childPIDs, pid)
		}
	}

	return childPIDs
}

// getParentPID returns the parent PID of the given process
func getParentPID(t *testing.T, pid int) int {
	t.Helper()

	output, err := execCommand("ps", "-o", "ppid=", "-p", fmt.Sprintf("%d", pid))
	if err != nil {
		t.Logf("failed to get parent PID for %d: %v", pid, err)
		return -1
	}

	var ppid int
	_, err = fmt.Sscanf(strings.TrimSpace(string(output)), "%d", &ppid)
	if err != nil {
		t.Logf("failed to parse parent PID: %v", err)
		return -1
	}

	return ppid
}

// execCommand is a helper to execute shell commands
func execCommand(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	return cmd.Output()
}

// killProcess forcefully terminates a process
func killProcess(pid int) error {
	cmd := exec.Command("kill", "-9", fmt.Sprintf("%d", pid))
	return cmd.Run()
}

// TestOwnershipStopRejected validates that a Manager cannot stop a daemon it
// did not start, but can still observe its status and it remains running.
func TestOwnershipStopRejected(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	if err := logs.Setup(); err != nil {
		t.Fatalf("logs setup: %v", err)
	}

	owner := NewManager()
	logPath := logs.GetLogPath("svc")
	if err := owner.Start("svc", "sess-owner", "sleep 30", nil, "", logPath, ""); err != nil {
		t.Fatalf("start: %v", err)
	}
	_, pid, _ := owner.Status("svc")

	// Non-owner tries to stop — must be rejected and daemon must stay alive.
	other := NewManager()
	if err := other.Stop("svc"); err == nil {
		t.Error("non-owner Stop should have returned an error")
	}
	if !isProcessAlive(pid) {
		t.Errorf("daemon PID %d killed by non-owner", pid)
	}

	// Non-owner can still read status.
	running, _, _ := other.Status("svc")
	if !running {
		t.Error("non-owner should be able to observe status of running daemon")
	}

	// Owner can stop it.
	if err := owner.Stop("svc"); err != nil {
		t.Errorf("owner Stop failed: %v", err)
	}
	if isProcessAlive(pid) {
		t.Errorf("daemon PID %d still alive after owner stopped it", pid)
	}
}

// TestOrphanAdoption verifies that when the process that started a daemon is
// dead (e.g. SIGKILL'd), a new Manager adopts the orphaned daemon and can
// stop it. This covers the real-world case where `runbook start api` exits
// normally (making the daemon orphaned from the start) and any subsequent
// `runbook stop api` invocation must be able to stop it.
func TestOrphanAdoption(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	if err := logs.Setup(); err != nil {
		t.Fatalf("logs setup: %v", err)
	}

	// Start a daemon with m1.
	m1 := NewManager()
	logPath := logs.GetLogPath("orphan-svc")
	if err := m1.Start("orphan-svc", "sess-orphan", "sleep 30", nil, "", logPath, ""); err != nil {
		t.Fatalf("start: %v", err)
	}
	_, pid, _ := m1.Status("orphan-svc")
	if pid == 0 {
		t.Fatal("daemon did not start")
	}

	// Simulate the owning process being dead by overwriting the PID file
	// with a dead OwnerPID. PID 1 is always alive (launchd/init), so we use
	// an unlikely PID instead. Using the daemon's own PID as a dead owner
	// would conflict, so use a clearly-dead PID.
	deadOwnerPID := 999997 // almost certainly dead
	if err := writePIDFile(pidFileData{
		PID:       pid,
		OwnerID:   "dead-owner-uuid",
		OwnerPID:  deadOwnerPID,
		SessionID: "sess-orphan",
		TaskName:  "orphan-svc",
		StartTime: time.Now(),
		LogFile:   logPath,
	}); err != nil {
		t.Fatalf("write fake PID file: %v", err)
	}

	// m2 simulates a fresh CLI invocation after the original owner is dead.
	// It must adopt the orphan and be able to stop it.
	m2 := NewManager()

	// Daemon must still be alive (restore doesn't kill it).
	if !isProcessAlive(pid) {
		t.Fatalf("daemon PID %d was killed during restore — orphan was not adopted, it was killed", pid)
	}

	// m2 must report it as running.
	running, pid2, _ := m2.Status("orphan-svc")
	if !running {
		t.Error("m2 should see the orphaned daemon as running")
	}
	if pid2 != pid {
		t.Errorf("PID mismatch: got %d, want %d", pid2, pid)
	}

	// m2 must be able to stop the adopted orphan.
	if err := m2.Stop("orphan-svc"); err != nil {
		t.Errorf("m2 failed to stop adopted orphan: %v", err)
	}
	if isProcessAlive(pid) {
		t.Errorf("daemon PID %d still alive after adopted stop", pid)
	}
}

// TestStopAllOnlyKillsOwnedDaemons ensures StopAll does not touch daemons
// owned by other Manager instances.
func TestStopAllOnlyKillsOwnedDaemons(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	if err := logs.Setup(); err != nil {
		t.Fatalf("logs setup: %v", err)
	}

	// m1 starts daemon-a.
	m1 := NewManager()
	logA := logs.GetLogPath("daemon-a")
	if err := m1.Start("daemon-a", "sess-a", "sleep 30", nil, "", logA, ""); err != nil {
		t.Fatalf("start daemon-a: %v", err)
	}
	_, pidA, _ := m1.Status("daemon-a")

	// m2 starts daemon-b and then calls StopAll.
	m2 := NewManager()
	logB := logs.GetLogPath("daemon-b")
	if err := m2.Start("daemon-b", "sess-b", "sleep 30", nil, "", logB, ""); err != nil {
		t.Fatalf("start daemon-b: %v", err)
	}
	_, pidB, _ := m2.Status("daemon-b")

	if err := m2.StopAll(); err != nil {
		t.Fatalf("StopAll: %v", err)
	}

	// daemon-b (owned by m2) must be dead.
	if isProcessAlive(pidB) {
		t.Errorf("daemon-b PID %d still alive after StopAll by its owner", pidB)
	}

	// daemon-a (owned by m1) must still be running.
	if !isProcessAlive(pidA) {
		t.Errorf("daemon-a PID %d was killed by a StopAll from a non-owner", pidA)
	}

	// Cleanup.
	_ = m1.Stop("daemon-a")
}
