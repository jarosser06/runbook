package config

// TaskType represents the type of task execution
type TaskType string

const (
	// TaskTypeOneShot represents a task that runs once and completes
	TaskTypeOneShot TaskType = "oneshot"
	// TaskTypeDaemon represents a long-running background process
	TaskTypeDaemon TaskType = "daemon"
)

// Manifest represents the complete task configuration
type Manifest struct {
	Version    string                 `yaml:"version"`
	Imports    []string               `yaml:"imports,omitempty"`
	Tasks      map[string]Task        `yaml:"tasks"`
	TaskGroups map[string]TaskGroup   `yaml:"task_groups"`
	Prompts    map[string]Prompt      `yaml:"prompts"`
	Resources  map[string]Resource    `yaml:"resources"`
	Defaults   Defaults               `yaml:"defaults"`
	Workflows  map[string]Workflow    `yaml:"workflows"`
}

// Task represents a single executable task
type Task struct {
	Description            string            `yaml:"description"`
	Command                string            `yaml:"command"`
	Type                   TaskType          `yaml:"type"`
	WorkingDirectory       string            `yaml:"working_directory"`
	ExposeWorkingDirectory bool              `yaml:"expose_working_directory"`
	Env                    map[string]string `yaml:"env"`
	Timeout                int               `yaml:"timeout"`
	Shell                  string            `yaml:"shell"`
	Parameters             map[string]Param  `yaml:"parameters"`
	DependsOn              []string          `yaml:"depends_on"`
	DisableMCP             bool              `yaml:"disable_mcp,omitempty"`
}

// Param represents a task parameter definition
type Param struct {
	Type        string  `yaml:"type"`
	Required    bool    `yaml:"required"`
	Description string  `yaml:"description"`
	Default     *string `yaml:"default"`
}

// TaskGroup represents a collection of related tasks
type TaskGroup struct {
	Description string   `yaml:"description"`
	Tasks       []string `yaml:"tasks"`
}

// Prompt represents a templated prompt for AI agents
type Prompt struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Content     string `yaml:"content"`
}

// Resource represents a custom MCP resource with either inline or file-based content
type Resource struct {
	Description string `yaml:"description"`
	Content     string `yaml:"content"`
	File        string `yaml:"file"`
	MIMEType    string `yaml:"mime_type"`
}

// Defaults represents default values for task configuration
type Defaults struct {
	Timeout int               `yaml:"timeout"`
	Shell   string            `yaml:"shell"`
	Env     map[string]string `yaml:"env"`
}

// Workflow represents a composite workflow that runs multiple tasks sequentially
type Workflow struct {
	Description string           `yaml:"description"`
	Timeout     int              `yaml:"timeout"`
	Parameters  map[string]Param `yaml:"parameters"`
	Steps       []WorkflowStep   `yaml:"steps"`
}

// WorkflowStep represents a single step in a workflow
type WorkflowStep struct {
	Task              string            `yaml:"task"`
	Params            map[string]string `yaml:"params"`
	ContinueOnFailure bool             `yaml:"continue_on_failure"`
}
