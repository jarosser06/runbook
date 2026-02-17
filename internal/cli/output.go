package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"runbookmcp.dev/internal/task"
)

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
	colorDim    = "\033[2m"
)

// isTerminal returns true if the given file is a terminal.
func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// color wraps text in ANSI color if stderr is a terminal.
func color(code, text string) string {
	if !isTerminal(os.Stderr) {
		return text
	}
	return code + text + colorReset
}

// printExecutionResult prints a oneshot execution result with human-friendly formatting.
// Task output goes to stdout (pipeable), metadata goes to stderr.
func printExecutionResult(r *task.ExecutionResult) {
	// Print task output to stdout
	if r.Stdout != "" {
		fmt.Print(r.Stdout)
		if !strings.HasSuffix(r.Stdout, "\n") {
			fmt.Println()
		}
	}
	if r.Stderr != "" {
		fmt.Fprint(os.Stderr, r.Stderr)
		if !strings.HasSuffix(r.Stderr, "\n") {
			fmt.Fprintln(os.Stderr)
		}
	}

	// Print summary to stderr
	fmt.Fprintln(os.Stderr)
	if r.Success {
		fmt.Fprintf(os.Stderr, "%s  %s\n",
			color(colorGreen+colorBold, "[OK]"),
			color(colorDim, formatDuration(r.Duration)))
	} else if r.TimedOut {
		fmt.Fprintf(os.Stderr, "%s  %s\n",
			color(colorYellow+colorBold, "[TIMEOUT]"),
			color(colorDim, formatDuration(r.Duration)))
	} else {
		fmt.Fprintf(os.Stderr, "%s  exit code %d  %s\n",
			color(colorRed+colorBold, "[FAIL]"),
			r.ExitCode,
			color(colorDim, formatDuration(r.Duration)))
	}
	if r.Error != "" {
		fmt.Fprintf(os.Stderr, "%s %s\n", color(colorRed, "Error:"), r.Error)
	}
	if r.SessionID != "" {
		fmt.Fprintf(os.Stderr, "%s %s\n", color(colorDim, "Session:"), r.SessionID)
	}
}

// printWorkflowResult prints a workflow execution result with human-friendly formatting.
func printWorkflowResult(r *task.WorkflowResult) {
	fmt.Fprintln(os.Stderr)
	for _, step := range r.Steps {
		if step.Skipped {
			fmt.Fprintf(os.Stderr, "  %s %s\n",
				color(colorDim, "[SKIP]"),
				step.TaskName)
			continue
		}
		if step.Result == nil {
			continue
		}
		// Print step output to stdout
		if step.Result.Stdout != "" {
			fmt.Print(step.Result.Stdout)
			if !strings.HasSuffix(step.Result.Stdout, "\n") {
				fmt.Println()
			}
		}
		if step.Result.Stderr != "" {
			fmt.Fprint(os.Stderr, step.Result.Stderr)
			if !strings.HasSuffix(step.Result.Stderr, "\n") {
				fmt.Fprintln(os.Stderr)
			}
		}
		if step.Result.Success {
			fmt.Fprintf(os.Stderr, "  %s %s  %s\n",
				color(colorGreen, "[OK]"),
				step.TaskName,
				color(colorDim, formatDuration(step.Result.Duration)))
		} else {
			fmt.Fprintf(os.Stderr, "  %s %s  exit code %d  %s\n",
				color(colorRed, "[FAIL]"),
				step.TaskName,
				step.Result.ExitCode,
				color(colorDim, formatDuration(step.Result.Duration)))
		}
	}

	// Summary
	fmt.Fprintln(os.Stderr)
	if r.Success {
		fmt.Fprintf(os.Stderr, "%s  %d steps  %s\n",
			color(colorGreen+colorBold, "[OK]"),
			r.StepsRun,
			color(colorDim, formatDuration(r.Duration)))
	} else {
		fmt.Fprintf(os.Stderr, "%s  %d/%d steps failed  %s\n",
			color(colorRed+colorBold, "[FAIL]"),
			r.StepsFailed, r.StepsRun,
			color(colorDim, formatDuration(r.Duration)))
	}
	if r.Error != "" {
		fmt.Fprintf(os.Stderr, "%s %s\n", color(colorRed, "Error:"), r.Error)
	}
}

// printDaemonStartResult prints a daemon start result.
func printDaemonStartResult(r *task.DaemonStartResult) {
	if r.Success {
		fmt.Fprintf(os.Stderr, "%s  PID %d\n",
			color(colorGreen+colorBold, "[STARTED]"),
			r.PID)
		fmt.Fprintf(os.Stderr, "%s %s\n", color(colorDim, "Logs:"), r.LogPath)
		if r.SessionID != "" {
			fmt.Fprintf(os.Stderr, "%s %s\n", color(colorDim, "Session:"), r.SessionID)
		}
	} else {
		fmt.Fprintf(os.Stderr, "%s %s\n",
			color(colorRed+colorBold, "[ERROR]"),
			r.Error)
	}
}

// printDaemonStopResult prints a daemon stop result.
func printDaemonStopResult(r *task.DaemonStopResult) {
	if r.Success {
		fmt.Fprintf(os.Stderr, "%s\n", color(colorGreen+colorBold, "[STOPPED]"))
	} else {
		fmt.Fprintf(os.Stderr, "%s %s\n",
			color(colorRed+colorBold, "[ERROR]"),
			r.Error)
	}
}

// printDaemonStatus prints daemon status information.
func printDaemonStatus(s *task.DaemonStatus) {
	if s.Running {
		fmt.Fprintf(os.Stderr, "%s  PID %d\n",
			color(colorGreen+colorBold, "[RUNNING]"),
			s.PID)
		if s.LogPath != "" {
			fmt.Fprintf(os.Stderr, "%s %s\n", color(colorDim, "Logs:"), s.LogPath)
		}
		if s.SessionID != "" {
			fmt.Fprintf(os.Stderr, "%s %s\n", color(colorDim, "Session:"), s.SessionID)
		}
	} else {
		fmt.Fprintf(os.Stderr, "%s\n", color(colorYellow+colorBold, "[STOPPED]"))
	}
}

// formatDuration formats a duration for human display.
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
}
