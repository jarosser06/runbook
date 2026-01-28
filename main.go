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
	// These variables are set at build time via -ldflags
	version = "dev"
	commit  = "none"   //nolint:unused
	date    = "unknown" //nolint:unused
)

func main() {
	// Parse command-line flags
	configPath := flag.String("config", "", "Path to task manifest file")
	initFlag := flag.Bool("init", false, "Initialize configuration file")
	flag.Parse()

	// Handle init flag - create config file and exit
	if *initFlag {
		handleInit()
		return
	}

	// Setup logging
	if err := logs.Setup(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to setup logs: %v\n", err)
		os.Exit(1)
	}

	// Load configuration
	manifest, loaded, err := config.LoadManifest(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load manifest: %v\n", err)
		os.Exit(1)
	}

	// Print warning if no config was found
	if !loaded {
		fmt.Fprintln(os.Stderr, "Warning: No config file found. Server starting with empty configuration.")
		fmt.Fprintln(os.Stderr, "Use the 'init' tool to create mcp-tasks.yaml")
	}

	// Create process manager
	processManager := process.NewManager()

	// Create task manager
	taskManager := task.NewManager(manifest, processManager)

	// Create MCP server (pass loaded flag so init tool is only available when needed)
	mcpServer := server.NewServer(manifest, taskManager, loaded, version)

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

const minimalConfig = `version: "1.0"

# Example tasks - customize these for your project
tasks:
  build:
    description: "Build the project"
    command: "echo 'Add your build command here'"
    type: oneshot

  test:
    description: "Run tests"
    command: "echo 'Add your test command here'"
    type: oneshot

  lint:
    description: "Run linter"
    command: "echo 'Add your lint command here'"
    type: oneshot

# Task groups organize related tasks
task_groups:
  ci:
    description: "CI pipeline tasks"
    tasks:
      - lint
      - test
      - build
`

// handleInit creates a minimal config file
func handleInit() {
	targetPath := "./mcp-tasks.yaml"

	// Check if file already exists
	if _, err := os.Stat(targetPath); err == nil {
		fmt.Fprintf(os.Stderr, "Error: %s already exists\n", targetPath)
		fmt.Fprintf(os.Stderr, "Remove the existing file or use the MCP 'init' tool with overwrite=true\n")
		os.Exit(1)
	}

	// Write config file
	if err := os.WriteFile(targetPath, []byte(minimalConfig), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to create config file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully created %s\n", targetPath)
	fmt.Println("Edit this file to add your project's tasks, then start the MCP server.")
	os.Exit(0)
}
