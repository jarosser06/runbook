# dev-workflow-mcp

MCP server that exposes development tasks defined in YAML as MCP tools.

## Build

```bash
go build -o bin/dev-workflow-mcp main.go
```

## Configuration

The server looks for configuration in this order:

1. Path given via `-config` flag (file or directory)
2. `.dev_workflow.yaml` in the current directory
3. `.dev_workflow/` directory — all `*.yaml` files are merged automatically

### Single file

Create `.dev_workflow.yaml`:

```yaml
version: "1.0"

tasks:
  test:
    description: "Run tests"
    command: "go test ./..."
    type: oneshot

  dev:
    description: "Start dev server"
    command: "npm run dev"
    type: daemon
```

### Directory

Split config across files in `.dev_workflow/`:

```
.dev_workflow/
  tasks.yaml
  daemons.yaml
  prompts.yaml
```

Each file is a standard manifest. They are merged together — task names, group names, and prompt names must be unique across files.

## Usage

### Stdio (default)

Add to `.mcp.json`:

```json
{
  "mcpServers": {
    "dev-workflow-mcp": {
      "command": "/path/to/bin/dev-workflow-mcp",
      "args": ["-config", "/path/to/.dev_workflow.yaml"]
    }
  }
}
```

### Standalone HTTP server

Run as a long-lived service using MCP-over-StreamableHTTP:

```bash
./bin/dev-workflow-mcp -serve -addr :8080 -config .dev_workflow.yaml
```

This mode supports multiple concurrent clients and survives beyond a single session. Graceful shutdown on SIGINT/SIGTERM stops the HTTP server and all running daemons.

## Tasks

Task types:
- `oneshot` — runs once, returns output. Generates a `run_<name>` tool.
- `daemon` — runs in background. Generates `start_<name>`, `stop_<name>`, `status_<name>`, `logs_<name>` tools.

### Parameters

Tasks can accept parameters using Go template syntax:

```yaml
tasks:
  deploy:
    description: "Deploy to environment"
    command: "kubectl apply -f {{.manifest}} -n {{.namespace}}"
    type: oneshot
    parameters:
      manifest:
        type: string
        required: true
        description: "Path to manifest file"
      namespace:
        type: string
        required: false
        default: "default"
        description: "Kubernetes namespace"
```

### Deduplication

Concurrent identical one-shot requests (same task name + same parameters) are deduplicated. The first request executes; subsequent requests wait and receive the same result. This prevents duplicate work when multiple clients call the same tool simultaneously.

## Prompts

Prompts are predefined text templates exposed via the MCP `prompts/list` and `prompts/get` methods. They're useful for giving AI agents context about available workflows.

Prompt templates can reference task tool names using `{{.Tasks.<name>.<method>}}`:

| Method | Returns | Example |
|--------|---------|---------|
| `.Run` | Oneshot tool name | `run_test` |
| `.Start` | Daemon start tool | `start_dev` |
| `.Stop` | Daemon stop tool | `stop_dev` |
| `.Status` | Daemon status tool | `status_dev` |
| `.Logs` | Daemon logs tool | `logs_dev` |
| `.Desc` | Task description | `"Run tests"` |

### Example

```yaml
prompts:
  dev_setup:
    description: "Development workflow guide"
    content: |
      ## Running Tests
      Use {{.Tasks.test.Run}} to run the test suite.

      ## Dev Server
      - Start: {{.Tasks.dev.Start}}
      - Stop: {{.Tasks.dev.Stop}}
      - Status: {{.Tasks.dev.Status}}
      - Logs: {{.Tasks.dev.Logs}}
```

When a client calls `prompts/get` with `name: "dev_setup"`, the template is resolved:

```
## Running Tests
Use run_test to run the test suite.

## Dev Server
- Start: start_dev
- Stop: stop_dev
- Status: status_dev
- Logs: logs_dev
```

This lets an AI agent discover the correct tool names without hardcoding them.

## Resources

The server exposes MCP resources — read-only data that clients can fetch via `resources/list` and `resources/read`. These give clients structured information about the server's configuration.

| URI | Description | Format |
|-----|-------------|--------|
| `dev-workflow://task-groups` | Task groups and their member tasks | JSON |
| `dev-workflow://task-dependencies` | Dependency graph between tasks | JSON |
| `dev-workflow://docs/templates` | Template syntax reference | Markdown |
| `dev-workflow://docs/configuration` | Full configuration guide | Markdown |

### Example: task-groups

Given this config:

```yaml
task_groups:
  ci:
    description: "CI pipeline"
    tasks:
      - lint
      - test
      - build
```

Reading `dev-workflow://task-groups` returns:

```json
{
  "ci": {
    "Description": "CI pipeline",
    "Tasks": ["lint", "test", "build"]
  }
}
```

### Example: task-dependencies

Given tasks with `depends_on`:

```yaml
tasks:
  test:
    description: "Run tests"
    command: "go test ./..."
    type: oneshot
  build:
    description: "Build project"
    command: "go build"
    type: oneshot
    depends_on:
      - test
```

Reading `dev-workflow://task-dependencies` returns:

```json
{
  "build": ["test"]
}
```

## Built-in Tools

These tools are always available regardless of configuration:

- `refresh_config` — reloads all configuration from disk without restarting the server. New tasks appear immediately, removed tasks are cleaned up.
- `list_sessions` — lists recent execution sessions for a task.
- `read_session_metadata` — reads metadata (params, timing, exit code) for a session.
- `read_session_log` — reads log output for a session.
- `init` — creates a starter `.dev_workflow.yaml` (only available when no config is loaded).

## Logs

All task executions are logged to `._dev_tools/logs/sessions/<session-id>/`. Each execution gets a unique session ID for traceability.

## CLI Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-config` | _(auto-detect)_ | Path to config file or directory |
| `-init` | `false` | Create a starter `.dev_workflow.yaml` and exit |
| `-serve` | `false` | Run as standalone HTTP server instead of stdio |
| `-addr` | `:8080` | Listen address for HTTP mode |
