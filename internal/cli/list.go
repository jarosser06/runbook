package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available tasks",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWithRemoteFallback("list", args, func(_ []string) int {
				return cmdList()
			})
		},
	}
}

func cmdList() int {
	manifest, _, _, err := bootstrap(globalConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	var taskNames []string
	for name := range manifest.Tasks {
		taskNames = append(taskNames, name)
	}
	sort.Strings(taskNames)

	var workflowNames []string
	for name := range manifest.Workflows {
		workflowNames = append(workflowNames, name)
	}
	sort.Strings(workflowNames)

	if len(taskNames) == 0 && len(workflowNames) == 0 {
		fmt.Fprintln(os.Stderr, "No tasks or workflows defined.")
		return 0
	}

	const colGap = 2

	if len(taskNames) > 0 {
		// Compute column widths from data (no ANSI codes involved).
		// Include param label lengths in col1 so they never bleed into the TYPE column.
		col1 := len("TASK")
		col2 := len("TYPE")
		for _, name := range taskNames {
			if len(name) > col1 {
				col1 = len(name)
			}
			t := manifest.Tasks[name]
			if len(string(t.Type)) > col2 {
				col2 = len(string(t.Type))
			}
			for pn, p := range t.Parameters {
				var plainLabel string
				if p.Required {
					plainLabel = fmt.Sprintf("  --%s (required)", pn)
				} else if p.Default != nil {
					plainLabel = fmt.Sprintf("  --%s [default: %v]", pn, *p.Default)
				} else {
					plainLabel = fmt.Sprintf("  --%s", pn)
				}
				if len(plainLabel) > col1 {
					col1 = len(plainLabel)
				}
			}
		}

		// Header: color the words, pad with plain spaces so alignment is exact
		fmt.Printf("%s%s  %s%s  %s\n",
			color(colorBold, "TASK"), strings.Repeat(" ", col1-len("TASK")),
			color(colorBold, "TYPE"), strings.Repeat(" ", col2-len("TYPE")),
			color(colorBold, "DESCRIPTION"))

		for _, name := range taskNames {
			t := manifest.Tasks[name]
			fmt.Printf("%-*s  %-*s  %s\n", col1, name, col2, string(t.Type), t.Description)

			if len(t.Parameters) > 0 {
				var paramNames []string
				for pn := range t.Parameters {
					paramNames = append(paramNames, pn)
				}
				sort.Strings(paramNames)

				for _, pn := range paramNames {
					p := t.Parameters[pn]

					// Param rows: col1=label, col2=empty, col3=description.
					// Using %-*s for col1 ensures description aligns with DESCRIPTION header.
					var displayLabel string
					if p.Required {
						displayLabel = fmt.Sprintf("  --%s %s", pn, color(colorRed, "(required)"))
						plainLabel := fmt.Sprintf("  --%s (required)", pn)
						fmt.Printf("%s%s  %-*s  %s\n", displayLabel, strings.Repeat(" ", col1-len(plainLabel)), col2, "", p.Description)
					} else if p.Default != nil {
						displayLabel = fmt.Sprintf("  --%s [default: %v]", pn, *p.Default)
						fmt.Printf("%-*s  %-*s  %s\n", col1, displayLabel, col2, "", p.Description)
					} else {
						displayLabel = fmt.Sprintf("  --%s", pn)
						fmt.Printf("%-*s  %-*s  %s\n", col1, displayLabel, col2, "", p.Description)
					}
				}
			}
		}
	}

	if len(workflowNames) > 0 {
		if len(taskNames) > 0 {
			fmt.Println()
		}

		col1 := len("WORKFLOW")
		col2 := len("STEPS")
		for _, name := range workflowNames {
			if len(name) > col1 {
				col1 = len(name)
			}
			wf := manifest.Workflows[name]
			var steps []string
			for _, s := range wf.Steps {
				steps = append(steps, s.Task)
			}
			if stepsStr := strings.Join(steps, " -> "); len(stepsStr) > col2 {
				col2 = len(stepsStr)
			}
		}

		fmt.Printf("%s%s  %s%s  %s\n",
			color(colorBold, "WORKFLOW"), strings.Repeat(" ", col1-len("WORKFLOW")),
			color(colorBold, "STEPS"), strings.Repeat(" ", col2-len("STEPS")),
			color(colorBold, "DESCRIPTION"))

		for _, name := range workflowNames {
			wf := manifest.Workflows[name]
			var steps []string
			for _, s := range wf.Steps {
				steps = append(steps, s.Task)
			}
			fmt.Printf("%-*s  %-*s  %s\n", col1, name, col2, strings.Join(steps, " -> "), wf.Description)
		}
	}

	return 0
}
