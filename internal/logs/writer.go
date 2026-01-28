package logs

import (
	"fmt"
	"io"
	"os"
	"time"
)

// Writer handles writing logs to both a file and stdout
type Writer struct {
	taskName string
	file     *os.File
	logPath  string
}

// NewWriter creates a new log writer for a task
// It opens/creates the log file and sets up dual output
func NewWriter(taskName string) (*Writer, error) {
	logPath := GetLogPath(taskName)

	// Check if rotation is needed before opening
	if err := rotateIfNeeded(taskName, logPath); err != nil {
		return nil, fmt.Errorf("failed to rotate log: %w", err)
	}

	// Open log file for appending
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	return &Writer{
		taskName: taskName,
		file:     file,
		logPath:  logPath,
	}, nil
}

// Write writes data to both the log file and stdout
func (w *Writer) Write(p []byte) (n int, err error) {
	// Write to file
	n, err = w.file.Write(p)
	if err != nil {
		return n, err
	}

	// Check if rotation is needed after write
	if err := rotateIfNeeded(w.taskName, w.logPath); err != nil {
		// Log rotation failure but don't fail the write
		fmt.Fprintf(os.Stderr, "Warning: log rotation failed: %v\n", err)
	}

	return n, nil
}

// Close closes the log file
func (w *Writer) Close() error {
	if w.file != nil {
		return w.file.Close()
	}
	return nil
}

// MultiWriter creates a writer that writes to both the log file and stdout
func (w *Writer) MultiWriter() io.Writer {
	return io.MultiWriter(w.file, os.Stdout)
}

// rotateIfNeeded checks if the log file exceeds MaxLogSize and rotates it if necessary
func rotateIfNeeded(taskName, logPath string) error {
	info, err := os.Stat(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // File doesn't exist yet, no rotation needed
		}
		return fmt.Errorf("failed to stat log file: %w", err)
	}

	if info.Size() >= MaxLogSize {
		return rotateLog(taskName, logPath)
	}

	return nil
}

// rotateLog rotates a log file by renaming it with a timestamp
func rotateLog(taskName, logPath string) error {
	timestamp := time.Now().Unix()
	rotatedPath := GetRotatedLogPath(taskName, timestamp)

	if err := os.Rename(logPath, rotatedPath); err != nil {
		return fmt.Errorf("failed to rename log file: %w", err)
	}

	return nil
}
