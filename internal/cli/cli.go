package cli

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/jarosser06/runbook/internal/config"
	"github.com/jarosser06/runbook/internal/logs"
	"github.com/jarosser06/runbook/internal/process"
	"github.com/jarosser06/runbook/internal/task"
)

// Execute dispatches the CLI subcommand and returns the exit code.
// args should be os.Args[1:] (i.e., starting with the subcommand name).
func Execute(args []string) int {
	if len(args) == 0 {
		printUsage()
		return 1
	}

	subcmd := args[0]
	remaining := args[1:]

	switch subcmd {
	case "list":
		return cmdList(remaining)
	case "run":
		return cmdRun(remaining)
	case "start":
		return cmdStart(remaining)
	case "stop":
		return cmdStop(remaining)
	case "status":
		return cmdStatus(remaining)
	case "logs":
		return cmdLogs(remaining)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", subcmd)
		printUsage()
		return 1
	}
}

// bootstrap loads config and creates the task manager, mirroring main.go setup.
func bootstrap(configPath string) (*config.Manifest, *task.Manager, *process.Manager, error) {
	if err := logs.Setup(); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to setup logs: %w", err)
	}

	manifest, loaded, err := config.LoadManifest(configPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to load config: %w", err)
	}
	if !loaded {
		return nil, nil, nil, fmt.Errorf("no config file found (use --config or create .dev_workflow.yaml)")
	}

	processManager := process.NewManager()
	taskManager := task.NewManager(manifest, processManager)
	return manifest, taskManager, processManager, nil
}

// parseGlobalFlags extracts --config from args and returns (configPath, remainingArgs).
// It uses a dedicated FlagSet so task-specific flags are left untouched.
func parseGlobalFlags(args []string) (configPath string, remaining []string) {
	fs := flag.NewFlagSet("global", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	cfg := fs.String("config", "", "Path to task manifest file or directory")

	// Separate global flags from the rest.
	// Global flags come before the task name.
	var globalArgs []string
	for i, arg := range args {
		if strings.HasPrefix(arg, "--config") || strings.HasPrefix(arg, "-config") {
			if strings.Contains(arg, "=") {
				globalArgs = append(globalArgs, arg)
			} else if i+1 < len(args) {
				globalArgs = append(globalArgs, arg, args[i+1])
			}
		}
	}

	// Parse only global flags
	_ = fs.Parse(globalArgs)

	// Remaining = everything except --config and its value
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "--config") || strings.HasPrefix(arg, "-config") {
			if strings.Contains(arg, "=") {
				continue // skip --config=value
			}
			i++ // skip next arg (the value)
			continue
		}
		remaining = append(remaining, arg)
	}

	return *cfg, remaining
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

	// Register a string flag for each parameter
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

	// Collect values: only include flags that were explicitly set or have defaults
	for name, ptr := range flagPtrs {
		if *ptr != "" {
			params[name] = *ptr
		}
	}

	// Check required parameters
	for name, param := range taskDef.Parameters {
		if param.Required {
			if _, ok := params[name]; !ok {
				return nil, fmt.Errorf("required parameter --%s is missing", name)
			}
		}
	}

	return params, nil
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage: runbook <command> [options]")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Commands:")
	fmt.Fprintln(os.Stderr, "  list                              List available tasks")
	fmt.Fprintln(os.Stderr, "  run <task> [--param=value...]      Run a oneshot task or workflow")
	fmt.Fprintln(os.Stderr, "  start <task> [--param=value...]    Start a daemon")
	fmt.Fprintln(os.Stderr, "  stop <task>                        Stop a daemon")
	fmt.Fprintln(os.Stderr, "  status <task>                      Show daemon status")
	fmt.Fprintln(os.Stderr, "  logs <task> [options]               Show task logs")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Global options:")
	fmt.Fprintln(os.Stderr, "  --config=PATH    Path to task manifest file or directory")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Server mode (original behavior):")
	fmt.Fprintln(os.Stderr, "  runbook                            Start MCP server (stdio)")
	fmt.Fprintln(os.Stderr, "  runbook -serve -addr :8080         Start MCP server (HTTP)")
	fmt.Fprintln(os.Stderr, "  runbook -init                      Initialize config file")
}
