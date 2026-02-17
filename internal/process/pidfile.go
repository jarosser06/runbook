package process

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const pidsDir = "._dev_tools/pids"

// pidFileData is what gets persisted to disk for each running daemon.
type pidFileData struct {
	PID       int       `json:"pid"`
	SessionID string    `json:"session_id"`
	TaskName  string    `json:"task_name"`
	StartTime time.Time `json:"start_time"`
	LogFile   string    `json:"log_file"`
}

func pidFilePath(taskName string) string {
	return filepath.Join(pidsDir, taskName+".pid")
}

func writePIDFile(data pidFileData) error {
	if err := os.MkdirAll(pidsDir, 0755); err != nil {
		return fmt.Errorf("failed to create pids directory: %w", err)
	}
	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal PID file: %w", err)
	}
	return os.WriteFile(pidFilePath(data.TaskName), b, 0644)
}

func readPIDFile(taskName string) (*pidFileData, error) {
	b, err := os.ReadFile(pidFilePath(taskName))
	if err != nil {
		return nil, err
	}
	var data pidFileData
	if err := json.Unmarshal(b, &data); err != nil {
		return nil, fmt.Errorf("failed to parse PID file for %q: %w", taskName, err)
	}
	return &data, nil
}

func deletePIDFile(taskName string) {
	_ = os.Remove(pidFilePath(taskName))
}

// scanPIDFiles returns all valid PID files found on disk.
func scanPIDFiles() ([]*pidFileData, error) {
	entries, err := os.ReadDir(pidsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read pids directory: %w", err)
	}

	var result []*pidFileData
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".pid" {
			continue
		}
		taskName := entry.Name()[:len(entry.Name())-4] // strip .pid
		data, err := readPIDFile(taskName)
		if err != nil {
			// Corrupt PID file — remove and skip
			deletePIDFile(taskName)
			continue
		}
		result = append(result, data)
	}
	return result, nil
}
