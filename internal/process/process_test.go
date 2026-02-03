package process

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jarosser06/dev-toolkit-mcp/internal/logs"
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
	err = manager.Start("test-daemon", "test-session-id", "sleep 10", nil, "", logPath)
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
	err = manager.Start("test-daemon", "test-session-id", "sleep 10", nil, "", logPath)
	if err != nil {
		t.Fatalf("failed to start daemon: %v", err)
	}
	defer func() {
		_ = manager.Stop("test-daemon") // Cleanup, ignore errors
	}()

	// Try to start again
	err = manager.Start("test-daemon", "test-session-id", "sleep 10", nil, "", logPath)
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
		err := manager.Start(taskName, "test-session-id", "sleep 10", nil, "", logPath)
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
	err = manager.Start("test-daemon", "test-session-id", "echo 'hello'", nil, "", logPath)
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
	err = manager.Start("test-daemon", "test-session-id", "echo $TEST_VAR", env, "", logPath)
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
	err = manager.Start("test-daemon", "test-session-id", "pwd", nil, testDir, logPath)
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
	err = manager.Start("test-daemon", "test-session-id", "sleep 10", nil, "", logPath)
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
