package main

import (
	"fmt"
	"os"

	"github.com/steipete/camsnap/internal/cli"
)

var version = "0.1.0"

func main() {
	root := cli.NewRootCommand(version)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
