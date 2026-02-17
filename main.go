package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"runbookmcp.dev/internal/cli"
	"runbookmcp.dev/internal/config"
	"runbookmcp.dev/internal/logs"
	"runbookmcp.dev/internal/process"
	"runbookmcp.dev/internal/server"
	"runbookmcp.dev/internal/task"
)

var (
	// These variables are set at build time via -ldflags
	version = "dev"
	commit  = "none"    //nolint:unused
	date    = "unknown" //nolint:unused
)

func main() {
	// Extract -working-dir early so it works before both subcommand detection
	// and server-mode flag parsing.
	workingDir, filteredArgs := extractWorkingDir(os.Args[1:])

	// Detect CLI subcommands. The subcommand may be preceded by --local or
	// other CLI-only flags, so we scan past leading dashes.
	if len(filteredArgs) > 0 {
		for _, arg := range filteredArgs {
			if arg == "--local" || arg == "-local" {
				continue
			}
			if strings.HasPrefix(arg, "-") {
				break // hit a server flag; stop looking for a subcommand
			}
			switch arg {
			case "run", "start", "stop", "status", "logs", "list":
				os.Exit(cli.Execute(workingDir, filteredArgs))
			}
			break // first non-flag arg is not a subcommand
		}
	}

	// Parse command-line flags for server / stdio mode.
	configPath := flag.String("config", "", "Path to task manifest file or directory")
	initFlag := flag.Bool("init", false, "Initialize configuration file")
	serveFlag := flag.Bool("serve", false, "Run as standalone HTTP server")
	addrFlag := flag.String("addr", ":8080", "Listen address for HTTP mode")
	flag.CommandLine.Parse(filteredArgs) //nolint:errcheck

	// For stdio mode (no -serve), check whether a server is already running
	// before doing anything else. If one is found, proxy to it instead.
	if !*serveFlag && !*initFlag {
		serverData, err := process.ReadServerFile(workingDir)
		if err == nil {
			// Registry file exists — verify both PID liveness and HTTP reachability,
			// matching the same check performed by cli.Execute for subcommand routing.
			if !process.IsProcessAlive(serverData.PID) || !process.ProbeHTTP(serverData.Addr) {
				fmt.Fprintf(os.Stderr, "error: server.json exists but the server is not running (PID %d dead).\n", serverData.PID)
				fmt.Fprintf(os.Stderr, "Remove %s to continue in local mode.\n", process.ServerRegistryFile)
				os.Exit(1)
			}
			// Proxy all stdio MCP traffic to the running server.
			fmt.Fprintf(os.Stderr, "Proxying stdio to server at %s\n", serverData.Addr)
			if err := server.ServeStdioProxy(serverData.Addr); err != nil {
				fmt.Fprintf(os.Stderr, "Proxy error: %v\n", err)
				os.Exit(1)
			}
			return
		}
		// err != nil means no registry file → start a local server below.
	}

	// Change to the requested working directory for all local server modes.
	if workingDir != "" {
		if err := os.Chdir(workingDir); err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot change to directory %s: %v\n", workingDir, err)
			os.Exit(1)
		}
	}

	// Handle init flag — create config file and exit.
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
		// HTTP mode — signal handling and server registry are managed inside ServeHTTP.
		if err := mcpServer.ServeHTTP(*addrFlag); err != nil {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
			os.Exit(1)
		}
	} else {
		// Stdio mode (local) — setup signal handling for graceful shutdown.
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

// extractWorkingDir scans args for -working-dir / --working-dir and returns
// the directory value and the remaining args with that flag (and its value) removed.
func extractWorkingDir(args []string) (workingDir string, remaining []string) {
	remaining = make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-working-dir" || arg == "--working-dir" {
			if i+1 < len(args) {
				workingDir = args[i+1]
				i++ // skip the value
			}
			continue
		}
		if strings.HasPrefix(arg, "-working-dir=") {
			workingDir = arg[len("-working-dir="):]
			continue
		}
		if strings.HasPrefix(arg, "--working-dir=") {
			workingDir = arg[len("--working-dir="):]
			continue
		}
		remaining = append(remaining, arg)
	}
	return
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
