package cli

import (
	"flag"
	"fmt"
	"os"

	"runbookmcp.dev/internal/logs"
)

func cmdLogs(args []string) int {
	configPath, remaining := parseGlobalFlags(args)

	if len(remaining) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: runbook logs <task> [--lines=N] [--filter=REGEX] [--session=ID]")
		return 1
	}

	// First positional arg is the task name
	taskName := remaining[0]
	flagArgs := remaining[1:]

	// Parse logs-specific flags
	fs := flag.NewFlagSet("logs", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	lines := fs.Int("lines", 0, "Number of lines to tail (0 = all)")
	filter := fs.String("filter", "", "Regex pattern to filter lines")
	sessionID := fs.String("session", "", "Session ID to read from (default: latest)")

	if err := fs.Parse(flagArgs); err != nil {
		return 1
	}

	// Bootstrap to validate config
	manifest, _, _, err := bootstrap(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	if _, exists := manifest.Tasks[taskName]; !exists {
		fmt.Fprintf(os.Stderr, "Error: task '%s' not found\n", taskName)
		return 1
	}

	opts := logs.ReadOptions{
		Lines:     *lines,
		Filter:    *filter,
		SessionID: *sessionID,
	}

	logLines, err := logs.ReadLog(taskName, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	if len(logLines) == 0 {
		fmt.Fprintln(os.Stderr, "No log output found.")
		return 0
	}

	for _, line := range logLines {
		fmt.Println(line)
	}
	return 0
}
