project {
  name            = "dev-workflow-mcp"
  agentic_platform = "claude-code"

  agent_instructions = <<EOF
# Claude Development Guidelines

**Always prefer MCP tools over raw bash commands for development tasks.** This ensures consistency, logging, and proper error handling across all development workflows.

  EOF
}

claude_settings "project_permissions" {
  enable_all_project_mcp_servers = true
}

mcp_server "runbook" {
  description = "Local Runbook"
  command = "$${PWD}/bin/runbook"
  args = [
    "--config",
    "$${PWD}/examples/basic/mcp-tasks.yaml"
  ]
}

registry "nexus" {
  url = "https://nexustemplateproduction.z13.web.core.windows.net"
}

plugin "base-dev" {}

plugin "code-review" {}

plugin "typescript" {}


