project {
  name            = "dev-workflow-mcp"
  default_platform = "claude-code"

  git_exclude = true

  agent_instructions = <<EOF
# Claude Development Guidelines

**Always prefer MCP tools over raw bash commands for development tasks.** This ensures consistency, logging, and proper error handling across all development workflows.

  EOF
}

mcp_server "runbook" {
  description = "Local Runbook"
  command = "$${PWD}/bin/runbook"
  args = [
    "--config",
    "$${PWD}/examples/basic/mcp-tasks.yaml"
  ]
}
