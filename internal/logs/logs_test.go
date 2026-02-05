package logs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSetup(t *testing.T) {
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

	// Run Setup
	if err := Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Verify log directory was created
	if _, err := os.Stat(LogDir); os.IsNotExist(err) {
		t.Errorf("log directory was not created")
	}

	// Verify .gitignore was created
	gitignorePath := filepath.Join(filepath.Dir(LogDir), ".gitignore")
	content, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Errorf("failed to read .gitignore: %v", err)
	}

	if !strings.Contains(string(content), "logs/") {
		t.Errorf("expected .gitignore to contain 'logs/', got: %s", content)
	}

	// Run Setup again to ensure it's idempotent
	if err := Setup(); err != nil {
		t.Errorf("Setup should be idempotent, but failed on second run: %v", err)
	}
}

func TestGetLogPath(t *testing.T) {
	expected := filepath.Join(LogDir, "test-task.log")
	actual := GetLogPath("test-task")
	if actual != expected {
		t.Errorf("expected %s, got %s", expected, actual)
	}
}

func TestGetRotatedLogPath(t *testing.T) {
	timestamp := int64(1234567890)
	expected := filepath.Join(LogDir, "test-task.log.1234567890")
	actual := GetRotatedLogPath("test-task", timestamp)
	if actual != expected {
		t.Errorf("expected %s, got %s", expected, actual)
	}
}

func TestNewWriter(t *testing.T) {
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

	// Setup log directory
	if err := Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Create session metadata
	sessionID := GenerateSessionID()
	metadata := &SessionMetadata{
		SessionID: sessionID,
		TaskName:  "test-task",
		TaskType:  "oneshot",
		StartTime: time.Now(),
	}

	// Create writer
	writer, err := NewWriter(sessionID, metadata)
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}
	defer writer.Close()

	// Verify log file was created
	if _, err := os.Stat(writer.logPath); os.IsNotExist(err) {
		t.Errorf("log file was not created")
	}
}

func TestWriterWrite(t *testing.T) {
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

	// Setup log directory
	if err := Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Create session metadata
	sessionID := GenerateSessionID()
	metadata := &SessionMetadata{
		SessionID: sessionID,
		TaskName:  "test-task",
		TaskType:  "oneshot",
		StartTime: time.Now(),
	}

	// Create writer
	writer, err := NewWriter(sessionID, metadata)
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}
	defer writer.Close()

	// Write some data
	testData := "test log line\n"
	n, err := writer.Write([]byte(testData))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	if n != len(testData) {
		t.Errorf("expected to write %d bytes, wrote %d", len(testData), n)
	}

	// Close writer to flush
	if err := writer.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Read back the log file
	content, err := os.ReadFile(writer.logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	if string(content) != testData {
		t.Errorf("expected log content %q, got %q", testData, content)
	}
}

func TestLogRotation(t *testing.T) {
	// Note: Log rotation is no longer applicable with session-based logging
	// Each session has its own directory, so logs are naturally bounded
	// This test is kept for backward compatibility but tests session isolation instead

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

	// Setup log directory
	if err := Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Create two separate sessions
	sessionID1 := GenerateSessionID()
	metadata1 := &SessionMetadata{
		SessionID: sessionID1,
		TaskName:  "test-task",
		TaskType:  "oneshot",
		StartTime: time.Now(),
	}

	writer1, err := NewWriter(sessionID1, metadata1)
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}
	_, _ = writer1.Write([]byte("session 1 data"))
	writer1.Close()

	sessionID2 := GenerateSessionID()
	metadata2 := &SessionMetadata{
		SessionID: sessionID2,
		TaskName:  "test-task",
		TaskType:  "oneshot",
		StartTime: time.Now(),
	}

	writer2, err := NewWriter(sessionID2, metadata2)
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}
	_, _ = writer2.Write([]byte("session 2 data"))
	writer2.Close()

	// Verify both session directories exist
	sessionsDir := filepath.Join(LogDir, "sessions")
	sessions, err := os.ReadDir(sessionsDir)
	if err != nil {
		t.Fatalf("failed to read sessions directory: %v", err)
	}

	if len(sessions) < 2 {
		t.Errorf("expected at least 2 session directories, found %d", len(sessions))
	}
}

func TestReadLog(t *testing.T) {
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

	// Setup log directory
	if err := Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Create log file with test data
	logPath := GetLogPath("test-task")
	testLines := []string{
		"line 1",
		"line 2",
		"line 3",
		"error occurred",
		"line 5",
	}
	content := strings.Join(testLines, "\n")
	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create log file: %v", err)
	}

	// Test reading all lines
	lines, err := ReadLog("test-task", ReadOptions{})
	if err != nil {
		t.Fatalf("ReadLog failed: %v", err)
	}

	if len(lines) != len(testLines) {
		t.Errorf("expected %d lines, got %d", len(testLines), len(lines))
	}

	// Test tailing
	lines, err = ReadLog("test-task", ReadOptions{Lines: 2})
	if err != nil {
		t.Fatalf("ReadLog with tail failed: %v", err)
	}

	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d", len(lines))
	}

	if lines[0] != "error occurred" || lines[1] != "line 5" {
		t.Errorf("expected last 2 lines, got: %v", lines)
	}

	// Test filtering
	lines, err = ReadLog("test-task", ReadOptions{Filter: "error"})
	if err != nil {
		t.Fatalf("ReadLog with filter failed: %v", err)
	}

	if len(lines) != 1 {
		t.Errorf("expected 1 line with 'error', got %d", len(lines))
	}

	if lines[0] != "error occurred" {
		t.Errorf("expected 'error occurred', got: %s", lines[0])
	}
}

func TestTailLog(t *testing.T) {
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

	// Setup log directory
	if err := Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Create log file with test data
	logPath := GetLogPath("test-task")
	testLines := []string{"line 1", "line 2", "line 3"}
	content := strings.Join(testLines, "\n")
	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create log file: %v", err)
	}

	// Test tailing
	lines, err := TailLog("test-task", 2)
	if err != nil {
		t.Fatalf("TailLog failed: %v", err)
	}

	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d", len(lines))
	}
}

func TestFilterLog(t *testing.T) {
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

	// Setup log directory
	if err := Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Create log file with test data
	logPath := GetLogPath("test-task")
	testLines := []string{"INFO: starting", "ERROR: failed", "INFO: done"}
	content := strings.Join(testLines, "\n")
	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create log file: %v", err)
	}

	// Test filtering
	lines, err := FilterLog("test-task", "ERROR")
	if err != nil {
		t.Fatalf("FilterLog failed: %v", err)
	}

	if len(lines) != 1 {
		t.Errorf("expected 1 line with 'ERROR', got %d", len(lines))
	}

	if lines[0] != "ERROR: failed" {
		t.Errorf("expected 'ERROR: failed', got: %s", lines[0])
	}
}

func TestReadLogNonExistent(t *testing.T) {
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

	// Setup log directory
	if err := Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Read non-existent log
	lines, err := ReadLog("nonexistent", ReadOptions{})
	if err != nil {
		t.Fatalf("ReadLog should not fail for non-existent log: %v", err)
	}

	if len(lines) != 0 {
		t.Errorf("expected 0 lines for non-existent log, got %d", len(lines))
	}
}

func TestFilterLinesInvalidRegex(t *testing.T) {
	lines := []string{"test"}
	_, err := filterLines(lines, "[invalid")
	if err == nil {
		t.Errorf("expected error for invalid regex, got nil")
	}
}
