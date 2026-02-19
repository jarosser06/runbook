package logs

import (
	"fmt"
	"os"
	"path/filepath"

	"runbookmcp.dev/internal/dirs"
)

const (
	// LogDir is the directory where all logs are stored
	LogDir = dirs.StateDir + "/logs"
	// MaxLogSize is the maximum size of a log file before rotation (10MB)
	MaxLogSize = 10 * 1024 * 1024
)

// Setup initializes the log directory structure
// Creates the log directory and a .gitignore file to ignore logs
func Setup() error {
	// Create the log directory
	if err := os.MkdirAll(LogDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Create sessions subdirectory
	sessionsDir := filepath.Join(LogDir, "sessions")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		return fmt.Errorf("failed to create sessions directory: %w", err)
	}

	// Create latest subdirectory
	latestDir := filepath.Join(LogDir, "latest")
	if err := os.MkdirAll(latestDir, 0755); err != nil {
		return fmt.Errorf("failed to create latest directory: %w", err)
	}

	// Create parent directory for gitignore
	devToolsDir := filepath.Dir(LogDir)
	gitignorePath := filepath.Join(devToolsDir, ".gitignore")

	// Check if .gitignore already exists
	if _, err := os.Stat(gitignorePath); os.IsNotExist(err) {
		// Ignore everything in the state directory — logs, pids, server registry, etc.
		content := "*\n!.gitignore\n"
		if err := os.WriteFile(gitignorePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to create .gitignore: %w", err)
		}
	}

	return nil
}

// GetLogPath returns the full path for a task's log file
func GetLogPath(taskName string) string {
	return filepath.Join(LogDir, taskName+".log")
}

// GetRotatedLogPath returns the path for a rotated log file with timestamp
func GetRotatedLogPath(taskName string, timestamp int64) string {
	return filepath.Join(LogDir, fmt.Sprintf("%s.log.%d", taskName, timestamp))
}
