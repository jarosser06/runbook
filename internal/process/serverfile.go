package process

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"runbookmcp.dev/internal/dirs"
)

// ServerRegistryFile is the path (relative to the project root) where the HTTP
// server writes its address and PID when it starts.
const ServerRegistryFile = dirs.StateDir + "/server.json"

// ServerFileData is persisted to disk when the HTTP server starts.
type ServerFileData struct {
	Addr string `json:"addr"`
	PID  int    `json:"pid"`
}

func serverFilePath(workingDir string) string {
	if workingDir == "" {
		return ServerRegistryFile
	}
	return filepath.Join(workingDir, ServerRegistryFile)
}

// WriteServerFile writes the server registry to disk in the current working directory.
func WriteServerFile(data ServerFileData) error {
	dir := filepath.Dir(ServerRegistryFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal server file: %w", err)
	}
	return os.WriteFile(ServerRegistryFile, b, 0644)
}

// ReadServerFile reads the server registry. workingDir="" uses the current working directory.
func ReadServerFile(workingDir string) (*ServerFileData, error) {
	b, err := os.ReadFile(serverFilePath(workingDir))
	if err != nil {
		return nil, err
	}
	var data ServerFileData
	if err := json.Unmarshal(b, &data); err != nil {
		return nil, fmt.Errorf("failed to parse server file: %w", err)
	}
	return &data, nil
}

// DeleteServerFile removes the server registry. workingDir="" uses the current working directory.
func DeleteServerFile(workingDir string) {
	_ = os.Remove(serverFilePath(workingDir))
}

// IsProcessAlive reports whether a process with the given PID is running.
func IsProcessAlive(pid int) bool {
	return isProcessAlive(pid)
}
