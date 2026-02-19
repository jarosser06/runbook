package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"runbookmcp.dev/internal/config"
	"runbookmcp.dev/internal/dirs"
	"runbookmcp.dev/internal/logs"
	"runbookmcp.dev/internal/process"
	"runbookmcp.dev/internal/server"
	"runbookmcp.dev/internal/task"
)

// Package-level vars are the standard way to bind Cobra persistent flags (same
// pattern used by viper). Execute() resets them before each invocation to ensure
// test isolation.
var (
	globalConfig     string
	globalWorkingDir string
	globalLocal      bool
)

// exitError is a sentinel error that carries a specific exit code.
// RunE functions return this instead of calling os.Exit directly, allowing
// Execute to handle process termination in one place.
type exitError struct{ code int }

func (e *exitError) Error() string { return fmt.Sprintf("exit status %d", e.code) }

// newMCPServer performs the common server bootstrap: sets up logging, loads the
// manifest, and creates the process manager, task manager, and MCP server.
func newMCPServer(v string) (*server.Server, *process.Manager, error) {
	if err := logs.Setup(); err != nil {
		return nil, nil, fmt.Errorf("failed to setup logs: %w", err)
	}

	manifest, loaded, err := config.LoadManifest(globalConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load manifest: %w", err)
	}
	if !loaded {
		fmt.Fprintln(os.Stderr, "Warning: No config file found. Server starting with empty configuration.")
		fmt.Fprintf(os.Stderr, "Create %s/ directory with YAML files, or use --config flag\n", dirs.ConfigDir)
	}

	processManager := process.NewManager()
	taskManager := task.NewManager(manifest, processManager)
	mcpServer := server.NewServer(manifest, taskManager, processManager, loaded, v, globalConfig)
	return mcpServer, processManager, nil
}

// newRootCmd builds and returns the full Cobra command tree.
// It is separated from Execute so tests can construct a fresh command.
func newRootCmd(v string) *cobra.Command {
	root := &cobra.Command{
		Use:           "runbook",
		Short:         "MCP server for shell tasks",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Stdio mode (default, no subcommand): check for proxy first.
			if !globalLocal {
				serverData, err := process.ReadServerFile(globalWorkingDir)
				if err == nil {
					if !process.IsProcessAlive(serverData.PID) || !process.ProbeHTTP(serverData.Addr) {
						fmt.Fprintf(os.Stderr, "error: server.json exists but the server is not running (PID %d dead).\n", serverData.PID)
						fmt.Fprintf(os.Stderr, "Remove %s to continue in local mode.\n", process.ServerRegistryFile)
						return &exitError{code: 1}
					}
					fmt.Fprintf(os.Stderr, "Proxying stdio to server at %s\n", serverData.Addr)
					if err := server.ServeStdioProxy(serverData.Addr); err != nil {
						return fmt.Errorf("proxy error: %w", err)
					}
					return nil
				}
			}

			if err := applyWorkingDir(); err != nil {
				return err
			}

			fmt.Fprintln(os.Stderr, "runbook: standalone mode")

			mcpServer, processManager, err := newMCPServer(v)
			if err != nil {
				return err
			}

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

			return mcpServer.Serve()
		},
	}

	// Persistent flags available to all subcommands.
	root.PersistentFlags().StringVar(&globalConfig, "config", "", "Path to task manifest file or directory")
	root.PersistentFlags().StringVar(&globalWorkingDir, "working-dir", "", "Set project working directory")
	root.PersistentFlags().BoolVar(&globalLocal, "local", false, "Run locally, bypassing any running server")

	root.AddCommand(newServeCmd(v), newInitCmd(), newListCmd(), newRunCmd(), newStartCmd(), newStopCmd(), newStatusCmd(), newLogsCmd())
	return root
}

func newServeCmd(v string) *cobra.Command {
	var serveAddr string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run as standalone HTTP server",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := applyWorkingDir(); err != nil {
				return err
			}
			mcpServer, _, err := newMCPServer(v)
			if err != nil {
				return err
			}
			return mcpServer.ServeHTTP(serveAddr)
		},
	}
	cmd.Flags().StringVar(&serveAddr, "addr", ":8080", "Listen address for HTTP mode")
	return cmd
}

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize configuration file",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := applyWorkingDir(); err != nil {
				return err
			}
			return handleInit()
		},
	}
}

// Execute sets up and runs the Cobra command tree.
func Execute(v string) {
	// Reset global state for each invocation.
	globalConfig = ""
	globalWorkingDir = ""
	globalLocal = false

	cmd := newRootCmd(v)
	if err := cmd.Execute(); err != nil {
		var exitErr *exitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.code)
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// bootstrap loads config and creates the task manager, mirroring server setup.
func bootstrap(configPath string) (*config.Manifest, *task.Manager, *process.Manager, error) {
	if err := logs.Setup(); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to setup logs: %w", err)
	}

	manifest, loaded, err := config.LoadManifest(configPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to load config: %w", err)
	}
	if !loaded {
		return nil, nil, nil, fmt.Errorf("no config file found (use --config or create %s/ directory)", dirs.ConfigDir)
	}

	processManager := process.NewManager()
	taskManager := task.NewManager(manifest, processManager)
	taskManager.SetStreaming(os.Stdout, os.Stderr)
	return manifest, taskManager, processManager, nil
}

// parseTaskParams dynamically parses --key=value flags from args based on the task's
// parameter definitions. Returns a map suitable for passing to ExecuteOneShot/StartDaemon.
func parseTaskParams(taskDef config.Task, args []string) (map[string]interface{}, error) {
	params := make(map[string]interface{})
	if len(taskDef.Parameters) == 0 {
		if len(args) > 0 {
			return nil, fmt.Errorf("task does not accept parameters, but got: %s", strings.Join(args, " "))
		}
		return params, nil
	}

	fs := flag.NewFlagSet("params", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	flagPtrs := make(map[string]*string)
	for name, param := range taskDef.Parameters {
		defaultVal := ""
		if param.Default != nil {
			defaultVal = fmt.Sprintf("%v", *param.Default)
		}
		flagPtrs[name] = fs.String(name, defaultVal, param.Description)
	}

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	if fs.NArg() > 0 {
		return nil, fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " "))
	}

	for name, ptr := range flagPtrs {
		if *ptr != "" {
			params[name] = *ptr
		}
	}

	for name, param := range taskDef.Parameters {
		if param.Required {
			if _, ok := params[name]; !ok {
				return nil, fmt.Errorf("required parameter --%s is missing", name)
			}
		}
	}

	return params, nil
}

// isMCPEnabled returns false when the first arg names a task that has
// disable_mcp: true, indicating the task should bypass any running server and
// execute locally. Returns true on any error or when no task matches.
//
// This must use config.LoadManifest directly rather than bootstrap to avoid
// creating a process.Manager, which calls restoreFromPIDFiles() and would kill
// any currently-running daemons whose PID files it finds.
func isMCPEnabled(args []string) bool {
	if len(args) == 0 {
		return true
	}
	taskName := args[0]
	manifest, loaded, err := config.LoadManifest(globalConfig)
	if err != nil || !loaded {
		return true // no config available; let remote handle it
	}
	if t, exists := manifest.Tasks[taskName]; exists && t.DisableMCP {
		return false
	}
	return true
}

// applyWorkingDir changes to the configured working directory if set.
func applyWorkingDir() error {
	if globalWorkingDir != "" {
		if err := os.Chdir(globalWorkingDir); err != nil {
			return fmt.Errorf("cannot change to directory %s: %w", globalWorkingDir, err)
		}
	}
	return nil
}

// tryRemoteExecute checks for a running server and routes the command through it.
// Returns (exitCode, true) if handled remotely, or (0, false) if no server found.
func tryRemoteExecute(subcmd string, args []string) (int, bool) {
	serverData, err := process.ReadServerFile(globalWorkingDir)
	if err != nil {
		return 0, false
	}
	if !process.IsProcessAlive(serverData.PID) || !process.ProbeHTTP(serverData.Addr) {
		fmt.Fprintf(os.Stderr, "error: server.json exists but the server is not running (PID %d dead).\n", serverData.PID)
		fmt.Fprintf(os.Stderr, "Remove %s to continue in local mode.\n", process.ServerRegistryFile)
		return 1, true
	}
	fmt.Fprintf(os.Stderr, "runbook: proxying to server at %s\n", serverData.Addr)
	return remoteExecute(serverData.Addr, subcmd, args), true
}

// runWithRemoteFallback handles the common pattern used by most subcommand RunE
// functions: apply working dir, try remote execution if not --local, then fall
// back to a local command function. The localFn receives the args and returns an
// exit code.
func runWithRemoteFallback(subcmd string, args []string, localFn func([]string) int) error {
	if err := applyWorkingDir(); err != nil {
		return err
	}
	if !globalLocal {
		if code, handled := tryRemoteExecute(subcmd, args); handled {
			if code != 0 {
				return &exitError{code: code}
			}
			return nil
		}
	}
	if code := localFn(args); code != 0 {
		return &exitError{code: code}
	}
	return nil
}

// extractGlobalFlagsManual scans raw args for --config, --working-dir, --local
// and returns their values plus the remaining args with those flags stripped.
// Used by DisableFlagParsing commands where Cobra doesn't parse persistent flags.
func extractGlobalFlagsManual(args []string) (configPath, workingDir string, local bool, remaining []string) {
	remaining = make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--local" || arg == "-local":
			local = true
		case arg == "--config" || arg == "-config":
			if i+1 < len(args) {
				configPath = args[i+1]
				i++
			}
		case strings.HasPrefix(arg, "--config=") || strings.HasPrefix(arg, "-config="):
			configPath = arg[strings.IndexByte(arg, '=')+1:]
		case arg == "--working-dir" || arg == "-working-dir":
			if i+1 < len(args) {
				workingDir = args[i+1]
				i++
			}
		case strings.HasPrefix(arg, "--working-dir=") || strings.HasPrefix(arg, "-working-dir="):
			workingDir = arg[strings.IndexByte(arg, '=')+1:]
		default:
			remaining = append(remaining, arg)
		}
	}
	return
}

// mergeExtractedGlobals merges manually-extracted global flags into the package-level vars.
func mergeExtractedGlobals(configPath, workingDir string, local bool) {
	if configPath != "" {
		globalConfig = configPath
	}
	if workingDir != "" {
		globalWorkingDir = workingDir
	}
	if local {
		globalLocal = true
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

func handleInit() error {
	targetPath := "./" + dirs.ConfigDir + "/tasks.yaml"
	if _, err := os.Stat(targetPath); err == nil {
		return fmt.Errorf("%s already exists (remove it or use the MCP 'init' tool with overwrite=true)", targetPath)
	}
	if err := os.MkdirAll(dirs.ConfigDir, 0755); err != nil {
		return fmt.Errorf("failed to create %s directory: %w", dirs.ConfigDir, err)
	}
	if err := os.WriteFile(targetPath, []byte(minimalConfig), 0644); err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}
	fmt.Printf("Successfully created %s\n", targetPath)
	fmt.Println("Edit this file to add your project's tasks, then start the MCP server.")
	return nil
}
