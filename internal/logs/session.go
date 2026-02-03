package logs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/google/uuid"
)

// SessionMetadata holds metadata about a task execution session
type SessionMetadata struct {
	SessionID  string                 `json:"session_id"`
	TaskName   string                 `json:"task_name"`
	TaskType   string                 `json:"task_type"` // "oneshot" or "daemon"
	StartTime  time.Time              `json:"start_time"`
	EndTime    *time.Time             `json:"end_time,omitempty"`
	Duration   *time.Duration         `json:"duration,omitempty"`
	ExitCode   *int                   `json:"exit_code,omitempty"`
	Success    *bool                  `json:"success,omitempty"`
	TimedOut   bool                   `json:"timed_out"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
	Command    string                 `json:"command,omitempty"`
	WorkingDir string                 `json:"working_dir,omitempty"`
}

// SessionInfo holds basic information about a session
type SessionInfo struct {
	SessionID string    `json:"session_id"`
	TaskName  string    `json:"task_name"`
	StartTime time.Time `json:"start_time"`
	LogPath   string    `json:"log_path"`
}

// GenerateSessionID generates a new UUID for a session
func GenerateSessionID() string {
	return uuid.New().String()
}

// GetSessionDirectory returns the directory path for a session
func GetSessionDirectory(sessionID string) string {
	return filepath.Join(LogDir, "sessions", sessionID)
}

// GetSessionLogPath returns the path to the log file for a session
func GetSessionLogPath(sessionID string) string {
	return filepath.Join(GetSessionDirectory(sessionID), "task.log")
}

// GetSessionMetadataPath returns the path to the metadata file for a session
func GetSessionMetadataPath(sessionID string) string {
	return filepath.Join(GetSessionDirectory(sessionID), "metadata.json")
}

// GetLatestSymlinkPath returns the path to the latest symlink for a task
func GetLatestSymlinkPath(taskName string) string {
	return filepath.Join(LogDir, "latest", taskName)
}

// CreateSessionDirectory creates the directory structure for a session
func CreateSessionDirectory(sessionID string) error {
	dir := GetSessionDirectory(sessionID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create session directory: %w", err)
	}
	return nil
}

// WriteSessionMetadata writes session metadata to a JSON file
func WriteSessionMetadata(sessionID string, metadata *SessionMetadata) error {
	path := GetSessionMetadataPath(sessionID)

	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write metadata file: %w", err)
	}

	return nil
}

// ReadSessionMetadata reads session metadata from a JSON file
func ReadSessionMetadata(sessionID string) (*SessionMetadata, error) {
	path := GetSessionMetadataPath(sessionID)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata file: %w", err)
	}

	var metadata SessionMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return &metadata, nil
}

// UpdateSessionMetadata updates specific fields in session metadata
func UpdateSessionMetadata(sessionID string, updates map[string]interface{}) error {
	// Read existing metadata
	metadata, err := ReadSessionMetadata(sessionID)
	if err != nil {
		return fmt.Errorf("failed to read metadata: %w", err)
	}

	// Apply updates
	if endTime, ok := updates["end_time"].(time.Time); ok {
		metadata.EndTime = &endTime
	}
	if duration, ok := updates["duration"].(time.Duration); ok {
		metadata.Duration = &duration
	}
	if exitCode, ok := updates["exit_code"].(int); ok {
		metadata.ExitCode = &exitCode
	}
	if success, ok := updates["success"].(bool); ok {
		metadata.Success = &success
	}
	if timedOut, ok := updates["timed_out"].(bool); ok {
		metadata.TimedOut = timedOut
	}

	// Write updated metadata
	return WriteSessionMetadata(sessionID, metadata)
}

// GetLatestSessionID resolves the latest session ID for a task by reading the symlink
func GetLatestSessionID(taskName string) (string, error) {
	symlinkPath := GetLatestSymlinkPath(taskName)

	// Read the symlink
	target, err := os.Readlink(symlinkPath)
	if err != nil {
		return "", fmt.Errorf("failed to read latest symlink: %w", err)
	}

	// Extract session ID from the target path
	// Target format: ../../sessions/<uuid>
	sessionID := filepath.Base(target)
	return sessionID, nil
}

// CreateLatestLink creates or updates the latest symlink for a task
func CreateLatestLink(taskName, sessionID string) error {
	symlinkPath := GetLatestSymlinkPath(taskName)
	targetPath := filepath.Join("..", "..", "sessions", sessionID)

	// Ensure the latest directory exists
	latestDir := filepath.Join(LogDir, "latest")
	if err := os.MkdirAll(latestDir, 0755); err != nil {
		return fmt.Errorf("failed to create latest directory: %w", err)
	}

	// Remove existing symlink if it exists
	if _, err := os.Lstat(symlinkPath); err == nil {
		if err := os.Remove(symlinkPath); err != nil {
			return fmt.Errorf("failed to remove old symlink: %w", err)
		}
	}

	// Create new symlink
	if err := os.Symlink(targetPath, symlinkPath); err != nil {
		return fmt.Errorf("failed to create symlink: %w", err)
	}

	return nil
}

// ListSessions lists recent sessions for a task, sorted by start time (newest first)
func ListSessions(taskName string, limit int) ([]SessionInfo, error) {
	sessionsDir := filepath.Join(LogDir, "sessions")

	// Read all session directories
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []SessionInfo{}, nil
		}
		return nil, fmt.Errorf("failed to read sessions directory: %w", err)
	}

	var sessions []SessionInfo

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		sessionID := entry.Name()

		// Read metadata
		metadata, err := ReadSessionMetadata(sessionID)
		if err != nil {
			// Skip sessions with missing or invalid metadata
			continue
		}

		// Filter by task name
		if metadata.TaskName != taskName {
			continue
		}

		sessions = append(sessions, SessionInfo{
			SessionID: sessionID,
			TaskName:  metadata.TaskName,
			StartTime: metadata.StartTime,
			LogPath:   GetSessionLogPath(sessionID),
		})
	}

	// Sort by start time (newest first)
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].StartTime.After(sessions[j].StartTime)
	})

	// Apply limit
	if limit > 0 && len(sessions) > limit {
		sessions = sessions[:limit]
	}

	return sessions, nil
}
