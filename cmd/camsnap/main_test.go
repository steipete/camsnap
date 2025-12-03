package main

import (
	"bytes"
	"testing"

	"github.com/steipete/camsnap/internal/cli"
)

func TestRootVersionFlag(t *testing.T) {
	root := cli.NewRootCommand("test-version")
	root.SetArgs([]string{"--version"})
	var buf bytes.Buffer
	root.SetOut(&buf)
	if err := root.Execute(); err != nil {
		t.Fatalf("--version execute: %v", err)
	}
	if buf.String() == "" {
		t.Fatalf("expected version output")
	}
}
