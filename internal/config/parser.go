package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// parseManifestWithImports recursively parses a manifest and all its imports
// visited tracks files already processed to detect circular dependencies
func parseManifestWithImports(path string, visited map[string]bool) (*Manifest, []*Manifest, error) {
	// Normalize path for consistent comparison
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to resolve absolute path for %s: %w", path, err)
	}

	// Check for circular dependency
	if err := detectCircularDependency(absPath, visited); err != nil {
		return nil, nil, err
	}

	// Mark this file as visited
	visited[absPath] = true
	defer delete(visited, absPath)

	// Read and parse the file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read manifest file %s: %w", path, err)
	}

	var manifest Manifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, nil, fmt.Errorf("failed to parse YAML from %s: %w", path, err)
	}

	// Resolve file-based resources relative to this YAML file's directory
	if err := resolveResourceFiles(&manifest, filepath.Dir(absPath)); err != nil {
		return nil, nil, fmt.Errorf("failed to resolve resource files in %s: %w", path, err)
	}

	// If no imports, return just this manifest
	if len(manifest.Imports) == 0 {
		return &manifest, nil, nil
	}

	// Resolve import paths (expand globs)
	baseDir := filepath.Dir(path)
	importPaths, err := resolveImports(baseDir, manifest.Imports)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to resolve imports in %s: %w", path, err)
	}

	// Recursively parse all imports
	var importedManifests []*Manifest
	for _, importPath := range importPaths {
		imported, nestedImports, err := parseManifestWithImports(importPath, visited)
		if err != nil {
			return nil, nil, err
		}
		importedManifests = append(importedManifests, imported)
		// Add any nested imports
		importedManifests = append(importedManifests, nestedImports...)
	}

	return &manifest, importedManifests, nil
}

// resolveImports expands glob patterns and resolves relative paths
func resolveImports(baseDir string, imports []string) ([]string, error) {
	var resolved []string
	seen := make(map[string]bool)

	for _, importPattern := range imports {
		// Make path absolute relative to base directory
		pattern := importPattern
		if !filepath.IsAbs(pattern) {
			pattern = filepath.Join(baseDir, pattern)
		}

		// Expand glob pattern
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid glob pattern '%s': %w", importPattern, err)
		}

		if len(matches) == 0 {
			return nil, fmt.Errorf("import pattern '%s' matched no files", importPattern)
		}

		// Add unique matches
		for _, match := range matches {
			absMatch, err := filepath.Abs(match)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve absolute path for %s: %w", match, err)
			}
			if !seen[absMatch] {
				resolved = append(resolved, absMatch)
				seen[absMatch] = true
			}
		}
	}

	return resolved, nil
}

// detectCircularDependency checks if a file is already being processed
func detectCircularDependency(path string, visited map[string]bool) error {
	if visited[path] {
		// Build dependency chain for error message
		var chain []string
		for p := range visited {
			chain = append(chain, p)
		}
		chain = append(chain, path)
		return fmt.Errorf("circular import detected: %s", strings.Join(chain, " -> "))
	}
	return nil
}

// ParseManifest parses a YAML file into a Manifest structure
// If the manifest contains imports, it recursively loads and merges them
func ParseManifest(path string) (*Manifest, error) {
	// Parse main manifest and any imports
	visited := make(map[string]bool)
	mainManifest, importedManifests, err := parseManifestWithImports(path, visited)
	if err != nil {
		return nil, err
	}

	// If no imports, use the main manifest as-is
	var manifest *Manifest
	if len(importedManifests) == 0 {
		manifest = mainManifest
	} else {
		// Merge all imported manifests
		manifest, err = mergeManifests(mainManifest, importedManifests)
		if err != nil {
			return nil, fmt.Errorf("failed to merge manifests: %w", err)
		}
	}

	// Apply defaults to tasks
	applyDefaults(manifest)

	return manifest, nil
}

// resolveResourceFiles reads file-based resources and populates their Content field.
// File paths are resolved relative to the given base directory (the YAML file's dir).
func resolveResourceFiles(manifest *Manifest, baseDir string) error {
	for name, resource := range manifest.Resources {
		if resource.File == "" {
			continue
		}

		filePath := resource.File
		if !filepath.IsAbs(filePath) {
			filePath = filepath.Join(baseDir, filePath)
		}

		data, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("resource '%s': failed to read file %s: %w", name, filePath, err)
		}

		resource.Content = string(data)
		resource.File = ""
		manifest.Resources[name] = resource
	}
	return nil
}

// applyDefaults merges manifest-level defaults with task-specific values
// Task-level values take precedence over manifest-level defaults
func applyDefaults(manifest *Manifest) {
	for taskName, task := range manifest.Tasks {
		// Default task type to oneshot if not specified
		if task.Type == "" {
			task.Type = TaskTypeOneShot
		}

		// Apply default timeout if not set
		if task.Timeout == 0 && manifest.Defaults.Timeout > 0 {
			task.Timeout = manifest.Defaults.Timeout
		}

		// Apply default shell if not set
		if task.Shell == "" && manifest.Defaults.Shell != "" {
			task.Shell = manifest.Defaults.Shell
		}

		// Merge environment variables (task-level overrides defaults)
		if len(manifest.Defaults.Env) > 0 {
			if task.Env == nil {
				task.Env = make(map[string]string)
			}
			for key, value := range manifest.Defaults.Env {
				if _, exists := task.Env[key]; !exists {
					task.Env[key] = value
				}
			}
		}

		// Update the task in the map
		manifest.Tasks[taskName] = task
	}
}
