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

mcp_server "dev-workflow-mcp" {
  description = "Dev Workflow MCP"
  command = "$${PWD}/bin/dev-workflow-mcp"
  args = [
    "-config",
    "$${PWD}/examples/basic/mcp-tasks.yaml"
  ]
}

registry "dex-dev-registry" {
  url = "http://dex-dev-registry-production-471112549359.s3-website-us-west-2.amazonaws.com"
}


plugin "base-dev" {
  registry = "dex-dev-registry"
}

plugin "code-review" {
  registry = "dex-dev-registry"
}
