package template

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/jarosser06/runbook/internal/config"
)

// shellQuote single-quotes a string for safe shell interpolation.
// Embedded single quotes are escaped using the '\'' technique.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// TaskWrapper wraps a task to provide methods for template operations
type TaskWrapper struct {
	Name        string
	Description string
	Type        config.TaskType
}

// Run returns the tool name for running a one-shot task
func (t *TaskWrapper) Run() string {
	return "run_" + t.Name
}

// Start returns the tool name for starting a daemon
func (t *TaskWrapper) Start() string {
	return "start_" + t.Name
}

// Stop returns the tool name for stopping a daemon
func (t *TaskWrapper) Stop() string {
	return "stop_" + t.Name
}

// Status returns the tool name for checking daemon status
func (t *TaskWrapper) Status() string {
	return "status_" + t.Name
}

// Logs returns the tool name for reading task logs
func (t *TaskWrapper) Logs() string {
	return "logs_" + t.Name
}

// Desc returns the task description
func (t *TaskWrapper) Desc() string {
	return t.Description
}

// TaskTemplateData wraps tasks for template execution
type TaskTemplateData struct {
	Tasks map[string]*TaskWrapper
}

// ResolvePromptTemplate resolves template variables in prompt content
// Uses standard delimiters {{ and }} for template actions
// Provides task operations through TaskWrapper methods
func ResolvePromptTemplate(content string, tasks map[string]config.Task) (string, error) {
	// Create template with standard delimiters {{ and }}
	tmpl, err := template.New("prompt").Parse(content)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	// Wrap tasks for template access
	data := TaskTemplateData{Tasks: make(map[string]*TaskWrapper)}
	for name, task := range tasks {
		data.Tasks[name] = &TaskWrapper{
			Name:        name,
			Description: task.Description,
			Type:        task.Type,
		}
	}

	// Execute template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	return buf.String(), nil
}

// SubstituteParameters substitutes parameters in a command template
// Uses standard delimiters {{ and }} for template actions
// Fails if required parameters are missing (strict mode)
func SubstituteParameters(command string, params map[string]interface{}) (string, error) {
	// Create template with strict mode (fails on missing keys)
	tmpl, err := template.New("command").
		Funcs(template.FuncMap{"shellQuote": shellQuote}).
		Option("missingkey=error").
		Parse(command)
	if err != nil {
		return "", fmt.Errorf("parse command template: %w", err)
	}

	// Execute template with parameters
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, params); err != nil {
		return "", fmt.Errorf("execute command template: %w", err)
	}

	return buf.String(), nil
}
