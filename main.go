package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/jarosser06/dev-toolkit-mcp/internal/config"
	"github.com/jarosser06/dev-toolkit-mcp/internal/logs"
	"github.com/jarosser06/dev-toolkit-mcp/internal/process"
	"github.com/jarosser06/dev-toolkit-mcp/internal/server"
	"github.com/jarosser06/dev-toolkit-mcp/internal/task"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	// Parse command-line flags
	configPath := flag.String("config", "", "Path to task manifest file")
	flag.Parse()

	// Setup logging
	if err := logs.Setup(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to setup logs: %v\n", err)
		os.Exit(1)
	}

	// Load configuration
	manifest, err := config.LoadManifest(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load manifest: %v\n", err)
		os.Exit(1)
	}

	// Create process manager
	processManager := process.NewManager()

	// Create task manager
	taskManager := task.NewManager(manifest, processManager)

	// Create MCP server
	mcpServer := server.NewServer(manifest, taskManager)

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Fprintln(os.Stderr, "\nShutting down...")
		// Stop all running daemons
		if err := processManager.StopAll(); err != nil {
			fmt.Fprintf(os.Stderr, "Error stopping daemons: %v\n", err)
		}
		os.Exit(0)
	}()

	// Start server
	if err := mcpServer.Serve(); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}
