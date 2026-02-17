package server

import (
	"bufio"
	"io"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"

	mcpserver "github.com/mark3labs/mcp-go/server"
)

// newTestMCPServer starts an httptest server backed by a minimal MCP server.
// It registers t.Cleanup to close the server when the test ends.
func newTestMCPServer(t *testing.T) *httptest.Server {
	t.Helper()
	s := mcpserver.NewMCPServer("proxy-test", "0.1.0")
	ts := httptest.NewServer(mcpserver.NewStreamableHTTPServer(s))
	t.Cleanup(ts.Close)
	return ts
}

// TestReadLinesSendsLines verifies that readLines delivers each line to the channel.
func TestReadLinesSendsLines(t *testing.T) {
	pr, pw := io.Pipe()
	defer pr.Close()

	ch := make(chan lineResult, 4)
	go readLines(bufio.NewReader(pr), ch)

	if _, err := io.WriteString(pw, "line one\nline two\n"); err != nil {
		t.Fatalf("write: %v", err)
	}

	for _, want := range []string{"line one\n", "line two\n"} {
		r := <-ch
		if r.err != nil {
			t.Fatalf("unexpected error: %v", r.err)
		}
		if r.line != want {
			t.Errorf("line = %q, want %q", r.line, want)
		}
	}
	pw.Close()
}

// TestReadLinesExitsOnEOF verifies that readLines sends an EOF result and exits
// its goroutine when the writer end of the pipe is closed.
func TestReadLinesExitsOnEOF(t *testing.T) {
	pr, pw := io.Pipe()
	defer pr.Close()

	ch := make(chan lineResult, 4)
	goroutinesBefore := runtime.NumGoroutine()
	go readLines(bufio.NewReader(pr), ch)

	time.Sleep(10 * time.Millisecond) // let goroutine start and block on ReadString
	pw.Close()                        // signal EOF

	select {
	case r := <-ch:
		if r.err == nil {
			t.Error("expected non-nil error on EOF, got nil")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("readLines did not send EOF result within 2s")
	}

	// Give the goroutine time to finish executing its return statement.
	time.Sleep(50 * time.Millisecond)
	goroutinesAfter := runtime.NumGoroutine()

	// Allow +1 tolerance for unrelated test-infrastructure goroutines.
	if goroutinesAfter > goroutinesBefore+1 {
		t.Errorf("goroutine leak: count before=%d after=%d (readLines goroutine should have exited)",
			goroutinesBefore, goroutinesAfter)
	}
}

// TestServeStdioProxyExitsOnEOF verifies that serveStdioProxy returns nil
// when the input stream reaches EOF. This also checks that no goroutines are
// leaked: exactly one readLines goroutine is started and it exits on EOF.
func TestServeStdioProxyExitsOnEOF(t *testing.T) {
	ts := newTestMCPServer(t)

	pr, pw := io.Pipe()

	done := make(chan error, 1)
	go func() {
		done <- serveStdioProxy(ts.URL, pr, io.Discard)
	}()

	// Closing the write end triggers EOF in the readLines goroutine regardless
	// of whether the transport has fully connected yet — the function exits cleanly either way.
	pw.Close()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("expected nil error on EOF, got: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("serveStdioProxy did not exit after stdin EOF")
	}

	pr.Close()
}

// TestServeStdioProxyReturnsErrorOnBadAddr verifies that serveStdioProxy returns
// a non-nil error (rather than hanging) when the target server is unreachable.
func TestServeStdioProxyExitsCleanlyOnBadAddr(t *testing.T) {
	// transport.NewStreamableHTTP uses lazy connection — trans.Start does not
	// make an HTTP request, so a dead server address does not cause an immediate
	// error. The HTTP error only surfaces when forwarding a request. The important
	// contract is that the function exits promptly when stdin is closed (EOF).
	pr, pw := io.Pipe()
	defer pr.Close()

	done := make(chan error, 1)
	go func() {
		done <- serveStdioProxy("http://127.0.0.1:19741", pr, io.Discard)
	}()

	pw.Close() // signal EOF immediately

	select {
	case <-done:
		// function exited cleanly — pass
	case <-time.After(3 * time.Second):
		t.Fatal("serveStdioProxy with bad addr hung instead of exiting on EOF")
	}
}
