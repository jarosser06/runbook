package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// LoadManifest loads and validates a task manifest from a file or directory.
// It searches for the manifest in the following priority order:
// 1. Custom path (if provided) — can be a file or directory
// 2. ./.dev_workflow.yaml (single file)
// 3. ./.dev_workflow/ directory (auto-loads all *.yaml files)
//
// Returns:
//   - manifest: The loaded manifest, or an empty manifest if none found
//   - loaded: true if a config file was successfully loaded, false if using default empty config
//   - error: Any error that occurred during parsing or validation (nil if successful or no config found)
func LoadManifest(customPath string) (*Manifest, bool, error) {
	// If custom path is provided, try it first (may be file or directory)
	if customPath != "" {
		manifest, err := loadFromPath(customPath)
		if err != nil {
			return nil, false, err
		}
		if manifest != nil {
			return manifest, true, nil
		}
		// Custom path didn't exist — fall through to defaults
	}

	// Try .dev_workflow.yaml single file
	if manifest, err := loadFromFile("./.dev_workflow.yaml"); err != nil {
		return nil, false, err
	} else if manifest != nil {
		return manifest, true, nil
	}

	// Try .dev_workflow/ directory
	if manifest, err := LoadFromDirectory("./.dev_workflow"); err != nil {
		return nil, false, err
	} else if manifest != nil {
		return manifest, true, nil
	}

	// No manifest found - return empty manifest instead of error
	// This allows the server to start and provide the init tool
	emptyManifest := &Manifest{
		Version: "1.0",
		Tasks:   make(map[string]Task),
	}

	return emptyManifest, false, nil
}

// loadFromPath loads a manifest from a path that may be a file or directory.
func loadFromPath(path string) (*Manifest, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to stat %s: %w", path, err)
	}

	if info.IsDir() {
		return LoadFromDirectory(path)
	}
	return loadFromFile(path)
}

// loadFromFile loads and validates a manifest from a single file.
// Returns nil manifest if the file does not exist.
func loadFromFile(path string) (*Manifest, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to stat %s: %w", path, err)
	}

	manifest, err := ParseManifest(path)
	if err != nil {
		return nil, fmt.Errorf("failed to parse manifest at %s: %w", path, err)
	}

	if err := Validate(manifest); err != nil {
		return nil, fmt.Errorf("invalid manifest at %s: %w", path, err)
	}

	return manifest, nil
}

// LoadFromDirectory scans a directory for *.yaml files and merges them
// into a single manifest. Returns nil manifest if the directory does not
// exist or contains no YAML files.
func LoadFromDirectory(dirPath string) (*Manifest, error) {
	info, err := os.Stat(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to stat directory %s: %w", dirPath, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dirPath)
	}

	// Glob for *.yaml files (non-recursive, top level only)
	pattern := filepath.Join(dirPath, "*.yaml")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to glob %s: %w", pattern, err)
	}

	if len(matches) == 0 {
		return nil, nil
	}

	// Sort for deterministic ordering
	sort.Strings(matches)

	root := &Manifest{
		Version: "1.0",
		Tasks:   make(map[string]Task),
	}

	var imported []*Manifest
	visited := make(map[string]bool)
	for _, match := range matches {
		m, nested, err := parseManifestWithImports(match, visited)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", match, err)
		}
		imported = append(imported, m)
		imported = append(imported, nested...)
	}

	// Merge all discovered manifests using root as the base
	manifest, err := mergeManifests(root, imported)
	if err != nil {
		return nil, fmt.Errorf("failed to merge manifests from %s: %w", dirPath, err)
	}

	// Apply defaults
	applyDefaults(manifest)

	// Validate the merged manifest
	if err := Validate(manifest); err != nil {
		return nil, fmt.Errorf("invalid merged manifest from %s: %w", dirPath, err)
	}

	return manifest, nil
}
