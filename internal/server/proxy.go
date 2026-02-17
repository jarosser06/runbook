package server

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"runbookmcp.dev/internal/mcputil"
)

// ServeStdioProxy forwards stdin MCP traffic to a running HTTP MCP server and
// writes responses to stdout. It allows stdio MCP clients (e.g. Claude Desktop)
// to transparently use a shared running HTTP server instance.
func ServeStdioProxy(addr string) error {
	return serveStdioProxy(addr, os.Stdin, os.Stdout)
}

// lineResult carries one read result from the stdin reader goroutine.
type lineResult struct {
	line string
	err  error
}

// readLines reads newline-delimited lines from r and sends each result to ch.
// It runs until r returns a non-nil error (including io.EOF), then exits.
// Callers should launch this in a goroutine; exactly one goroutine is used
// per call, avoiding per-line goroutine spawning.
func readLines(r *bufio.Reader, ch chan<- lineResult) {
	for {
		line, err := r.ReadString('\n')
		ch <- lineResult{line, err}
		if err != nil {
			return
		}
	}
}

// serveStdioProxy is the testable core of ServeStdioProxy. Accepting in/out
// instead of os.Stdin/Stdout allows tests to pass pipe readers/writers.
func serveStdioProxy(addr string, in io.Reader, out io.Writer) error {
	trans, err := transport.NewStreamableHTTP(mcputil.Endpoint(addr))
	if err != nil {
		return fmt.Errorf("failed to create HTTP transport: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := trans.Start(ctx); err != nil {
		return fmt.Errorf("failed to start HTTP transport: %w", err)
	}
	defer trans.Close()

	var writeMu sync.Mutex
	writeMsg := func(v any) {
		b, err := json.Marshal(v)
		if err != nil {
			return
		}
		writeMu.Lock()
		fmt.Fprintf(out, "%s\n", b)
		writeMu.Unlock()
	}

	// Forward server-to-client notifications to the output stream.
	trans.SetNotificationHandler(func(notif mcp.JSONRPCNotification) {
		writeMsg(notif)
	})

	// Signal handling — cancel ctx on SIGTERM/interrupt so the loop exits.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan)
	go func() {
		select {
		case <-sigChan:
			cancel()
		case <-ctx.Done():
		}
	}()

	// One persistent goroutine reads lines from in. This avoids the goroutine
	// leak that would occur if a new goroutine were spawned per ReadString call:
	// a goroutine blocked on ReadString cannot be unblocked by context cancellation,
	// so it would leak until the underlying reader is closed.
	lineCh := make(chan lineResult, 16)
	go readLines(bufio.NewReader(in), lineCh)

	for {
		select {
		case <-ctx.Done():
			return nil
		case r := <-lineCh:
			if r.err != nil {
				if r.err == io.EOF {
					return nil
				}
				return r.err
			}
			line := strings.TrimRight(r.line, "\r\n")
			if line == "" {
				continue
			}

			// Peek at the ID to distinguish requests (have id) from notifications (no id).
			var peek struct {
				ID mcp.RequestId `json:"id"`
			}
			if err := json.Unmarshal([]byte(line), &peek); err != nil {
				writeMsg(map[string]any{
					"jsonrpc": "2.0",
					"error": map[string]any{
						"code":    -32700,
						"message": "Parse error",
					},
				})
				continue
			}

			if peek.ID.IsNil() {
				// Notification — forward without expecting a response.
				var notif mcp.JSONRPCNotification
				if err := json.Unmarshal([]byte(line), &notif); err == nil {
					_ = trans.SendNotification(ctx, notif)
				}
			} else {
				// Request — forward and relay the response.
				var req transport.JSONRPCRequest
				if err := json.Unmarshal([]byte(line), &req); err != nil {
					writeMsg(map[string]any{
						"jsonrpc": "2.0",
						"id":      peek.ID,
						"error": map[string]any{
							"code":    -32700,
							"message": "Parse error",
						},
					})
					continue
				}

				resp, err := trans.SendRequest(ctx, req)
				if err != nil {
					writeMsg(map[string]any{
						"jsonrpc": "2.0",
						"id":      peek.ID,
						"error": map[string]any{
							"code":    -32603,
							"message": err.Error(),
						},
					})
					continue
				}
				writeMsg(resp)
			}
		}
	}
}
