package main

import "runbookmcp.dev/internal/cli"

var (
	// These variables are set at build time via -ldflags
	version = "dev"
	commit  = "none"    //nolint:unused
	date    = "unknown" //nolint:unused
)

func main() {
	cli.Execute(version)
}
