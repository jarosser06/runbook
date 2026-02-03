package logs

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// SessionRetention defines retention policies for session cleanup
type SessionRetention struct {
	MaxSessions int           // Maximum number of sessions to keep per task (0 = unlimited)
	MaxAge      time.Duration // Maximum age of sessions to keep (0 = unlimited)
}

// DefaultRetention provides default retention policy
var DefaultRetention = SessionRetention{
	MaxSessions: 100,
	MaxAge:      7 * 24 * time.Hour, // 7 days
}

// CleanupOldSessions removes old sessions according to the retention policy
// Returns the number of sessions deleted and any error
func CleanupOldSessions(taskName string, retention SessionRetention) (int, error) {
	// Get all sessions for the task
	sessions, err := ListSessions(taskName, 0) // Get all sessions
	if err != nil {
		return 0, fmt.Errorf("failed to list sessions: %w", err)
	}

	if len(sessions) == 0 {
		return 0, nil
	}

	// Sort sessions by start time (oldest first)
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].StartTime.Before(sessions[j].StartTime)
	})

	var toDelete []string
	now := time.Now()

	// Apply age-based retention
	if retention.MaxAge > 0 {
		for _, session := range sessions {
			if now.Sub(session.StartTime) > retention.MaxAge {
				toDelete = append(toDelete, session.SessionID)
			}
		}
	}

	// Apply count-based retention
	if retention.MaxSessions > 0 && len(sessions) > retention.MaxSessions {
		// Keep the most recent MaxSessions, delete the rest
		numToDelete := len(sessions) - retention.MaxSessions
		for i := 0; i < numToDelete; i++ {
			sessionID := sessions[i].SessionID
			// Only add if not already in toDelete list
			found := false
			for _, id := range toDelete {
				if id == sessionID {
					found = true
					break
				}
			}
			if !found {
				toDelete = append(toDelete, sessionID)
			}
		}
	}

	// Delete sessions
	deleted := 0
	for _, sessionID := range toDelete {
		if err := deleteSession(sessionID); err != nil {
			// Log error but continue with other sessions
			fmt.Fprintf(os.Stderr, "Warning: failed to delete session %s: %v\n", sessionID, err)
		} else {
			deleted++
		}
	}

	return deleted, nil
}

// CleanupAllSessions cleans up sessions for all tasks according to the retention policy
func CleanupAllSessions(retention SessionRetention) (int, error) {
	sessionsDir := filepath.Join(LogDir, "sessions")

	// Read all session directories
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to read sessions directory: %w", err)
	}

	// Collect all unique task names
	taskNames := make(map[string]bool)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		sessionID := entry.Name()
		metadata, err := ReadSessionMetadata(sessionID)
		if err != nil {
			// Skip sessions with missing or invalid metadata
			continue
		}

		taskNames[metadata.TaskName] = true
	}

	// Clean up sessions for each task
	totalDeleted := 0
	for taskName := range taskNames {
		deleted, err := CleanupOldSessions(taskName, retention)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to cleanup sessions for task %s: %v\n", taskName, err)
		}
		totalDeleted += deleted
	}

	return totalDeleted, nil
}

// deleteSession deletes a session directory and all its contents
func deleteSession(sessionID string) error {
	sessionDir := GetSessionDirectory(sessionID)
	return os.RemoveAll(sessionDir)
}
