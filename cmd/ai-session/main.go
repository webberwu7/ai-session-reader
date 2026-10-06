package main

import (
	"fmt"
	"github.com/webberwu7/ai-session-reader/internal/cli"
	"os"
)

func main() {
	if err := cli.Run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
