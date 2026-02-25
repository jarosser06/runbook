package config

import (
	"fmt"
	"strings"
)

// Validate performs validation on a parsed manifest
func Validate(manifest *Manifest) error {
	var errors []string

	// Validate version
	if manifest.Version == "" {
		errors = append(errors, "version is required")
	}

	// Validate imports
	for i, importPath := range manifest.Imports {
		if importPath == "" {
			errors = append(errors, fmt.Sprintf("import at index %d cannot be empty", i))
		}
	}

	// Validate tasks - allow empty task map (for fresh init), but not nil
	if manifest.Tasks == nil {
		errors = append(errors, "tasks map must be initialized")
	}

	for taskName, task := range manifest.Tasks {
		if err := validateTask(taskName, task, manifest.Tasks); err != nil {
			errors = append(errors, err.Error())
		}
	}

	// Validate task groups
	for groupName, group := range manifest.TaskGroups {
		if err := validateTaskGroup(groupName, group, manifest.Tasks); err != nil {
			errors = append(errors, err.Error())
		}
	}

	// Validate prompts
	for promptName, prompt := range manifest.Prompts {
		if err := validatePrompt(promptName, prompt); err != nil {
			errors = append(errors, err.Error())
		}
	}

	// Validate resources
	for resourceName, resource := range manifest.Resources {
		if err := validateResource(resourceName, resource); err != nil {
			errors = append(errors, err.Error())
		}
	}

	// Validate workflows
	for workflowName, workflow := range manifest.Workflows {
		if err := validateWorkflow(workflowName, workflow, manifest.Tasks); err != nil {
			errors = append(errors, err.Error())
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("validation errors:\n  - %s", strings.Join(errors, "\n  - "))
	}

	return nil
}

func validateTask(name string, task Task, allTasks map[string]Task) error {
	var errors []string

	// Required fields
	if task.Description == "" {
		errors = append(errors, fmt.Sprintf("task '%s': description is required", name))
	}

	if task.Command == "" {
		errors = append(errors, fmt.Sprintf("task '%s': command is required", name))
	}

	// Validate task type (defaults are applied in parser.go applyDefaults)
	if task.Type != "" && task.Type != TaskTypeOneShot && task.Type != TaskTypeDaemon {
		errors = append(errors, fmt.Sprintf("task '%s': invalid type '%s' (must be 'oneshot' or 'daemon')", name, task.Type))
	}

	// Validate parameters
	for paramName, param := range task.Parameters {
		if param.Type == "" {
			errors = append(errors, fmt.Sprintf("task '%s': parameter '%s' must specify a type", name, paramName))
		}
		if param.Description == "" {
			errors = append(errors, fmt.Sprintf("task '%s': parameter '%s' must have a description", name, paramName))
		}
	}

	// Validate dependencies
	for _, dep := range task.DependsOn {
		if _, exists := allTasks[dep]; !exists {
			errors = append(errors, fmt.Sprintf("task '%s': dependency '%s' does not exist", name, dep))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("%s", strings.Join(errors, "; "))
	}

	return nil
}

func validateTaskGroup(name string, group TaskGroup, allTasks map[string]Task) error {
	var errors []string

	if group.Description == "" {
		errors = append(errors, fmt.Sprintf("task_group '%s': description is required", name))
	}

	if len(group.Tasks) == 0 {
		errors = append(errors, fmt.Sprintf("task_group '%s': must contain at least one task", name))
	}

	// Validate task references
	for _, taskName := range group.Tasks {
		if _, exists := allTasks[taskName]; !exists {
			errors = append(errors, fmt.Sprintf("task_group '%s': task '%s' does not exist", name, taskName))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("%s", strings.Join(errors, "; "))
	}

	return nil
}

func validatePrompt(name string, prompt Prompt) error {
	var errors []string

	if prompt.Description == "" {
		errors = append(errors, fmt.Sprintf("prompt '%s': description is required", name))
	}

	if prompt.Content == "" && prompt.File == "" {
		errors = append(errors, fmt.Sprintf("prompt '%s': either content or file is required", name))
	}

	if prompt.Content != "" && prompt.File != "" {
		errors = append(errors, fmt.Sprintf("prompt '%s': content and file are mutually exclusive", name))
	}

	if len(errors) > 0 {
		return fmt.Errorf("%s", strings.Join(errors, "; "))
	}

	return nil
}

func validateResource(name string, resource Resource) error {
	var errors []string

	if resource.Description == "" {
		errors = append(errors, fmt.Sprintf("resource '%s': description is required", name))
	}

	if resource.Content == "" && resource.File == "" {
		errors = append(errors, fmt.Sprintf("resource '%s': either content or file is required", name))
	}

	if resource.Content != "" && resource.File != "" {
		errors = append(errors, fmt.Sprintf("resource '%s': content and file are mutually exclusive", name))
	}

	if len(errors) > 0 {
		return fmt.Errorf("%s", strings.Join(errors, "; "))
	}

	return nil
}

func validateWorkflow(name string, workflow Workflow, allTasks map[string]Task) error {
	var errors []string

	if workflow.Description == "" {
		errors = append(errors, fmt.Sprintf("workflow '%s': description is required", name))
	}

	if len(workflow.Steps) == 0 {
		errors = append(errors, fmt.Sprintf("workflow '%s': must contain at least one step", name))
	}

	// Validate each step
	for i, step := range workflow.Steps {
		if step.Task == "" {
			errors = append(errors, fmt.Sprintf("workflow '%s': step %d must reference a task", name, i))
			continue
		}

		task, exists := allTasks[step.Task]
		if !exists {
			errors = append(errors, fmt.Sprintf("workflow '%s': step %d references non-existent task '%s'", name, i, step.Task))
			continue
		}

		if task.Type == TaskTypeDaemon {
			errors = append(errors, fmt.Sprintf("workflow '%s': step %d references daemon task '%s' (only oneshot tasks allowed)", name, i, step.Task))
		}
	}

	// Validate workflow parameters
	for paramName, param := range workflow.Parameters {
		if param.Type == "" {
			errors = append(errors, fmt.Sprintf("workflow '%s': parameter '%s' must specify a type", name, paramName))
		}
		if param.Description == "" {
			errors = append(errors, fmt.Sprintf("workflow '%s': parameter '%s' must have a description", name, paramName))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("%s", strings.Join(errors, "; "))
	}

	return nil
}
