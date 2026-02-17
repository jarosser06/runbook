package cli

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"runbookmcp.dev/internal/mcputil"
)

// newMCPClient creates, starts, and initializes an MCP HTTP client against addr.
// The returned cleanup function should be deferred by the caller.
func newMCPClient(addr string) (*mcpclient.Client, func(), error) {
	c, err := mcpclient.NewStreamableHttpClient(mcputil.Endpoint(addr))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create MCP client: %w", err)
	}

	ctx := context.Background()
	if err := c.Start(ctx); err != nil {
		return nil, nil, fmt.Errorf("failed to start MCP client: %w", err)
	}

	if _, err = c.Initialize(ctx, mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			ClientInfo: mcp.Implementation{
				Name:    "runbook-cli",
				Version: "0.0.1",
			},
		},
	}); err != nil {
		c.Close()
		return nil, nil, fmt.Errorf("failed to initialize MCP client: %w", err)
	}

	return c, func() { c.Close() }, nil
}

// remoteExecute routes a CLI command through the running HTTP server.
func remoteExecute(addr, subcmd string, args []string) int {
	c, cleanup, err := newMCPClient(addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to server at %s: %v\n", addr, err)
		return 1
	}
	defer cleanup()

	ctx := context.Background()
	switch subcmd {
	case "list":
		return remoteList(ctx, c)
	case "run":
		// Try as oneshot first; if not found, try as workflow group.
		return remoteRun(ctx, c, args)
	case "start":
		return remoteToolCall(ctx, c, "start_", args)
	case "stop":
		return remoteToolCall(ctx, c, "stop_", args)
	case "status":
		return remoteToolCall(ctx, c, "status_", args)
	case "logs":
		// Logs are stored as files on disk regardless of whether the server is running.
		// Read them locally rather than routing through the server.
		return cmdLogs(args)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", subcmd)
		return 1
	}
}

// remoteList fetches and displays tools from the remote server.
func remoteList(ctx context.Context, c *mcpclient.Client) int {
	result, err := c.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing tools: %v\n", err)
		return 1
	}
	if len(result.Tools) == 0 {
		fmt.Fprintln(os.Stderr, "No tasks available on server.")
		return 0
	}

	type entry struct{ name, kind, desc string }
	var tasks, daemons []entry
	var workflows []entry

	for _, t := range result.Tools {
		switch {
		case strings.HasPrefix(t.Name, "run_workflow_"):
			workflows = append(workflows, entry{t.Name[13:], "workflow", t.Description})
		case strings.HasPrefix(t.Name, "run_"):
			tasks = append(tasks, entry{t.Name[4:], "oneshot", t.Description})
		case strings.HasPrefix(t.Name, "start_"):
			desc := strings.TrimPrefix(t.Description, "Start daemon: ")
			daemons = append(daemons, entry{t.Name[6:], "daemon", desc})
		}
	}

	// Merge tasks and daemons into one sorted table (matches local list format).
	all := append(tasks, daemons...)
	sort.Slice(all, func(i, j int) bool { return all[i].name < all[j].name })
	sort.Slice(workflows, func(i, j int) bool { return workflows[i].name < workflows[j].name })

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)

	if len(all) > 0 {
		fmt.Fprintf(w, "%s\t%s\t%s\n",
			color(colorBold, "TASK"),
			color(colorBold, "TYPE"),
			color(colorBold, "DESCRIPTION"))
		for _, e := range all {
			fmt.Fprintf(w, "%s\t%s\t%s\n", e.name, e.kind, e.desc)
		}
	}

	if len(workflows) > 0 {
		if len(all) > 0 {
			fmt.Fprintln(w)
		}
		fmt.Fprintf(w, "%s\t%s\n",
			color(colorBold, "WORKFLOW"),
			color(colorBold, "DESCRIPTION"))
		for _, wf := range workflows {
			fmt.Fprintf(w, "%s\t%s\n", wf.name, wf.desc)
		}
	}

	w.Flush()
	return 0
}

// remoteRun handles "runbook run <task>" by trying oneshot first, then workflow.
func remoteRun(ctx context.Context, c *mcpclient.Client, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: runbook run <task> [--param=value...]")
		return 1
	}
	taskName := args[0]
	params := parseRawParams(args[1:])

	// Try oneshot tool first.
	code, found := callTool(ctx, c, "run_"+taskName, params)
	if found {
		return code
	}
	// Fall back to workflow group.
	code, found = callTool(ctx, c, "run_workflow_"+taskName, params)
	if found {
		return code
	}
	fmt.Fprintf(os.Stderr, "Error: task or workflow '%s' not found\n", taskName)
	return 1
}

// remoteToolCall invokes a named tool on the remote server and prints the result.
// prefix is "start_", "stop_", "status_", or "logs_".
// args should be [taskName, --param=value, ...]
func remoteToolCall(ctx context.Context, c *mcpclient.Client, prefix string, args []string) int {
	if len(args) == 0 {
		cmd := strings.TrimSuffix(prefix, "_")
		fmt.Fprintf(os.Stderr, "Usage: runbook %s <task> [--param=value...]\n", cmd)
		return 1
	}
	taskName := args[0]
	toolName := prefix + taskName
	params := parseRawParams(args[1:])

	code, found := callTool(ctx, c, toolName, params)
	if !found {
		fmt.Fprintf(os.Stderr, "Error: task '%s' not found\n", taskName)
		return 1
	}
	return code
}

// callTool calls the named tool and returns (exitCode, toolWasFound).
// It prints output and errors. Returns found=false only on "tool not found" errors.
func callTool(ctx context.Context, c *mcpclient.Client, toolName string, params map[string]any) (int, bool) {
	result, err := c.CallTool(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      toolName,
			Arguments: params,
		},
	})
	if err != nil {
		// NOTE: mcp-go returns a plain error whose message contains "not found"
		// when the requested tool doesn't exist. This string match is coupled to
		// the mcp-go implementation; if the library changes its error message this
		// sentinel check will break.
		if strings.Contains(err.Error(), "not found") {
			return 1, false
		}
		fmt.Fprintf(os.Stderr, "Error calling %s: %v\n", toolName, err)
		return 1, true
	}

	for _, content := range result.Content {
		if tc, ok := mcp.AsTextContent(content); ok {
			fmt.Println(tc.Text)
		}
	}
	if result.IsError {
		return 1, true
	}
	return 0, true
}

// parseRawParams parses --key=value and --key value flags into a map.
func parseRawParams(args []string) map[string]any {
	params := make(map[string]any)
	for i := 0; i < len(args); i++ {
		arg := args[i]
		// Strip leading dashes to get the key (handles -key and --key)
		stripped := strings.TrimLeft(arg, "-")
		if stripped == "" || stripped == arg {
			continue // not a flag
		}
		if idx := strings.IndexByte(stripped, '='); idx >= 0 {
			params[stripped[:idx]] = stripped[idx+1:]
		} else if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
			params[stripped] = args[i+1]
			i++
		}
	}
	return params
}

