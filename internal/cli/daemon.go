package cli

import (
	"fmt"
	"os"

	"github.com/jarosser06/runbook/internal/config"
)

func cmdStart(args []string) int {
	configPath, remaining := parseGlobalFlags(args)
	if len(remaining) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: runbook start <task> [--param=value...]")
		return 1
	}

	taskName := remaining[0]
	taskArgs := remaining[1:]

	manifest, manager, _, err := bootstrap(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	taskDef, exists := manifest.Tasks[taskName]
	if !exists {
		fmt.Fprintf(os.Stderr, "Error: task '%s' not found\n", taskName)
		return 1
	}

	if taskDef.Type != config.TaskTypeDaemon {
		fmt.Fprintf(os.Stderr, "Error: '%s' is not a daemon task. Use 'runbook run %s' instead.\n", taskName, taskName)
		return 1
	}

	params, err := parseTaskParams(taskDef, taskArgs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	result, err := manager.StartDaemon(taskName, params)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	printDaemonStartResult(result)

	if !result.Success {
		return 1
	}
	return 0
}

func cmdStop(args []string) int {
	configPath, remaining := parseGlobalFlags(args)
	if len(remaining) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: runbook stop <task>")
		return 1
	}

	taskName := remaining[0]

	_, manager, _, err := bootstrap(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	result, err := manager.StopDaemon(taskName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	printDaemonStopResult(result)

	if !result.Success {
		return 1
	}
	return 0
}

func cmdStatus(args []string) int {
	configPath, remaining := parseGlobalFlags(args)
	if len(remaining) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: runbook status <task>")
		return 1
	}

	taskName := remaining[0]

	_, manager, _, err := bootstrap(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	status, err := manager.DaemonStatus(taskName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	printDaemonStatus(status)
	return 0
}
