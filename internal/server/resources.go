package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"runbookmcp.dev/internal/template"
	"github.com/mark3labs/mcp-go/mcp"
)

// registerResources registers MCP resources for task metadata
func (s *Server) registerResources() {
	// Register task-groups resource
	s.mcpServer.AddResource(
		mcp.NewResource(
			"dev-workflow://task-groups",
			"Task Groups",
			mcp.WithResourceDescription("List of all task groups and their tasks"),
		),
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			// Marshal task groups to JSON
			data, err := json.MarshalIndent(s.manifest.TaskGroups, "", "  ")
			if err != nil {
				return nil, fmt.Errorf("failed to marshal task groups: %w", err)
			}

			return []mcp.ResourceContents{
				mcp.TextResourceContents{
					URI:      "dev-workflow://task-groups",
					MIMEType: "application/json",
					Text:     string(data),
				},
			}, nil
		},
	)

	// Register task-dependencies resource
	s.mcpServer.AddResource(
		mcp.NewResource(
			"dev-workflow://task-dependencies",
			"Task Dependencies",
			mcp.WithResourceDescription("Task dependency graph"),
		),
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			// Build dependency map
			dependencies := make(map[string][]string)
			for taskName, task := range s.manifest.Tasks {
				if len(task.DependsOn) > 0 {
					dependencies[taskName] = task.DependsOn
				}
			}

			// Marshal to JSON
			data, err := json.MarshalIndent(dependencies, "", "  ")
			if err != nil {
				return nil, fmt.Errorf("failed to marshal dependencies: %w", err)
			}

			return []mcp.ResourceContents{
				mcp.TextResourceContents{
					URI:      "dev-workflow://task-dependencies",
					MIMEType: "application/json",
					Text:     string(data),
				},
			}, nil
		},
	)

	// Register template documentation resource
	s.mcpServer.AddResource(
		mcp.NewResource(
			"dev-workflow://docs/templates",
			"Template System Documentation",
			mcp.WithResourceDescription("Comprehensive guide to the template system for prompts and commands"),
		),
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			doc := `# Dev Workflow MCP - Template System

## Overview

The Dev Workflow MCP server uses Go's text/template package for two types of templates:
1. **Prompt templates** - Reference available tasks and their operations
2. **Command templates** - Substitute parameters into task commands

## Prompt Templates

Prompt templates use standard {{ }} delimiters and provide access to all tasks through the .Tasks map.

### Available Task Methods

For each task, you can access:
- ` + "`.Run`" + ` - Returns the tool name for running the task (e.g., "run_test")
- ` + "`.Start`" + ` - Returns the tool name for starting a daemon (e.g., "start_dev")
- ` + "`.Stop`" + ` - Returns the tool name for stopping a daemon (e.g., "stop_dev")
- ` + "`.Status`" + ` - Returns the tool name for checking daemon status (e.g., "status_dev")
- ` + "`.Logs`" + ` - Returns the tool name for reading logs (e.g., "logs_dev")
- ` + "`.Desc`" + ` - Returns the task description

### Syntax

` + "```yaml" + `
prompts:
  my_prompt:
    description: "Development workflow guide"
    content: |
      To run tests: {{.Tasks.test.Run}}
      Task description: {{.Tasks.test.Desc}}

      For the dev server:
      - Start: {{.Tasks.dev.Start}}
      - Stop: {{.Tasks.dev.Stop}}
      - Check status: {{.Tasks.dev.Status}}
      - View logs: {{.Tasks.dev.Logs}}
` + "```" + `

### Example Output

When resolved, the template produces:
` + "```" + `
To run tests: run_test
Task description: Run all tests

For the dev server:
- Start: start_dev
- Stop: stop_dev
- Check status: status_dev
- View logs: logs_dev
` + "```" + `

## Command Templates

Command templates use standard {{ }} delimiters for parameter substitution in task commands.

### Parameter Access

Parameters are accessed using dot notation: {{.parameter_name}}

### Syntax

` + "```yaml" + `
tasks:
  echo_message:
    description: "Echo a custom message"
    command: "echo '{{.message}}'"
    type: oneshot
    parameters:
      message:
        type: string
        required: true
        description: "The message to echo"

  create_file:
    description: "Create a file with content"
    command: "echo '{{.content}}' > {{.filename}}"
    type: oneshot
    parameters:
      filename:
        type: string
        required: true
        description: "Name of the file to create"
      content:
        type: string
        required: true
        description: "Content to write to the file"

  grep_search:
    description: "Search for a pattern in files"
    command: "grep -r '{{.pattern}}' {{.path}}"
    type: oneshot
    parameters:
      pattern:
        type: string
        required: true
        description: "Pattern to search for"
      path:
        type: string
        required: false
        description: "Path to search in"
        default: "."
` + "```" + `

### Default Values

Optional parameters can have default values:
` + "```yaml" + `
parameters:
  path:
    type: string
    required: false
    default: "."
` + "```" + `

When the parameter is not provided, the default value is automatically substituted.

### Strict Mode

Command templates use strict mode - if a required parameter is missing, the template execution will fail with a clear error message.

### Whitespace Control

You can control whitespace in templates using {{- and -}}:
- ` + "{{- .value}}" + ` - Trims whitespace before
- ` + "{{.value -}}" + ` - Trims whitespace after
- ` + "{{- .value -}}" + ` - Trims whitespace on both sides

### Best Practices

1. **Always quote variables in shell commands**:
   ` + "```" + `
   command: "echo '{{.message}}'"  # Good
   command: echo {{.message}}      # Bad (fails with spaces)
   ` + "```" + `

2. **Use meaningful parameter names**:
   ` + "```" + `
   {{.filename}}  # Good
   {{.f}}         # Bad
   ` + "```" + `

3. **Provide clear descriptions**:
   ` + "```yaml" + `
   parameters:
     pattern:
       type: string
       required: true
       description: "Regex pattern to search for"  # Good
   ` + "```" + `

4. **Set sensible defaults for optional parameters**:
   ` + "```yaml" + `
   parameters:
     timeout:
       type: string
       required: false
       default: "30"
   ` + "```" + `

## Template Functions

Currently, templates support all standard Go text/template functions:
- and, or, not - Boolean operations
- eq, ne, lt, le, gt, ge - Comparisons
- len - Length of arrays, maps, strings
- index - Index into arrays and maps
- printf - Formatted printing

Example with conditionals:
` + "```yaml" + `
command: "{{if .verbose}}set -x; {{end}}./script.sh"
` + "```" + `

## Error Handling

### Missing Required Parameters

If a required parameter is missing:
` + "```" + `
Error: parameter substitution failed: execute command template:
template: command:1:15: executing "command" at <.missing>:
map has no entry for key "missing"
` + "```" + `

### Invalid Template Syntax

If template syntax is invalid:
` + "```" + `
Error: parse command template: template: command:1:
unexpected "}" in operand
` + "```" + `

## Advanced Examples

### Multi-line Commands

` + "```yaml" + `
tasks:
  deploy:
    command: |
      echo "Deploying to {{.environment}}..."
      docker build -t {{.image}}:{{.tag}} .
      docker push {{.image}}:{{.tag}}
      kubectl set image deployment/{{.service}} app={{.image}}:{{.tag}}
    parameters:
      environment:
        type: string
        required: true
      image:
        type: string
        required: true
      tag:
        type: string
        required: false
        default: "latest"
      service:
        type: string
        required: true
` + "```" + `

### Conditional Logic

` + "```yaml" + `
tasks:
  test:
    command: "{{if .coverage}}go test -coverprofile=coverage.out{{else}}go test{{end}} {{.path}}"
    parameters:
      path:
        type: string
        required: false
        default: "./..."
      coverage:
        type: string
        required: false
        default: ""
` + "```" + `
`
			return []mcp.ResourceContents{
				mcp.TextResourceContents{
					URI:      "dev-workflow://docs/templates",
					MIMEType: "text/markdown",
					Text:     doc,
				},
			}, nil
		},
	)

	// Register configuration documentation resource
	s.mcpServer.AddResource(
		mcp.NewResource(
			"dev-workflow://docs/configuration",
			"Configuration Documentation",
			mcp.WithResourceDescription("Complete guide to the .runbook/ configuration directory"),
		),
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			doc := `# Dev Workflow MCP - Configuration Guide

## Overview

The Dev Workflow MCP server reads task definitions from a YAML manifest file. This guide covers all configuration options and best practices.

## File Location Priority

The server searches for configuration files in this order:
1. Custom path specified with ` + "`-config <path>`" + ` flag
2. ` + "`./.runbook/`" + ` directory (all *.yaml files merged)

## Basic Structure

` + "```yaml" + `
version: "1.0"

defaults:
  timeout: 300
  shell: "/bin/bash"

tasks:
  # Task definitions...

workflows:
  # Workflow definitions...

task_groups:
  # Task group definitions...

prompts:
  # Prompt definitions...
` + "```" + `

## Version

**Required.** Specifies the manifest format version.

` + "```yaml" + `
version: "1.0"
` + "```" + `

Currently, only "1.0" is supported.

## Defaults

**Optional.** Global default values for all tasks.

` + "```yaml" + `
defaults:
  timeout: 300        # Default timeout in seconds
  shell: "/bin/bash"  # Default shell for command execution
  working_directory: "."           # Default working directory
  env:               # Default environment variables
    NODE_ENV: "development"
` + "```" + `

Task-specific values override these defaults.

## Tasks

**Required.** Map of task names to task definitions.

### Task Types

Two task types are supported:
- ` + "`oneshot`" + ` - Runs once and returns output
- ` + "`daemon`" + ` - Runs continuously in the background

### One-Shot Task

` + "```yaml" + `
tasks:
  test:
    description: "Run all tests"
    command: "go test ./..."
    type: oneshot
    timeout: 300
    shell: "/bin/bash"
    working_directory: "."
    env:
      GO_ENV: "test"
` + "```" + `

**Generated MCP Tool:** ` + "`run_test`" + `

### Daemon Task

` + "```yaml" + `
tasks:
  dev:
    description: "Start development server"
    command: "npm run dev"
    type: daemon
    working_directory: "./frontend"
    env:
      NODE_ENV: "development"
      PORT: "3000"
` + "```" + `

**Generated MCP Tools:**
- ` + "`start_dev`" + ` - Start the daemon
- ` + "`stop_dev`" + ` - Stop the daemon
- ` + "`status_dev`" + ` - Check if running
- ` + "`logs_dev`" + ` - Read daemon logs

### Task Fields

| Field | Required | Type | Description |
|-------|----------|------|-------------|
| description | Yes | string | Human-readable description shown in MCP tools |
| command | Yes | string | Shell command to execute (supports templates) |
| type | Yes | string | Either "oneshot" or "daemon" |
| timeout | No | int | Timeout in seconds (default: from defaults or 300) |
| shell | No | string | Shell to use (default: from defaults or /bin/bash) |
| working_directory | No | string | Working directory (default: from defaults or .) |
| expose_working_directory | No | bool | If true, adds a working_directory parameter to the MCP tool |
| env | No | map | Environment variables to set |
| parameters | No | map | Parameter definitions (see Parameters section) |
| depends_on | No | []string | List of task names this task depends on |
| disabled | No | bool | If true, hidden from MCP and CLI entirely |
| disable_mcp | No | bool | If true, hidden from MCP only; CLI can still run it |

### Parameterized Tasks

Tasks can accept parameters that are substituted into the command:

` + "```yaml" + `
tasks:
  echo_message:
    description: "Echo a custom message"
    command: "echo '{{.message}}'"
    type: oneshot
    parameters:
      message:
        type: string
        required: true
        description: "The message to echo"

  create_file:
    description: "Create a file with content"
    command: "echo '{{.content}}' > {{.filename}}"
    type: oneshot
    parameters:
      filename:
        type: string
        required: true
        description: "Name of the file to create"
      content:
        type: string
        required: true
        description: "Content to write to the file"

  grep_search:
    description: "Search for a pattern in files"
    command: "grep -r '{{.pattern}}' {{.path}}"
    type: oneshot
    parameters:
      pattern:
        type: string
        required: true
        description: "Pattern to search for"
      path:
        type: string
        required: false
        description: "Path to search in (default: current directory)"
        default: "."
` + "```" + `

### Parameter Fields

| Field | Required | Type | Description |
|-------|----------|------|-------------|
| type | Yes | string | Parameter type (string, number, boolean) |
| required | Yes | bool | Whether parameter is required |
| description | Yes | string | Human-readable description |
| default | No | string | Default value for optional parameters |

### Dynamic Working Directory

Tasks can expose their working directory as a runtime parameter, allowing it to be overridden when the tool is called:

` + "```yaml" + `
tasks:
  test:
    description: "Run tests with configurable working directory"
    command: "pytest {{.test_path}}"
    working_directory: "."
    expose_working_directory: true
    parameters:
      test_path:
        type: string
        required: true
        description: "Path to test file or directory"
` + "```" + `

When ` + "`expose_working_directory: true`" + ` is set, the generated MCP tool will include a ` + "`working_directory`" + ` parameter:

**Resolution Priority:**
1. If ` + "`expose_working_directory: true`" + ` AND parameter provided → use parameter value
2. Otherwise → use static ` + "`working_directory`" + ` field value
3. Empty string parameters are treated as "not provided" (fallback to static value)

This enables flexible task execution where the working directory can be determined dynamically based on context, while maintaining a sensible default.

## Workflows

**Optional.** Composite workflows that chain multiple oneshot tasks into a single MCP tool call.

` + "```yaml" + `
workflows:
  ci:
    description: "Run full CI pipeline"
    timeout: 900
    parameters:
      test_flags:
        type: string
        required: false
        description: "Flags for test step"
        default: "-v"
    steps:
      - task: lint
      - task: test
        params:
          flags: "{{.test_flags}}"
      - task: build
        continue_on_failure: true
` + "```" + `

**Generated MCP Tool:** ` + "`run_workflow_ci`" + ` — description includes step names.

### Workflow Fields

| Field | Required | Type | Description |
|-------|----------|------|-------------|
| description | Yes | string | Human-readable description |
| timeout | No | int | Timeout in seconds for entire workflow |
| parameters | No | map | Workflow-level parameters (same schema as task parameters) |
| steps | Yes | list | Ordered list of steps to execute |
| disabled | No | bool | If true, hidden from MCP and CLI entirely |
| disable_mcp | No | bool | If true, hidden from MCP only |

### Step Fields

| Field | Required | Type | Description |
|-------|----------|------|-------------|
| task | Yes | string | Name of an existing oneshot task |
| params | No | map | Parameter overrides — values can use ` + "`{{.param}}`" + ` to reference workflow parameters |
| continue_on_failure | No | bool | If true, pipeline continues when step fails (default: false) |

### Behavior

- Steps run sequentially. Failure stops the pipeline unless ` + "`continue_on_failure: true`" + `.
- Only oneshot tasks can be referenced — daemon tasks are not allowed.
- Each step gets its own session ID and logs.
- If ` + "`timeout`" + ` is set and exceeded, remaining steps are marked as skipped.

## Task Groups

**Optional.** Logical grouping of related tasks.

` + "```yaml" + `
task_groups:
  ci:
    description: "CI/CD pipeline"
    tasks:
      - lint
      - test
      - build

  frontend:
    description: "Frontend development tasks"
    tasks:
      - frontend_dev
      - frontend_build
      - frontend_test
` + "```" + `

Task groups are exposed as the ` + "`dev-workflow://task-groups`" + ` MCP resource.

## Prompts

**Optional.** Predefined prompts with template variable substitution.

### Prompt Fields

| Field | Required | Type | Description |
|-------|----------|------|-------------|
| description | Yes | string | Human-readable description shown in MCP |
| content | No* | string | Inline prompt content (supports templates) |
| file | No* | string | Path to file containing prompt content (supports templates) |
| disabled | No | bool | If true, hidden from MCP entirely |

*Either ` + "`content`" + ` or ` + "`file`" + ` must be provided.

### Template Methods

Prompts support ` + "`{{.Tasks.<name>.<method>}}`" + ` expressions:

| Method | Description | Example output |
|--------|-------------|----------------|
| ` + "`.Run`" + ` | Tool name for running a oneshot task | ` + "`run_test`" + ` |
| ` + "`.Start`" + ` | Tool name for starting a daemon | ` + "`start_dev`" + ` |
| ` + "`.Stop`" + ` | Tool name for stopping a daemon | ` + "`stop_dev`" + ` |
| ` + "`.Status`" + ` | Tool name for checking daemon status | ` + "`status_dev`" + ` |
| ` + "`.Logs`" + ` | Tool name for reading daemon logs | ` + "`logs_dev`" + ` |
| ` + "`.Desc`" + ` | Task description text | ` + "`Run all tests`" + ` |

### Example

` + "```yaml" + `
prompts:
  dev_setup:
    description: "Guide for setting up development environment"
    content: |
      To set up the development environment:

      1. Run tests: {{.Tasks.test.Run}}
      2. Build project: {{.Tasks.build.Run}}
      3. Start dev server: {{.Tasks.dev.Start}}

      To check dev server status: {{.Tasks.dev.Status}}
      To view dev server logs: {{.Tasks.dev.Logs}}

  ci_guide:
    description: "CI workflow guide loaded from file"
    file: "docs/ci-guide.md"

  draft_prompt:
    description: "Work in progress"
    content: "..."
    disabled: true
` + "```" + `

See the Template System documentation for full template syntax.

## Custom Resources

**Optional.** Static or template-driven content served as MCP resources under ` + "`runbook://custom/<name>`" + `.

### Resource Fields

| Field | Required | Type | Description |
|-------|----------|------|-------------|
| description | Yes | string | Human-readable description shown in MCP |
| content | No* | string | Inline resource content (supports templates) |
| file | No* | string | Path to file containing resource content (supports templates) |
| mime_type | No | string | MIME type of the resource (default: ` + "`text/markdown`" + `) |
| disabled | No | bool | If true, hidden from MCP entirely |

*Either ` + "`content`" + ` or ` + "`file`" + ` must be provided.

### URI Scheme

Custom resources are available at ` + "`runbook://custom/<name>`" + `. For example, a resource named ` + "`architecture`" + ` is served at ` + "`runbook://custom/architecture`" + `.

### Template Support

Resource content supports the same ` + "`{{.Tasks.<name>.<method>}}`" + ` template syntax as prompts.

### Example

` + "```yaml" + `
resources:
  architecture:
    description: "System architecture overview"
    content: |
      # Architecture

      Key tools:
      - Run tests: {{.Tasks.test.Run}}
      - Start server: {{.Tasks.dev.Start}}

  api_reference:
    description: "API reference"
    file: "docs/api.md"
    mime_type: "text/markdown"
` + "```" + `

## Disabling and Visibility

Items can be hidden from MCP (and optionally the CLI) using ` + "`disabled`" + ` and ` + "`disable_mcp`" + ` flags.

### Flag Semantics

| Flag | Effect |
|------|--------|
| ` + "`disabled: true`" + ` | Hidden from both MCP and CLI entirely |
| ` + "`disable_mcp: true`" + ` | Hidden from MCP only; CLI can still run it |

### Supported Per Item Type

| Item type | ` + "`disabled`" + ` | ` + "`disable_mcp`" + ` |
|-----------|-----------|---------------|
| tasks | yes | yes |
| workflows | yes | yes |
| prompts | yes | — |
| resources | yes | — |

### Examples

` + "```yaml" + `
tasks:
  internal_task:
    description: "Available via CLI but not MCP"
    command: "./internal-script.sh"
    type: oneshot
    disable_mcp: true

  archived_task:
    description: "Completely hidden"
    command: "..."
    type: oneshot
    disabled: true

workflows:
  debug_workflow:
    description: "Debug only"
    steps:
      - task: internal_task
    disable_mcp: true

prompts:
  draft_prompt:
    description: "Work in progress"
    content: "..."
    disabled: true

resources:
  internal_doc:
    description: "Internal reference"
    content: "..."
    disabled: true
` + "```" + `

## Overrides File

**Optional.** ` + "`.runbook.overrides.yaml`" + ` is a project-local file that lets you override visibility flags without modifying the main config. It is useful for personal or environment-specific settings and is safe to gitignore.

Place it in the project root alongside the ` + "`.runbook/`" + ` directory.

### Structure

` + "```yaml" + `
tasks:
  <name-or-glob>:
    disabled: true
    disable_mcp: true

workflows:
  <name-or-glob>:
    disabled: true
    disable_mcp: true

resources:
  <name-or-glob>:
    disabled: true

prompts:
  <name-or-glob>:
    disabled: true
` + "```" + `

### Fields

| Field | Type | Description |
|-------|------|-------------|
| ` + "`tasks`" + ` | map | Overrides for tasks by name or glob |
| ` + "`workflows`" + ` | map | Overrides for workflows by name or glob |
| ` + "`resources`" + ` | map | Overrides for resources by name or glob |
| ` + "`prompts`" + ` | map | Overrides for prompts by name or glob |

Each entry supports:
- ` + "`disabled`" + ` — hides the item from MCP and CLI
- ` + "`disable_mcp`" + ` — hides the item from MCP only (tasks and workflows only)

### Glob Patterns

Keys can be glob patterns (e.g., ` + "`debug_*`" + `, ` + "`*_internal`" + `) to match multiple items at once.

### Additive Flags

Override flags are additive: once ` + "`disabled: true`" + ` or ` + "`disable_mcp: true`" + ` is applied (from either the main config or the overrides file), it cannot be unset by the other source.

### Example

` + "```yaml" + `
# .runbook.overrides.yaml
tasks:
  debug_*:
    disable_mcp: true    # Hide all debug_ tasks from MCP

  legacy_deploy:
    disabled: true       # Completely hide this task

workflows:
  experimental_*:
    disable_mcp: true

prompts:
  draft_*:
    disabled: true
` + "```" + `

## Complete Example

` + "```yaml" + `
version: "1.0"

defaults:
  timeout: 300
  shell: "/bin/bash"

tasks:
  # Development
  dev:
    description: "Start development server"
    command: "npm run dev"
    type: daemon
    working_directory: "./frontend"
    env:
      NODE_ENV: "development"
      PORT: "3000"

  # Testing
  test:
    description: "Run all tests"
    command: "go test ./..."
    type: oneshot
    timeout: 600

  test_frontend:
    description: "Run frontend tests"
    command: "npm test"
    type: oneshot
    working_directory: "./frontend"

  # Building
  build:
    description: "Build the project"
    command: "make build"
    type: oneshot
    depends_on:
      - test

  # Linting
  lint:
    description: "Run linter"
    command: "golangci-lint run ./..."
    type: oneshot

  # Utilities
  echo_message:
    description: "Echo a custom message"
    command: "echo '{{.message}}'"
    type: oneshot
    parameters:
      message:
        type: string
        required: true
        description: "The message to echo"

  create_file:
    description: "Create a file with content"
    command: "echo '{{.content}}' > {{.filename}}"
    type: oneshot
    parameters:
      filename:
        type: string
        required: true
        description: "Name of the file to create"
      content:
        type: string
        required: true
        description: "Content to write to the file"

workflows:
  ci:
    description: "Run full CI pipeline"
    steps:
      - task: lint
      - task: test
      - task: build
        continue_on_failure: true

task_groups:
  ci:
    description: "CI/CD pipeline"
    tasks:
      - lint
      - test
      - build

  dev:
    description: "Development tasks"
    tasks:
      - dev
      - test_frontend

prompts:
  dev_setup:
    description: "Development environment setup guide"
    content: |
      # Development Setup

      ## Running Tests
      - All tests: {{.Tasks.test.Run}}
      - Frontend tests: {{.Tasks.test_frontend.Run}}

      ## Development Server
      - Start: {{.Tasks.dev.Start}}
      - Stop: {{.Tasks.dev.Stop}}
      - Status: {{.Tasks.dev.Status}}
      - Logs: {{.Tasks.dev.Logs}}

      ## Building
      - Build project: {{.Tasks.build.Run}}
      - Run linter: {{.Tasks.lint.Run}}
` + "```" + `

## Validation Rules

The server validates configurations on load:

1. **Required fields**: version, tasks, task.description, task.command, task.type
2. **Valid task types**: Must be "oneshot" or "daemon"
3. **Valid task references**: Task groups and dependencies must reference existing tasks
4. **Valid parameters**: Parameters must have type, required, and description
5. **Valid timeouts**: Must be positive integers
6. **Valid environment**: Must be key-value string pairs
7. **Valid workflows**: Must have description and at least one step; steps must reference existing oneshot tasks (not daemons); workflow parameters follow the same rules as task parameters

## Best Practices

### 1. Use Descriptive Task Names

` + "```yaml" + `
# Good
tasks:
  frontend_dev:
    description: "Start frontend development server"

# Bad
tasks:
  fd:
    description: "Start frontend development server"
` + "```" + `

### 2. Group Related Tasks

` + "```yaml" + `
task_groups:
  ci:
    description: "CI/CD pipeline"
    tasks:
      - lint
      - test
      - build
` + "```" + `

### 3. Set Reasonable Timeouts

` + "```yaml" + `
tasks:
  test:
    timeout: 600    # 10 minutes for test suite
  lint:
    timeout: 120    # 2 minutes for linting
` + "```" + `

### 4. Use Environment Variables for Configuration

` + "```yaml" + `
tasks:
  dev:
    env:
      NODE_ENV: "development"
      DEBUG: "true"
      PORT: "3000"
` + "```" + `

### 5. Provide Clear Parameter Descriptions

` + "```yaml" + `
parameters:
  filename:
    type: string
    required: true
    description: "Name of the file to create (e.g., 'output.txt')"
` + "```" + `

### 6. Set Sensible Defaults for Optional Parameters

` + "```yaml" + `
parameters:
  path:
    type: string
    required: false
    default: "."
    description: "Path to search in (default: current directory)"
` + "```" + `

### 7. Document Complex Commands

` + "```yaml" + `
tasks:
  deploy:
    description: "Deploy application to production"
    command: |
      # Build production bundle
      npm run build
      # Upload to S3
      aws s3 sync ./dist s3://my-bucket/
      # Invalidate CloudFront cache
      aws cloudfront create-invalidation --distribution-id XXX --paths "/*"
` + "```" + `

## Troubleshooting

### Configuration Not Found

**Error:** ` + "`no task manifest found`" + `

**Solution:** Ensure your config directory exists:
- ` + "`./.runbook/`" + ` directory with *.yaml files
- Or specify with ` + "`-config <path>`" + ` flag

### Invalid YAML Syntax

**Error:** ` + "`yaml: line X: mapping values are not allowed in this context`" + `

**Solution:** Check YAML indentation and syntax. Use a YAML validator.

### Task Not Found

**Error:** ` + "`task 'xxx' not found`" + `

**Solution:** Verify the task name exists in the tasks section.

### Parameter Missing

**Error:** ` + "`map has no entry for key 'xxx'`" + `

**Solution:** Either:
1. Provide the required parameter
2. Add a default value in the parameter definition
3. Make the parameter optional (` + "`required: false`" + `)

### Command Timeout

**Error:** ` + "`command timed out after X seconds`" + `

**Solution:** Increase the timeout value in the task definition.

## Resources

- Template documentation: ` + "`runbook://docs/templates`" + `
- Task groups: ` + "`runbook://task-groups`" + `
- Task dependencies: ` + "`runbook://task-dependencies`" + `
`
			return []mcp.ResourceContents{
				mcp.TextResourceContents{
					URI:      "dev-workflow://docs/configuration",
					MIMEType: "text/markdown",
					Text:     doc,
				},
			}, nil
		},
	)

	// Register custom resources from config
	s.registerCustomResources()
}

// registerCustomResources registers user-defined resources from the manifest
func (s *Server) registerCustomResources() {
	for resourceName, resourceDef := range s.manifest.Resources {
		if resourceDef.Disabled {
			continue
		}

		name := resourceName
		def := resourceDef

		mimeType := def.MIMEType
		if mimeType == "" {
			mimeType = "text/markdown"
		}

		uri := "runbook://custom/" + name

		var opts []mcp.ResourceOption
		opts = append(opts, mcp.WithResourceDescription(def.Description))
		opts = append(opts, mcp.WithMIMEType(mimeType))

		s.mcpServer.AddResource(
			mcp.NewResource(uri, name, opts...),
			func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
				var rawContent string
				if def.File != "" {
					data, err := os.ReadFile(def.File)
					if err != nil {
						return nil, fmt.Errorf("failed to read resource file %s: %w", def.File, err)
					}
					rawContent = string(data)
				} else {
					rawContent = def.Content
				}

				// Resolve template variables in content
				resolvedContent, err := template.ResolvePromptTemplate(rawContent, s.manifest.Tasks)
				if err != nil {
					return nil, fmt.Errorf("failed to resolve resource template: %w", err)
				}

				return []mcp.ResourceContents{
					mcp.TextResourceContents{
						URI:      uri,
						MIMEType: mimeType,
						Text:     resolvedContent,
					},
				}, nil
			},
		)
	}
}
