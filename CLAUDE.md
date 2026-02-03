# Claude Development Guidelines

This project uses the **Dev Toolkit MCP Server** to provide standardized development tools. When working on this codebase, you MUST use the MCP tools instead of running raw bash commands.

## Why Use MCP Tools?

- **Consistency**: All developers (human and AI) use the same commands
- **Logging**: All operations are logged to `._dev_tools/logs/`
- **Configuration**: Commands are defined in `examples/basic/mcp-tasks.yaml`
- **Discoverability**: Tools are self-documenting via MCP protocol

## Bootstrapping New Projects

The MCP server can start without a configuration file, making it easy to bootstrap new projects.

### Using the Init Tool

When no `mcp-tasks.yaml` file is found, the server starts with an empty configuration and provides the `init` tool:

```
Use init tool to create mcp-tasks.yaml
```

This creates a minimal configuration file with example tasks:
- `build` - Build the project
- `test` - Run tests
- `lint` - Run linter
- `ci` task group - Runs lint → test → build

After creating the config, restart the MCP server to load the new configuration.

### Using the CLI Flag

Alternatively, you can use the `-init` command-line flag:

```bash
./bin/dev-toolkit-mcp -init
```

This creates `mcp-tasks.yaml` in the current directory and exits. Edit the file to add your project's tasks, then start the server normally.

### Customizing the Init Tool

The `init` tool accepts optional parameters:

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `path` | string | `./mcp-tasks.yaml` | Where to create the config file |
| `overwrite` | boolean | `false` | Whether to replace existing file |

Example:
```
Use init tool with path=".mcp/tasks.yaml" and overwrite=false
```

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

### Session-Based Logging

The dev-toolkit-mcp uses UUID-based session tracking for all task executions. Each execution gets a unique session ID with logs organized in session directories for better traceability and debugging.

#### Log Structure

```
._dev_tools/logs/
  sessions/
    <uuid>/
      task.log          # Log output for this execution
      metadata.json     # Execution details (timestamp, params, exit code, etc.)
  latest/
    <task-name> -> ../sessions/<uuid>/  # Symlink to latest session
```

#### Session Benefits

- **Traceability**: Each execution has a unique UUID for tracking
- **Metadata**: Rich execution context (start time, duration, exit code, parameters, etc.)
- **History**: Keep multiple execution sessions for debugging
- **No rotation**: Sessions are naturally bounded, no log rotation needed

#### Accessing Logs

**Read latest execution logs:**
```
Read ._dev_tools/logs/latest/test/task.log
```

**List recent sessions:**
```
Use list_sessions with task_name="test" and limit=10
```

**Read session metadata:**
```
Use read_session_metadata with session_id="<uuid>"
```

**Read specific session logs:**
```
Use read_session_log with session_id="<uuid>"
```

#### Session IDs in Results

All MCP tool results now include a `session_id` field:

```json
{
  "success": true,
  "exit_code": 0,
  "task_name": "test",
  "session_id": "550e8400-e29b-41d4-a716-446655440000",
  "log_path": "._dev_tools/logs/sessions/550e8400-e29b-41d4-a716-446655440000/task.log"
}
```

#### Session Metadata

Each session includes comprehensive metadata:

```json
{
  "session_id": "550e8400-e29b-41d4-a716-446655440000",
  "task_name": "test",
  "task_type": "oneshot",
  "start_time": "2026-02-02T10:00:00Z",
  "end_time": "2026-02-02T10:01:30Z",
  "duration": 90000000000,
  "exit_code": 0,
  "success": true,
  "timed_out": false,
  "parameters": {},
  "command": "go test -race -cover ./...",
  "working_dir": "/path/to/project"
}
```

#### Session Management Tools

| Tool | Purpose | Parameters |
|------|---------|------------|
| `list_sessions` | List recent sessions | `task_name` (required), `limit` (optional, default 20) |
| `read_session_metadata` | Read session metadata | `session_id` (required) |
| `read_session_log` | Read session log | `session_id` (required), `lines` (optional), `filter` (optional) |

#### Daemon Logs with Sessions

Daemon tasks also support session-based logging. Access daemon logs by session:

```
Use logs_<daemon_name> with session_id="<uuid>"
```

Or get the current daemon's session ID from its status:

```
Use status_<daemon_name>
```

Returns `session_id` in the status response.

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

## Multi-File Configuration (Imports)

The MCP server supports splitting tasks, prompts, and task groups across multiple YAML files using the `imports` field. This enables modular configuration and packaging individual commands with tools like dex.

### Basic Import Example

**Main config (mcp-tasks.yaml):**
```yaml
version: "1.0"

imports:
  - "./tasks/build.yaml"
  - "./tasks/test.yaml"

defaults:
  timeout: 300
  shell: "/bin/bash"

tasks:
  main:
    description: "Main task"
    command: "echo 'Hello from main config'"
```

**Imported file (tasks/build.yaml):**
```yaml
version: "1.0"

tasks:
  build:
    description: "Build the project"
    command: "go build -o bin/app ."
```

### Import Features

- **Glob patterns**: Use wildcards to import multiple files
  ```yaml
  imports:
    - "./tasks/*.yaml"
    - "./prompts/dev/*.yaml"
  ```

- **Nested imports**: Imported files can have their own imports
  ```yaml
  # main.yaml imports level1.yaml
  # level1.yaml imports level2.yaml
  # All files are merged recursively
  ```

- **Relative paths**: Paths are resolved relative to the importing file
  ```yaml
  # In ./configs/main.yaml
  imports:
    - "./tasks/build.yaml"  # Resolves to ./configs/tasks/build.yaml
  ```

### Merge Behavior

When multiple files are imported:

- **Tasks**: Merged into a single task map. Duplicate task names cause an error.
- **TaskGroups**: Merged into a single map. Duplicate group names cause an error.
- **Prompts**: Merged into a single map. Duplicate prompt names cause an error.
- **Defaults**: Only the main file's defaults are used (imported defaults are ignored).
- **Version**: Only the main file's version is used.

### Error Handling

The import system detects common issues:

- **Circular imports**: Returns error with the dependency chain
  ```
  Error: circular import detected: a.yaml -> b.yaml -> a.yaml
  ```

- **Duplicate keys**: Returns error specifying which files have conflicts
  ```
  Error: duplicate task name 'build' found during merge
  ```

- **Missing files**: Returns error if import pattern matches no files
  ```
  Error: import pattern './nonexistent/*.yaml' matched no files
  ```

### Dex Integration Pattern

The import system is designed to work seamlessly with dex packages:

```yaml
version: "1.0"

imports:
  - "./.dex/*/tasks.yaml"    # Load all dex-installed tasks
  - "./local-tasks.yaml"      # Local project tasks

defaults:
  timeout: 300
  shell: "/bin/bash"
```

A dex package structure might look like:
```
.dex/
  my-commands/
    tasks.yaml              # Task definitions
    docs/context.md         # Documentation
```

### File Organization Patterns

**Pattern A - Split by type:**
```
mcp-tasks.yaml              # Main config with imports
tasks/
  build.yaml                # Build tasks
  test.yaml                 # Test tasks
prompts/
  dev.yaml                  # Development prompts
```

**Pattern B - Split by feature:**
```
mcp-tasks.yaml              # Main config
features/
  golang/
    tasks.yaml              # Go-specific tasks
    prompts.yaml            # Go-specific prompts
  docker/
    tasks.yaml              # Docker tasks
```

### Backward Compatibility

The import system is fully backward compatible:
- Existing single-file configurations work without changes
- Empty or missing `imports` field = single-file behavior
- All existing LoadManifest search paths still work

## Summary

**Always prefer MCP tools over raw bash commands for development tasks.** This ensures consistency, logging, and proper error handling across all development workflows.
