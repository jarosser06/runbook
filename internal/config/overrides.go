package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// LoadOverrides reads and parses the overrides YAML file at path.
// Returns nil if the file does not exist.
// Returns an error if the file exists but cannot be read or parsed.
func LoadOverrides(path string) (*Overrides, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read overrides file %s: %w", path, err)
	}

	var overrides Overrides
	if err := yaml.Unmarshal(data, &overrides); err != nil {
		return nil, fmt.Errorf("failed to parse overrides file %s: %w", path, err)
	}

	return &overrides, nil
}

// ApplyOverrides applies visibility overrides to the manifest in place.
// Glob patterns (e.g. "ts-*") are supported for all sections.
// Flags are additive: once set to true, they stay true.
func ApplyOverrides(manifest *Manifest, overrides *Overrides) {
	// Tasks
	for pattern, override := range overrides.Tasks {
		for name, task := range manifest.Tasks {
			if matchesPattern(pattern, name) {
				if override.Disabled {
					task.Disabled = true
				}
				if override.DisableMCP {
					task.DisableMCP = true
				}
				manifest.Tasks[name] = task
			}
		}
	}

	// Workflows
	for pattern, override := range overrides.Workflows {
		for name, wf := range manifest.Workflows {
			if matchesPattern(pattern, name) {
				if override.Disabled {
					wf.Disabled = true
				}
				if override.DisableMCP {
					wf.DisableMCP = true
				}
				manifest.Workflows[name] = wf
			}
		}
	}

	// Resources (MCP-only, so both flags have the same effect)
	for pattern, override := range overrides.Resources {
		for name, res := range manifest.Resources {
			if matchesPattern(pattern, name) {
				if override.Disabled || override.DisableMCP {
					res.Disabled = true
				}
				manifest.Resources[name] = res
			}
		}
	}

	// Prompts (MCP-only, so both flags have the same effect)
	for pattern, override := range overrides.Prompts {
		for name, prompt := range manifest.Prompts {
			if matchesPattern(pattern, name) {
				if override.Disabled || override.DisableMCP {
					prompt.Disabled = true
				}
				manifest.Prompts[name] = prompt
			}
		}
	}
}

// matchesPattern checks whether name matches pattern using filepath.Match glob syntax.
// An exact match is also accepted (filepath.Match handles that).
func matchesPattern(pattern, name string) bool {
	matched, err := filepath.Match(pattern, name)
	return err == nil && matched
}
