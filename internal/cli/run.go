package cli

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"runbookmcp.dev/internal/config"
	"runbookmcp.dev/internal/task"
)

func newRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:                "run <task> [--param=value...]",
		Short:              "Run a oneshot task or workflow",
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
				if code, handled := tryRemoteExecute("run", remaining); handled {
					if code != 0 {
						return &exitError{code: code}
					}
					return nil
				}
			}
			if code := cmdRun(remaining); code != 0 {
				return &exitError{code: code}
			}
			return nil
		},
	}
}

func cmdRun(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: runbook run <task> [--param=value...]")
		return 1
	}

	taskName := args[0]
	taskArgs := args[1:]

	manifest, manager, _, err := bootstrap(globalConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	// Check if it's a workflow
	if wfDef, isWorkflow := manifest.Workflows[taskName]; isWorkflow {
		return runWorkflow(manager, taskName, wfDef, taskArgs)
	}

	// Check if task exists
	taskDef, exists := manifest.Tasks[taskName]
	if !exists {
		fmt.Fprintf(os.Stderr, "Error: task '%s' not found\n", taskName)
		printAvailable(manifest)
		return 1
	}

	// Suggest 'start' for daemons
	if taskDef.Type == config.TaskTypeDaemon {
		fmt.Fprintf(os.Stderr, "Error: '%s' is a daemon task. Use 'runbook start %s' instead.\n", taskName, taskName)
		return 1
	}

	// Parse task parameters
	params, err := parseTaskParams(taskDef, taskArgs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	// Execute
	result, err := manager.ExecuteOneShot(taskName, params)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	printExecutionResult(result)

	if !result.Success {
		if result.ExitCode != 0 {
			return result.ExitCode
		}
		return 1
	}
	return 0
}

func runWorkflow(manager *task.Manager, workflowName string, wfDef config.Workflow, args []string) int {
	params, err := parseWorkflowParams(wfDef, args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	result, err := manager.ExecuteWorkflow(workflowName, params)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	printWorkflowResult(result)

	if !result.Success {
		return 1
	}
	return 0
}

// parseWorkflowParams parses --key=value flags for a workflow based on its parameter definitions.
func parseWorkflowParams(wfDef config.Workflow, args []string) (map[string]interface{}, error) {
	params := make(map[string]interface{})
	if len(wfDef.Parameters) == 0 {
		if len(args) > 0 {
			return nil, fmt.Errorf("workflow does not accept parameters, but got: %s", strings.Join(args, " "))
		}
		return params, nil
	}

	fs := flag.NewFlagSet("workflow-params", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	flagPtrs := make(map[string]*string)
	for name, param := range wfDef.Parameters {
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

	for name, param := range wfDef.Parameters {
		if param.Required {
			if _, ok := params[name]; !ok {
				return nil, fmt.Errorf("required parameter --%s is missing", name)
			}
		}
	}

	return params, nil
}

func printAvailable(manifest *config.Manifest) {
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Available tasks:")
	for name := range manifest.Tasks {
		fmt.Fprintf(os.Stderr, "  %s\n", name)
	}
	for name := range manifest.Workflows {
		fmt.Fprintf(os.Stderr, "  %s (workflow)\n", name)
	}
}
