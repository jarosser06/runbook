package config

import (
	"fmt"
	"os"
)

// LoadManifest loads and validates a task manifest from a file
// It searches for the manifest in the following priority order:
// 1. Custom path (if provided)
// 2. ./mcp-tasks.yaml (project root)
// 3. ./.mcp/tasks.yaml (hidden directory)
func LoadManifest(customPath string) (*Manifest, error) {
	searchPaths := []string{
		customPath,           // CLI flag (if provided)
		"./mcp-tasks.yaml",   // Project root
		"./.mcp/tasks.yaml",  // Hidden directory
	}

	var lastError error
	for _, path := range searchPaths {
		if path == "" {
			continue
		}

		// Check if file exists
		if _, err := os.Stat(path); err != nil {
			lastError = err
			continue
		}

		// Parse the manifest
		manifest, err := ParseManifest(path)
		if err != nil {
			return nil, fmt.Errorf("failed to parse manifest at %s: %w", path, err)
		}

		// Validate the manifest
		if err := Validate(manifest); err != nil {
			return nil, fmt.Errorf("invalid manifest at %s: %w", path, err)
		}

		return manifest, nil
	}

	// No manifest found in any location
	validPaths := []string{}
	for _, path := range searchPaths {
		if path != "" {
			validPaths = append(validPaths, path)
		}
	}

	return nil, fmt.Errorf("no task manifest found in: %v (last error: %v)", validPaths, lastError)
}
