// Command gen-docs generates Markdown CLI reference pages for all backlogit
// subcommands using cobra's doc generation utilities.
//
// Usage:
//
//	go run ./cmd/gen-docs [outdir]
//
// outdir defaults to docs/cli-reference.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra/doc"

	"github.com/softwaresalt/backlogit/internal/cli"
)

func main() {
	outDir := "docs/cli-reference"
	if len(os.Args) > 1 {
		outDir = os.Args[1]
	}
	if err := generateDocs(outDir); err != nil {
		fmt.Fprintf(os.Stderr, "gen-docs: %v\n", err)
		os.Exit(1)
	}
}

// generateDocs writes Markdown CLI reference pages for all backlogit commands
// into outDir. DisableAutoGenTag is set to true for deterministic output.
func generateDocs(outDir string) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}
	root := cli.NewRootCommand()
	root.DisableAutoGenTag = true
	// Propagate to all subcommands.
	for _, sub := range root.Commands() {
		sub.DisableAutoGenTag = true
		for _, child := range sub.Commands() {
			child.DisableAutoGenTag = true
		}
	}
	return doc.GenMarkdownTree(root, outDir)
}
