package mcputil

import "testing"

func TestEndpoint(t *testing.T) {
	tests := []struct {
		addr string
		want string
	}{
		{"http://localhost:8080", "http://localhost:8080/mcp"},
		{"http://localhost:8080/", "http://localhost:8080/mcp"},
		{"http://localhost:8080/mcp", "http://localhost:8080/mcp"},
		{"http://localhost:8080/mcp/", "http://localhost:8080/mcp"},
		{"http://localhost:9999", "http://localhost:9999/mcp"},
		{"http://localhost:9999/mcp", "http://localhost:9999/mcp"},
	}
	for _, tt := range tests {
		got := Endpoint(tt.addr)
		if got != tt.want {
			t.Errorf("Endpoint(%q) = %q, want %q", tt.addr, got, tt.want)
		}
	}
}
