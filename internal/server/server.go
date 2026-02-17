package server

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"runbookmcp.dev/internal/config"
	"runbookmcp.dev/internal/logs"
	"runbookmcp.dev/internal/process"
	"runbookmcp.dev/internal/task"
	"github.com/mark3labs/mcp-go/server"
)

// Server wraps the MCP server with task management
type Server struct {
	mu             sync.Mutex
	mcpServer      *server.MCPServer
	manager        *task.Manager
	manifest       *config.Manifest
	configLoaded   bool
	configPath     string
	version        string
	processManager task.ProcessManager
}

// NewServer creates a new MCP server with task management
func NewServer(manifest *config.Manifest, manager *task.Manager, processManager task.ProcessManager, configLoaded bool, version string, configPath string) *Server {
	// Create MCP server with capabilities
	mcpServer := server.NewMCPServer(
		"runbook",
		version,
		server.WithToolCapabilities(true),
		server.WithResourceCapabilities(true, false),
		server.WithPromptCapabilities(true),
	)

	s := &Server{
		mcpServer:      mcpServer,
		manager:        manager,
		manifest:       manifest,
		configLoaded:   configLoaded,
		configPath:     configPath,
		version:        version,
		processManager: processManager,
	}

	// Clean up old sessions at startup to bound directory size
	if _, err := logs.CleanupAllSessions(logs.DefaultRetention); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: session cleanup failed: %v\n", err)
	}

	// Register built-in tools (only if no config loaded)
	if !configLoaded {
		s.registerBuiltInTools()
	}

	// Register config refresh tool (always available)
	s.registerRefreshConfigTool()

	// Register tools, resources, and prompts from config
	s.registerTools()
	s.registerResources()
	s.registerPrompts()

	return s
}

// Serve starts the MCP server over stdio
func (s *Server) Serve() error {
	return server.ServeStdio(s.mcpServer)
}

// ServeHTTP starts the MCP server as a standalone HTTP server using
// StreamableHTTP transport. It handles graceful shutdown on SIGINT/SIGTERM.
// It writes a server registry file on start and removes it on shutdown.
func (s *Server) ServeHTTP(addr string) error {
	httpServer := server.NewStreamableHTTPServer(s.mcpServer)

	normalizedAddr := normalizeAddr(addr)
	if err := process.WriteServerFile(process.ServerFileData{
		Addr: normalizedAddr,
		PID:  os.Getpid(),
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to write server registry: %v\n", err)
	}
	defer process.DeleteServerFile("")

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Fprintln(os.Stderr, "\nShutting down HTTP server...")

		if err := httpServer.Shutdown(context.Background()); err != nil {
			fmt.Fprintf(os.Stderr, "Error shutting down HTTP server: %v\n", err)
		}

		// Stop all running daemons
		if s.processManager != nil {
			if err := s.processManager.StopAll(); err != nil {
				fmt.Fprintf(os.Stderr, "Error stopping daemons: %v\n", err)
			}
		}
	}()

	fmt.Fprintf(os.Stderr, "Dev Workflow MCP server listening on %s\n", normalizedAddr)
	return httpServer.Start(addr)
}

// normalizeAddr expands a bare port like ":8080" to "http://localhost:8080".
func normalizeAddr(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return "http://localhost" + addr
	}
	if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
		return "http://" + addr
	}
	return addr
}

// GetMCPServer returns the underlying MCP server
func (s *Server) GetMCPServer() *server.MCPServer {
	return s.mcpServer
}
