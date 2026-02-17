package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
)

func cmdList(args []string) int {
	configPath, remaining := parseGlobalFlags(args)
	if len(remaining) > 0 {
		fmt.Fprintf(os.Stderr, "list does not accept arguments: %s\n", strings.Join(remaining, " "))
		return 1
	}

	manifest, _, _, err := bootstrap(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	// Collect and sort task names
	var taskNames []string
	for name := range manifest.Tasks {
		taskNames = append(taskNames, name)
	}
	sort.Strings(taskNames)

	// Collect and sort workflow names
	var workflowNames []string
	for name := range manifest.Workflows {
		workflowNames = append(workflowNames, name)
	}
	sort.Strings(workflowNames)

	if len(taskNames) == 0 && len(workflowNames) == 0 {
		fmt.Fprintln(os.Stderr, "No tasks or workflows defined.")
		return 0
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)

	if len(taskNames) > 0 {
		fmt.Fprintf(w, "%s\t%s\t%s\n",
			color(colorBold, "TASK"),
			color(colorBold, "TYPE"),
			color(colorBold, "DESCRIPTION"))

		for _, name := range taskNames {
			t := manifest.Tasks[name]
			typeName := string(t.Type)
			fmt.Fprintf(w, "%s\t%s\t%s\n", name, typeName, t.Description)

			// Show parameters
			if len(t.Parameters) > 0 {
				var paramNames []string
				for pn := range t.Parameters {
					paramNames = append(paramNames, pn)
				}
				sort.Strings(paramNames)
				for _, pn := range paramNames {
					p := t.Parameters[pn]
					parts := []string{fmt.Sprintf("  --%s", pn)}
					if p.Required {
						parts = append(parts, color(colorRed, "(required)"))
					}
					if p.Default != nil {
						parts = append(parts, fmt.Sprintf("[default: %v]", *p.Default))
					}
					desc := ""
					if p.Description != "" {
						desc = p.Description
					}
					fmt.Fprintf(w, "%s\t\t%s\n", strings.Join(parts, " "), desc)
				}
			}
		}
	}

	if len(workflowNames) > 0 {
		if len(taskNames) > 0 {
			fmt.Fprintln(w)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n",
			color(colorBold, "WORKFLOW"),
			color(colorBold, "STEPS"),
			color(colorBold, "DESCRIPTION"))

		for _, name := range workflowNames {
			wf := manifest.Workflows[name]
			var stepNames []string
			for _, s := range wf.Steps {
				stepNames = append(stepNames, s.Task)
			}
			fmt.Fprintf(w, "%s\t%s\t%s\n", name, strings.Join(stepNames, " -> "), wf.Description)
		}
	}

	w.Flush()
	return 0
}
