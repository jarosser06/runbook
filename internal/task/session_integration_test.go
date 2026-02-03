package task

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jarosser06/dev-toolkit-mcp/internal/config"
	"github.com/jarosser06/dev-toolkit-mcp/internal/logs"
)

// TestSessionBasedLoggingIntegration tests the complete session-based logging system end-to-end
func TestSessionBasedLoggingIntegration(t *testing.T) {
	// Create temporary directory for test
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

	// Setup logs
	if err := logs.Setup(); err != nil {
		t.Fatalf("Failed to setup logs: %v", err)
	}

	// Verify sessions and latest directories exist
	sessionsDir := filepath.Join(logs.LogDir, "sessions")
	if _, err := os.Stat(sessionsDir); os.IsNotExist(err) {
		t.Fatalf("sessions directory was not created")
	}

	latestDir := filepath.Join(logs.LogDir, "latest")
	if _, err := os.Stat(latestDir); os.IsNotExist(err) {
		t.Fatalf("latest directory was not created")
	}

	// Create a simple manifest
	manifest := &config.Manifest{
		Version: "1.0",
		Tasks: map[string]config.Task{
			"test_session": {
				Description: "Test task for session logging",
				Command:     "echo 'Session integration test successful!'",
				Type:        config.TaskTypeOneShot,
			},
		},
	}

	// Execute the task
	executor := NewExecutor(manifest)
	result, err := executor.Execute("test_session", map[string]interface{}{})
	if err != nil {
		t.Fatalf("Failed to execute task: %v", err)
	}

	// Verify result has session ID
	if result.SessionID == "" {
		t.Fatalf("SessionID is empty in execution result")
	}
	t.Logf("SessionID: %s", result.SessionID)

	// Verify result indicates success
	if !result.Success {
		t.Fatalf("Task execution failed: %s", result.Error)
	}

	// Verify exit code is 0
	if result.ExitCode != 0 {
		t.Fatalf("Expected exit code 0, got %d", result.ExitCode)
	}

	// Verify log path is session-based
	expectedLogPath := logs.GetSessionLogPath(result.SessionID)
	if result.LogPath != expectedLogPath {
		t.Errorf("Expected log path %s, got %s", expectedLogPath, result.LogPath)
	}

	// Verify session directory exists
	sessionDir := logs.GetSessionDirectory(result.SessionID)
	if _, err := os.Stat(sessionDir); os.IsNotExist(err) {
		t.Fatalf("Session directory not created: %s", sessionDir)
	}
	t.Logf("✓ Session directory created: %s", sessionDir)

	// Verify metadata file exists
	metadataPath := logs.GetSessionMetadataPath(result.SessionID)
	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		t.Fatalf("Metadata file not created: %s", metadataPath)
	}
	t.Logf("✓ Metadata file created: %s", metadataPath)

	// Read and verify metadata
	metadata, err := logs.ReadSessionMetadata(result.SessionID)
	if err != nil {
		t.Fatalf("Failed to read metadata: %v", err)
	}

	// Verify metadata fields
	if metadata.SessionID != result.SessionID {
		t.Errorf("Metadata SessionID mismatch: expected %s, got %s", result.SessionID, metadata.SessionID)
	}
	if metadata.TaskName != "test_session" {
		t.Errorf("Metadata TaskName mismatch: expected test_session, got %s", metadata.TaskName)
	}
	if metadata.TaskType != "oneshot" {
		t.Errorf("Metadata TaskType mismatch: expected oneshot, got %s", metadata.TaskType)
	}
	if metadata.EndTime == nil {
		t.Error("Metadata EndTime is nil")
	}
	if metadata.Duration == nil {
		t.Error("Metadata Duration is nil")
	}
	if metadata.ExitCode == nil || *metadata.ExitCode != 0 {
		t.Errorf("Metadata ExitCode is not 0")
	}
	if metadata.Success == nil || !*metadata.Success {
		t.Error("Metadata Success is not true")
	}
	if metadata.TimedOut {
		t.Error("Metadata TimedOut should be false")
	}
	t.Logf("✓ Metadata validated successfully")

	// Verify log file exists and has content
	logPath := logs.GetSessionLogPath(result.SessionID)
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Fatalf("Log file not created: %s", logPath)
	}

	logContent, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}
	if len(logContent) == 0 {
		t.Error("Log file is empty")
	}
	t.Logf("✓ Log file created with content (%d bytes)", len(logContent))

	// Verify latest symlink exists
	latestLink := logs.GetLatestSymlinkPath("test_session")
	if _, err := os.Lstat(latestLink); os.IsNotExist(err) {
		t.Fatalf("Latest symlink not created: %s", latestLink)
	}

	// Verify symlink points to the session
	target, err := os.Readlink(latestLink)
	if err != nil {
		t.Fatalf("Failed to read symlink: %v", err)
	}
	expectedTarget := filepath.Join("..", "..", "sessions", result.SessionID)
	if target != expectedTarget {
		t.Errorf("Symlink target mismatch: expected %s, got %s", expectedTarget, target)
	}
	t.Logf("✓ Latest symlink created and validated")

	// Test ListSessions
	sessions, err := logs.ListSessions("test_session", 10)
	if err != nil {
		t.Fatalf("Failed to list sessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Errorf("Expected 1 session, got %d", len(sessions))
	}
	if sessions[0].SessionID != result.SessionID {
		t.Errorf("Listed session ID mismatch: expected %s, got %s", result.SessionID, sessions[0].SessionID)
	}
	t.Logf("✓ ListSessions returned %d session(s)", len(sessions))

	// Test reading logs through session
	lines, err := logs.ReadSessionLog(result.SessionID, logs.ReadOptions{})
	if err != nil {
		t.Fatalf("Failed to read session log: %v", err)
	}
	if len(lines) == 0 {
		t.Error("ReadSessionLog returned no lines")
	}
	t.Logf("✓ ReadSessionLog returned %d line(s)", len(lines))

	// Test reading latest logs
	latestLines, err := logs.ReadLog("test_session", logs.ReadOptions{})
	if err != nil {
		t.Fatalf("Failed to read latest log: %v", err)
	}
	if len(latestLines) == 0 {
		t.Error("ReadLog returned no lines")
	}
	t.Logf("✓ ReadLog (latest) returned %d line(s)", len(latestLines))

	// Execute another task to test multiple sessions
	result2, err := executor.Execute("test_session", map[string]interface{}{})
	if err != nil {
		t.Fatalf("Failed to execute second task: %v", err)
	}

	// Verify different session ID
	if result2.SessionID == result.SessionID {
		t.Error("Second execution has same SessionID as first")
	}
	t.Logf("✓ Second execution got unique SessionID: %s", result2.SessionID)

	// Verify ListSessions now returns 2 sessions
	sessions, err = logs.ListSessions("test_session", 10)
	if err != nil {
		t.Fatalf("Failed to list sessions after second execution: %v", err)
	}
	if len(sessions) != 2 {
		t.Errorf("Expected 2 sessions, got %d", len(sessions))
	}
	t.Logf("✓ ListSessions now returns %d sessions", len(sessions))

	// Verify sessions are sorted by start time (newest first)
	if sessions[0].StartTime.Before(sessions[1].StartTime) {
		t.Error("Sessions not sorted by start time (newest first)")
	}
	t.Logf("✓ Sessions are sorted correctly")

	// Verify latest symlink was updated
	target, err = os.Readlink(latestLink)
	if err != nil {
		t.Fatalf("Failed to read updated symlink: %v", err)
	}
	expectedTarget = filepath.Join("..", "..", "sessions", result2.SessionID)
	if target != expectedTarget {
		t.Errorf("Symlink not updated: expected %s, got %s", expectedTarget, target)
	}
	t.Logf("✓ Latest symlink updated to newest session")

	t.Log("✅ All session-based logging integration tests passed!")
}

// TestDaemonSessionLogging tests session-based logging for daemon tasks
func TestDaemonSessionLogging(t *testing.T) {
	// Create temporary directory for test
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

	// Setup logs
	if err := logs.Setup(); err != nil {
		t.Fatalf("Failed to setup logs: %v", err)
	}

	// Create manifest with daemon task
	manifest := &config.Manifest{
		Version: "1.0",
		Tasks: map[string]config.Task{
			"test_daemon": {
				Description: "Test daemon for session logging",
				Command:     "sleep 2 && echo 'Daemon test complete'",
				Type:        config.TaskTypeDaemon,
			},
		},
	}

	// Create mock process manager
	pm := NewMockProcessManager()
	manager := NewManager(manifest, pm)

	// Start daemon
	startResult, err := manager.StartDaemon("test_daemon", map[string]interface{}{})
	if err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}

	if !startResult.Success {
		t.Fatalf("Daemon start failed: %s", startResult.Error)
	}

	// Verify session ID is present
	if startResult.SessionID == "" {
		t.Fatal("Session ID is empty in daemon start result")
	}
	t.Logf("Daemon SessionID: %s", startResult.SessionID)

	// Verify log path is session-based
	expectedLogPath := logs.GetSessionLogPath(startResult.SessionID)
	if startResult.LogPath != expectedLogPath {
		t.Errorf("Expected log path %s, got %s", expectedLogPath, startResult.LogPath)
	}
	t.Logf("✓ Daemon log path is session-based: %s", startResult.LogPath)

	// Check daemon status
	status, err := manager.DaemonStatus("test_daemon")
	if err != nil {
		t.Fatalf("Failed to get daemon status: %v", err)
	}

	if !status.Running {
		t.Fatal("Daemon should be running")
	}

	// Verify session ID in status
	if status.SessionID != startResult.SessionID {
		t.Errorf("Status SessionID mismatch: expected %s, got %s", startResult.SessionID, status.SessionID)
	}
	t.Logf("✓ Daemon status includes session ID: %s", status.SessionID)

	// Verify log path in status
	if status.LogPath != startResult.LogPath {
		t.Errorf("Status LogPath mismatch: expected %s, got %s", startResult.LogPath, status.LogPath)
	}
	t.Logf("✓ Daemon status includes correct log path")

	// Stop daemon
	stopResult, err := manager.StopDaemon("test_daemon")
	if err != nil {
		t.Fatalf("Failed to stop daemon: %v", err)
	}

	if !stopResult.Success {
		t.Fatalf("Daemon stop failed: %s", stopResult.Error)
	}
	t.Logf("✓ Daemon stopped successfully")

	t.Log("✅ Daemon session logging tests passed!")
}
