# Claude Development Guidelines

This project uses the **Dev Toolkit MCP Server** to provide standardized development tools. When working on this codebase, you MUST use the MCP tools instead of running raw bash commands.

## Why Use MCP Tools?

- **Consistency**: All developers (human and AI) use the same commands
- **Logging**: All operations are logged to `._dev_tools/logs/`
- **Configuration**: Commands are defined in `examples/basic/mcp-tasks.yaml`
- **Discoverability**: Tools are self-documenting via MCP protocol

## Available Tools

### Core Development Tools

| Tool | Purpose | Example |
|------|---------|---------|
| `run_build` | Build the project | Build binary to `bin/dev-toolkit-mcp` |
| `run_test` | Run full test suite | Tests with race detection and coverage |
| `run_lint` | Run golangci-lint | Check code quality |
| `run_clean` | Clean build artifacts | Remove bin/, coverage files |
| `run_install` | Install dependencies | Download modules and install golangci-lint |

### Parameterized Tools

| Tool | Parameters | Purpose |
|------|------------|---------|
| `run_go_build` | flags, output, package | Build with custom options |
| `run_go_test` | flags, package | Run tests on specific packages |
| `run_echo_message` | message | Echo a message (example) |
| `run_create_file` | filename, content | Create a file (example) |
| `run_grep_search` | pattern, path | Search for patterns |

### Usage Examples

**DO THIS** ✅
```
Use run_test to verify all tests pass with race detection
Use run_lint to check code quality
Use run_go_test with flags="-race -cover" and package="./internal/config/..."
```

**DON'T DO THIS** ❌
```
Run: go test -race ./...
Run: golangci-lint run
Run: make build
```

## When to Use Raw Bash

Only use raw bash commands for:
- Operations not available as MCP tools
- One-off exploratory commands (checking file contents, etc.)
- System commands unrelated to development workflow

## Workflow Guidelines

### Before Making Changes
1. Use `run_lint` to check current code quality
2. Use `run_test` to ensure all tests pass

### After Making Changes
1. Use `run_lint` to verify code quality
2. Use `run_test` to verify all tests still pass
3. Use `run_build` to ensure project builds

### Testing Specific Packages
Use `run_go_test` with custom flags:
```
run_go_test with:
- flags: "-race -v"
- package: "./internal/process/..."
```

### Building with Custom Options
Use `run_go_build` with custom flags:
```
run_go_build with:
- flags: "-race"
- output: "bin/dev-toolkit-mcp-debug"
- package: "."
```

## Task Groups

Tasks are organized into logical groups:

### CI Pipeline
- lint → test → build
- Use for pre-commit validation

### Utilities
- echo_message, create_file, grep_search
- General-purpose helper commands

## Documentation Resources

Access comprehensive documentation via MCP resources:

- `devtoolkit://docs/configuration` - Complete guide to mcp-tasks.yaml
- `devtoolkit://docs/templates` - Template system documentation
- `devtoolkit://task-groups` - View available task groups
- `devtoolkit://task-dependencies` - Task dependency graph

## Important Notes

### Log Files
All task executions write logs to `._dev_tools/logs/<task-name>.log`

To read logs:
```
Read ._dev_tools/logs/test.log
Read ._dev_tools/logs/build.log
```

### Race Conditions
Tests are run with `-race` flag by default. This is intentional and important for catching concurrency bugs.

### Coverage
Test coverage reports are generated in `coverage.out` and summarized in test output.

## Configuration

Task definitions are in: `examples/basic/mcp-tasks.yaml`

To add new tasks:
1. Edit the YAML file
2. Restart Claude Code to reload configuration
3. New tools will be available as `run_<taskname>`

## Summary

**Always prefer MCP tools over raw bash commands for development tasks.** This ensures consistency, logging, and proper error handling across all development workflows.
