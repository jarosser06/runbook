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

func TestPIDFileRestoreAcrossManagerInstances(t *testing.T) {
	// Simulates a server being killed with SIGKILL (leaving orphaned daemons +
	// PID files on disk). The next manager startup must kill those orphans and
	// remove their PID files so the system starts with a clean slate.
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

	// Manager 1: start the daemon (simulates previous server instance)
	m1 := NewManager()
	logPath := logs.GetLogPath("persist-daemon")
	if err := m1.Start("persist-daemon", "test-session-id", "sleep 30", nil, "", logPath, ""); err != nil {
		t.Fatalf("failed to start daemon: %v", err)
	}

	running, pid, err := m1.Status("persist-daemon")
	if err != nil || !running || pid == 0 {
		t.Fatalf("expected running daemon, got running=%v pid=%d err=%v", running, pid, err)
	}
	t.Logf("Orphaned daemon PID: %d", pid)

	// Manager 2: simulates new server startup after previous was SIGKILL'd.
	// Must kill the orphaned process and remove PID files.
	m2 := NewManager()

	// Orphan should be dead
	if isProcessAlive(pid) {
		t.Errorf("expected orphaned daemon PID %d to be killed by new manager startup", pid)
	}

	// Manager 2 should not see the daemon as running
	running2, _, _ := m2.Status("persist-daemon")
	if running2 {
		t.Errorf("Manager 2 should not see orphaned daemon as running after cleanup")
	}

	// Manager 3: PID file should be gone, clean state
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
