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
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
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
// into outDir. Previously generated pages are removed before regeneration so
// deleted or renamed commands do not leave stale files behind.
func generateDocs(outDir string) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}
	// Remove stale generated pages before regenerating, preserving README.md
	// and any other hand-written files.
	if err := removeGeneratedPages(outDir); err != nil {
		return fmt.Errorf("remove stale docs: %w", err)
	}

	root := cli.NewRootCommand()
	// Recursively disable the cobra auto-gen tag so output is deterministic.
	disableAutoGenTag(root)

	// Build a filename→description map for the frontmatter prepender.
	descMap := buildDescMap(root, "")

	filePrepender := func(filename string) string {
		base := filepath.Base(filename)
		name := strings.TrimSuffix(base, ".md")
		title := strings.ReplaceAll(name, "_", " ")
		desc := descMap[base]
		if desc == "" {
			desc = title
		}
		return fmt.Sprintf("---\ntitle: %q\ndescription: %q\n---\n\n", title, desc)
	}
	linkHandler := func(name string) string { return name }

	if err := doc.GenMarkdownTreeCustom(root, outDir, filePrepender, linkHandler); err != nil {
		return fmt.Errorf("generate markdown tree: %w", err)
	}

	return addCodeBlockLanguages(outDir)
}

// disableAutoGenTag recursively sets DisableAutoGenTag = true on every command
// node so the cobra disclaimer never appears in generated output.
func disableAutoGenTag(cmd *cobra.Command) {
	cmd.DisableAutoGenTag = true
	for _, sub := range cmd.Commands() {
		disableAutoGenTag(sub)
	}
}

// buildDescMap walks the command tree and returns a map of generated filename
// to the command's Short description string.
func buildDescMap(cmd *cobra.Command, parent string) map[string]string {
	m := make(map[string]string)
	name := cmd.Name()
	if parent != "" {
		name = parent + " " + name
	}
	filename := strings.ReplaceAll(name, " ", "_") + ".md"
	m[filename] = cmd.Short
	for _, sub := range cmd.Commands() {
		for k, v := range buildDescMap(sub, name) {
			m[k] = v
		}
	}
	return m
}

// removeGeneratedPages deletes all .md files in dir except README.md.
func removeGeneratedPages(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read dir: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() || e.Name() == "README.md" {
			continue
		}
		if filepath.Ext(e.Name()) == ".md" {
			if err := os.Remove(filepath.Join(dir, e.Name())); err != nil {
				return fmt.Errorf("remove %s: %w", e.Name(), err)
			}
		}
	}
	return nil
}

// fenceRE matches any code fence delimiter line (labeled or unlabeled).
var fenceRE = regexp.MustCompile("(?m)^```(\\S*)\\s*$")

// addCodeBlockLanguages post-processes every generated .md file (excluding
// README.md) and replaces unlabeled opening code fences (```) with ```text
// for markdownlint compliance.
func addCodeBlockLanguages(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read dir: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".md" || e.Name() == "README.md" {
			continue
		}
		path := filepath.Join(dir, e.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", e.Name(), err)
		}
		updated := labelUnlabeledFences(string(raw))
		if updated == string(raw) {
			continue
		}
		if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", e.Name(), err)
		}
	}
	return nil
}

// labelUnlabeledFences replaces unlabeled opening code fences (```) with
// ```text. It tracks all fence delimiters so labeled opening fences (e.g.
// ```bash) are correctly paired with their closing fences.
func labelUnlabeledFences(content string) string {
	inFence := false
	return fenceRE.ReplaceAllStringFunc(content, func(match string) string {
		if inFence {
			// Closing fence — leave intact and exit fence context.
			inFence = false
			return match
		}
		// Opening fence — enter fence context.
		inFence = true
		if match == "```" {
			return "```text"
		}
		return match
	})
}
