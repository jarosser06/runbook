package config

import (
	"fmt"
)

// mergeManifests combines a base manifest with imported manifests
// The base manifest provides the version and defaults
// Imported manifests contribute tasks, task groups, and prompts
// Returns an error if duplicate keys are found
func mergeManifests(base *Manifest, imports []*Manifest) (*Manifest, error) {
	result := &Manifest{
		Version:    base.Version,
		Defaults:   base.Defaults,
		Tasks:      make(map[string]Task),
		TaskGroups: make(map[string]TaskGroup),
		Prompts:    make(map[string]Prompt),
		Resources:  make(map[string]Resource),
		Workflows:  make(map[string]Workflow),
	}

	// Start with base manifest tasks, groups, prompts, resources, and workflows
	if err := mergeTasks(result.Tasks, base.Tasks); err != nil {
		return nil, err
	}
	if err := mergeTaskGroups(result.TaskGroups, base.TaskGroups); err != nil {
		return nil, err
	}
	if err := mergePrompts(result.Prompts, base.Prompts); err != nil {
		return nil, err
	}
	if err := mergeResources(result.Resources, base.Resources); err != nil {
		return nil, err
	}
	if err := mergeWorkflows(result.Workflows, base.Workflows); err != nil {
		return nil, err
	}

	// Merge each imported manifest
	for _, imported := range imports {
		if err := mergeTasks(result.Tasks, imported.Tasks); err != nil {
			return nil, err
		}
		if err := mergeTaskGroups(result.TaskGroups, imported.TaskGroups); err != nil {
			return nil, err
		}
		if err := mergePrompts(result.Prompts, imported.Prompts); err != nil {
			return nil, err
		}
		if err := mergeResources(result.Resources, imported.Resources); err != nil {
			return nil, err
		}
		if err := mergeWorkflows(result.Workflows, imported.Workflows); err != nil {
			return nil, err
		}
	}

	return result, nil
}

// mergeTasks merges source tasks into destination
// Returns error if duplicate task names are found
func mergeTasks(dst, src map[string]Task) error {
	for name, task := range src {
		if _, exists := dst[name]; exists {
			return fmt.Errorf("duplicate task name '%s' found during merge", name)
		}
		dst[name] = task
	}
	return nil
}

// mergeTaskGroups merges source task groups into destination
// Returns error if duplicate group names are found
func mergeTaskGroups(dst, src map[string]TaskGroup) error {
	for name, group := range src {
		if _, exists := dst[name]; exists {
			return fmt.Errorf("duplicate task group name '%s' found during merge", name)
		}
		dst[name] = group
	}
	return nil
}

// mergePrompts merges source prompts into destination
// Returns error if duplicate prompt names are found
func mergePrompts(dst, src map[string]Prompt) error {
	for name, prompt := range src {
		if _, exists := dst[name]; exists {
			return fmt.Errorf("duplicate prompt name '%s' found during merge", name)
		}
		dst[name] = prompt
	}
	return nil
}

// mergeResources merges source resources into destination
// Returns error if duplicate resource names are found
func mergeResources(dst, src map[string]Resource) error {
	for name, resource := range src {
		if _, exists := dst[name]; exists {
			return fmt.Errorf("duplicate resource name '%s' found during merge", name)
		}
		dst[name] = resource
	}
	return nil
}

// mergeWorkflows merges source workflows into destination
// Returns error if duplicate workflow names are found
func mergeWorkflows(dst, src map[string]Workflow) error {
	for name, workflow := range src {
		if _, exists := dst[name]; exists {
			return fmt.Errorf("duplicate workflow name '%s' found during merge", name)
		}
		dst[name] = workflow
	}
	return nil
}
