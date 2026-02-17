# runbook

MCP server that exposes development tasks defined in YAML as MCP tools.

## Install

```bash
curl -fsSL https://runbook.dev/install.sh | bash
```

## Build

```bash
go build -o bin/runbook main.go
```

Or use make:

```bash
make build
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

## Usage with MCP

Add to your `.mcp.json`:

```json
{
  "mcpServers": {
    "runbook": {
      "args": ["-config", "/path/to/your/mcp-tasks.yaml"],
      "command": "/path/to/runbook"
    }
  }
}
```

## Development

```bash
make test    # Run tests
make lint    # Run linter
make build   # Build binary to bin/runbook
make install # Install to $HOME/.bin/runbook
```
