package server

import (
	"github.com/jarosser06/dev-toolkit-mcp/internal/config"
	"github.com/jarosser06/dev-toolkit-mcp/internal/task"
	"github.com/mark3labs/mcp-go/server"
)

// Server wraps the MCP server with task management
type Server struct {
	mcpServer *server.MCPServer
	manager   *task.Manager
	manifest  *config.Manifest
}

// NewServer creates a new MCP server with task management
func NewServer(manifest *config.Manifest, manager *task.Manager) *Server {
	// Create MCP server with capabilities
	mcpServer := server.NewMCPServer(
		"dev-toolkit-mcp",
		"1.0.0",
		server.WithToolCapabilities(true),
		server.WithResourceCapabilities(true, false),
		server.WithPromptCapabilities(true),
	)

	s := &Server{
		mcpServer: mcpServer,
		manager:   manager,
		manifest:  manifest,
	}

	// Register tools, resources, and prompts
	s.registerTools()
	s.registerResources()
	s.registerPrompts()

	return s
}

// Serve starts the MCP server
func (s *Server) Serve() error {
	return server.ServeStdio(s.mcpServer)
}

// GetMCPServer returns the underlying MCP server
func (s *Server) GetMCPServer() *server.MCPServer {
	return s.mcpServer
}
