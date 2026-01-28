package server

import (
	"github.com/jarosser06/dev-toolkit-mcp/internal/config"
	"github.com/jarosser06/dev-toolkit-mcp/internal/task"
	"github.com/mark3labs/mcp-go/server"
)

// Server wraps the MCP server with task management
type Server struct {
	mcpServer    *server.MCPServer
	manager      *task.Manager
	manifest     *config.Manifest
	configLoaded bool
}

// NewServer creates a new MCP server with task management
func NewServer(manifest *config.Manifest, manager *task.Manager, configLoaded bool, version string) *Server {
	// Create MCP server with capabilities
	mcpServer := server.NewMCPServer(
		"dev-toolkit-mcp",
		version,
		server.WithToolCapabilities(true),
		server.WithResourceCapabilities(true, false),
		server.WithPromptCapabilities(true),
	)

	s := &Server{
		mcpServer:    mcpServer,
		manager:      manager,
		manifest:     manifest,
		configLoaded: configLoaded,
	}

	// Register built-in tools (only if no config loaded)
	if !configLoaded {
		s.registerBuiltInTools()
	}

	// Register tools, resources, and prompts from config
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
