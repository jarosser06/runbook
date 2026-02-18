package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"runbookmcp.dev/internal/config"
)

func newStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:                "start <task> [--param=value...]",
		Short:              "Start a daemon",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, a := range args {
				if a == "--help" || a == "-h" {
					return cmd.Help()
				}
			}
			extractedConfig, extractedWorkingDir, extractedLocal, remaining := extractGlobalFlagsManual(args)
			mergeExtractedGlobals(extractedConfig, extractedWorkingDir, extractedLocal)

			if err := applyWorkingDir(); err != nil {
				return err
			}
			if !globalLocal {
				if code, handled := tryRemoteExecute("start", remaining); handled {
					if code != 0 {
						return &exitError{code: code}
					}
					return nil
				}
			}
			if code := cmdStart(remaining); code != 0 {
				return &exitError{code: code}
			}
			return nil
		},
	}
}

func newStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop <task>",
		Short: "Stop a daemon",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWithRemoteFallback("stop", args, func(a []string) int {
				return cmdStop(a[0])
			})
		},
	}
}

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <task>",
		Short: "Show daemon status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWithRemoteFallback("status", args, func(a []string) int {
				return cmdStatus(a[0])
			})
		},
	}
}

func cmdStart(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: runbook start <task> [--param=value...]")
		return 1
	}

	taskName := args[0]
	taskArgs := args[1:]

	manifest, manager, _, err := bootstrap(globalConfig)
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

func cmdStop(taskName string) int {
	_, manager, _, err := bootstrap(globalConfig)
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

func cmdStatus(taskName string) int {
	_, manager, _, err := bootstrap(globalConfig)
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
