package logs

import (
	"fmt"
	"io"
	"os"
	"time"
)

// Writer handles writing logs to both a file and stdout
type Writer struct {
	sessionID string
	metadata  *SessionMetadata
	file      *os.File
	logPath   string
}

// NewWriter creates a new log writer for a session
// It creates the session directory, opens the log file, and writes initial metadata
func NewWriter(sessionID string, metadata *SessionMetadata) (*Writer, error) {
	// Create session directory
	if err := CreateSessionDirectory(sessionID); err != nil {
		return nil, fmt.Errorf("failed to create session directory: %w", err)
	}

	// Get log path
	logPath := GetSessionLogPath(sessionID)

	// Open log file for writing
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	// Write initial metadata
	if err := WriteSessionMetadata(sessionID, metadata); err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to write initial metadata: %w", err)
	}

	// Create latest symlink
	if err := CreateLatestLink(metadata.TaskName, sessionID); err != nil {
		// Non-fatal error - log but continue
		fmt.Fprintf(os.Stderr, "Warning: failed to create latest symlink: %v\n", err)
	}

	return &Writer{
		sessionID: sessionID,
		metadata:  metadata,
		file:      file,
		logPath:   logPath,
	}, nil
}

// Write writes data to the log file
func (w *Writer) Write(p []byte) (n int, err error) {
	// Write to file
	return w.file.Write(p)
}

// Close closes the log file and updates session metadata with completion info
func (w *Writer) Close() error {
	if w.file != nil {
		if err := w.file.Close(); err != nil {
			return err
		}
	}

	// Update metadata with end time and duration
	endTime := time.Now()
	duration := endTime.Sub(w.metadata.StartTime)

	updates := map[string]interface{}{
		"end_time": endTime,
		"duration": duration,
	}

	// Only update exit code and success if they were set in the metadata
	if w.metadata.ExitCode != nil {
		updates["exit_code"] = *w.metadata.ExitCode
	}
	if w.metadata.Success != nil {
		updates["success"] = *w.metadata.Success
	}
	if w.metadata.TimedOut {
		updates["timed_out"] = w.metadata.TimedOut
	}

	if err := UpdateSessionMetadata(w.sessionID, updates); err != nil {
		// Non-fatal error - log but don't fail the close
		fmt.Fprintf(os.Stderr, "Warning: failed to update session metadata: %v\n", err)
	}

	return nil
}

// MultiWriter creates a writer that writes to both the log file and stdout
func (w *Writer) MultiWriter() io.Writer {
	return io.MultiWriter(w.file, os.Stdout)
}

// UpdateMetadata updates the metadata stored in the writer
// This is used to update fields like ExitCode, Success, and TimedOut during execution
func (w *Writer) UpdateMetadata(updates map[string]interface{}) {
	if exitCode, ok := updates["exit_code"].(int); ok {
		w.metadata.ExitCode = &exitCode
	}
	if success, ok := updates["success"].(bool); ok {
		w.metadata.Success = &success
	}
	if timedOut, ok := updates["timed_out"].(bool); ok {
		w.metadata.TimedOut = timedOut
	}
}

// GetSessionID returns the session ID
func (w *Writer) GetSessionID() string {
	return w.sessionID
}

// GetLogPath returns the log file path
func (w *Writer) GetLogPath() string {
	return w.logPath
}
