# runbook

MCP server that exposes development tasks defined in YAML as MCP tools.

## Install

```bash
curl -fsSL https://runbookmcp.dev/install.sh | bash
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

## CLI Usage

Run tasks directly from the command line:

```bash
runbook list                                    # List all available tasks
runbook run <task> [--param=value...]           # Run a oneshot task or workflow
runbook start <task> [--param=value...]         # Start a daemon
runbook stop <task>                             # Stop a daemon
runbook status <task>                           # Show daemon status
runbook logs <task> [--lines=N] [--filter=REGEX] [--session=ID]
```

All subcommands accept `--config=path` to specify a custom config location.

### Examples

```bash
# List tasks defined in your config
runbook list

# Run a task
runbook run build

# Run a parameterized task
runbook run go_test --flags="-v -race" --package="./..."

# Start/stop a daemon
runbook start dev
runbook stop dev
runbook status dev

# View logs
runbook logs dev --lines=50
runbook logs dev --filter="ERROR"
```

Task output goes to stdout (pipeable). Status and metadata go to stderr.

## Prompt Templates

Prompts support Go template syntax. Use `run_task` to reference task tool names — this works with any task name including those containing hyphens:

```yaml
prompts:
  dev-workflow:
    description: "How to work on this project"
    content: |
      Run tests: {{run_task "my-tests"}}
      Start server: {{run_task "dev-server"}}
```

`{{run_task "my-tests"}}` resolves to `run_my-tests`. For task names without hyphens, dot-access also works: `{{.Tasks.build.Run}}` → `run_build`.

## Usage with MCP

Add to your `.mcp.json`:

```json
{
  "mcpServers": {
    "runbook": {
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
