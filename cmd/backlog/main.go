package main

import (
	"fmt"
	"os"

	"github.com/mazen160/backlog/internal/cli"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

func main() {
	cli.SetVersion(version)
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
