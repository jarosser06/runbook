package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/jarosser06/runbook/internal/cli"
	"github.com/jarosser06/runbook/internal/config"
	"github.com/jarosser06/runbook/internal/logs"
	"github.com/jarosser06/runbook/internal/process"
	"github.com/jarosser06/runbook/internal/server"
	"github.com/jarosser06/runbook/internal/task"
)

var (
	// These variables are set at build time via -ldflags
	version = "dev"
	commit  = "none"   //nolint:unused
	date    = "unknown" //nolint:unused
)

func main() {
	// Detect CLI subcommands before flag.Parse()
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "run", "start", "stop", "status", "logs", "list":
			os.Exit(cli.Execute(os.Args[1:]))
		}
	}

	// Parse command-line flags (server mode)
	configPath := flag.String("config", "", "Path to task manifest file or directory")
	initFlag := flag.Bool("init", false, "Initialize configuration file")
	serveFlag := flag.Bool("serve", false, "Run as standalone HTTP server")
	addrFlag := flag.String("addr", ":8080", "Listen address for HTTP mode")
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
		fmt.Fprintln(os.Stderr, "Create .dev_workflow.yaml or .dev_workflow/ directory, or use -config flag")
	}

	// Create process manager
	processManager := process.NewManager()

	// Create task manager
	taskManager := task.NewManager(manifest, processManager)

	// Create MCP server
	mcpServer := server.NewServer(manifest, taskManager, processManager, loaded, version, *configPath)

	if *serveFlag {
		// HTTP mode — signal handling is done inside ServeHTTP
		if err := mcpServer.ServeHTTP(*addrFlag); err != nil {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
			os.Exit(1)
		}
	} else {
		// Stdio mode — setup signal handling for graceful shutdown
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

		go func() {
			<-sigChan
			fmt.Fprintln(os.Stderr, "\nShutting down...")
			if err := processManager.StopAll(); err != nil {
				fmt.Fprintf(os.Stderr, "Error stopping daemons: %v\n", err)
			}
			os.Exit(0)
		}()

		if err := mcpServer.Serve(); err != nil {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
			os.Exit(1)
		}
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
	targetPath := "./.dev_workflow.yaml"

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
