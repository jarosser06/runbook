package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ParseManifest parses a YAML file into a Manifest structure
func ParseManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest file: %w", err)
	}

	var manifest Manifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Apply defaults to tasks
	applyDefaults(&manifest)

	return &manifest, nil
}

// applyDefaults merges manifest-level defaults with task-specific values
// Task-level values take precedence over manifest-level defaults
func applyDefaults(manifest *Manifest) {
	for taskName, task := range manifest.Tasks {
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
