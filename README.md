# dev-toolkit-mcp

MCP server that exposes development tasks defined in YAML as MCP tools.

## Build

```bash
go build -o bin/dev-toolkit-mcp main.go
```

## Configuration

Create `mcp-tasks.yaml`:

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

Task types:
- `oneshot` - Runs once, returns output
- `daemon` - Runs in background, provides start/stop/status/logs tools

## Usage

Add to `.mcp.json`:

```json
{
  "mcpServers": {
    "dev-toolkit-mcp": {
      "command": "/path/to/bin/dev-toolkit-mcp",
      "args": ["-config", "/path/to/mcp-tasks.yaml"]
    }
  }
}
```

Tools are generated from task names:
- Oneshot task `test` → `run_test`
- Daemon task `dev` → `start_dev`, `stop_dev`, `status_dev`, `logs_dev`

## Parameters

Tasks can accept parameters:

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

## Documentation

The server exposes resources:
- `devtoolkit://docs/configuration` - Full configuration guide
- `devtoolkit://docs/templates` - Template syntax reference
- `devtoolkit://task-groups` - Task groups list
- `devtoolkit://task-dependencies` - Task dependency graph

## Logs

All task executions write logs to `._dev_tools/logs/<task-name>.log`. Logs rotate at 10MB.
