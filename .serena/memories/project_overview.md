# dev-workflow-mcp Project Overview

## Purpose
MCP (Model Context Protocol) server that exposes shell tasks (oneshot and daemon) as MCP tools based on YAML config.

## Tech Stack
- Language: Go
- Module: github.com/jarosser06/dev-workflow-mcp
- MCP library: github.com/mark3labs/mcp-go
- Template engine: text/template (Go stdlib)

## Structure
- `main.go` - Entry point
- `internal/config/` - YAML config parsing, validation, merging
- `internal/task/` - Task executor, dedup, manager
- `internal/server/` - MCP server, tool registration
- `internal/template/` - Template resolution for params and prompts
- `internal/logs/` - Session logging
- `internal/process/` - Daemon process management

## Commands
- Build: `go build ./...`
- Test: `go test ./...`
- Lint: Use project's configured linter
- Tidy: `go mod tidy`

## Paths
- Config directory: `.runbook/` (merged from all *.yaml files in dir)
- Overrides file: `.runbook.overrides.yaml` (optional, loaded after main manifest)
- Server registry: `._runbook_state/server.json`
- PID files: `._runbook_state/pids/`
- Logs: `._runbook_state/logs/`

## Key Patterns
- Config types in `config/types.go`, validation in `validator.go`, merging in `merger.go`
- Tools registered in `server/tools.go`, daemon tools in `tools_daemon.go`, session tools in `tools_session.go`
- `tools_refresh.go` handles config reload and tool cleanup via `collectToolNames()`
- Each oneshot tool is `run_<taskname>`, daemons get `start_/stop_/status_/logs_` prefixes
- Executor handles command execution, DedupExecutor wraps it for dedup
- Manager orchestrates executor + process manager

## Style
- Standard Go conventions
- No AI attribution in commits (per CLAUDE.md)
- All linting must pass before commit
