package cli

import (
	"flag"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"runbookmcp.dev/internal/logs"
)

func newLogsCmd() *cobra.Command {
	var (
		logsLines   int
		logsFilter  string
		logsSession string
		logsOffset  int
	)

	cmd := &cobra.Command{
		Use:   "logs <task>",
		Short: "Show task logs",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := applyWorkingDir(); err != nil {
				return err
			}
			// Logs always read locally (even when server is running).
			if code := execLogs(args[0], logsLines, logsFilter, logsSession, logsOffset); code != 0 {
				return &exitError{code: code}
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&logsLines, "lines", 0, "Number of lines to tail (0 = all)")
	cmd.Flags().StringVar(&logsFilter, "filter", "", "Regex pattern to filter lines")
	cmd.Flags().StringVar(&logsSession, "session", "", "Session ID to read from (default: latest)")
	cmd.Flags().IntVar(&logsOffset, "offset", 0, "Skip last N lines (for paging backwards through history)")

	return cmd
}

// cmdLogs accepts a raw arg slice (used by client.go's remoteExecute fallback).
func cmdLogs(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: runbook logs <task> [--lines=N] [--filter=REGEX] [--session=ID] [--offset=N]")
		return 1
	}

	taskName := args[0]
	flagArgs := args[1:]

	fs := flag.NewFlagSet("logs", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	lines := fs.Int("lines", 0, "Number of lines to tail (0 = all)")
	filter := fs.String("filter", "", "Regex pattern to filter lines")
	sessionID := fs.String("session", "", "Session ID to read from (default: latest)")
	offset := fs.Int("offset", 0, "Skip last N lines (for paging backwards through history)")

	if err := fs.Parse(flagArgs); err != nil {
		return 1
	}

	return execLogs(taskName, *lines, *filter, *sessionID, *offset)
}

// execLogs is the typed implementation shared by both entry points.
func execLogs(taskName string, lines int, filter string, sessionID string, offset int) int {
	manifest, _, _, err := bootstrap(globalConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	if _, exists := manifest.Tasks[taskName]; !exists {
		fmt.Fprintf(os.Stderr, "Error: task '%s' not found\n", taskName)
		return 1
	}

	opts := logs.ReadOptions{
		Lines:     lines,
		Filter:    filter,
		SessionID: sessionID,
		Offset:    offset,
	}

	logLines, _, err := logs.ReadLog(taskName, opts)
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
