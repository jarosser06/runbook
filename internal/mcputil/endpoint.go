package mcputil

import "strings"

// Endpoint normalizes an MCP server base address to include the /mcp path,
// since mcp-go's StreamableHTTPServer registers all handlers at /mcp by default.
func Endpoint(addr string) string {
	addr = strings.TrimRight(addr, "/")
	if !strings.HasSuffix(addr, "/mcp") {
		return addr + "/mcp"
	}
	return addr
}
