package parser

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// DocumentClass represents the type of a markdown document detected by classification.
type DocumentClass string

const (
	// ClassSpec indicates a requirements or specification document.
	ClassSpec DocumentClass = "spec"
	// ClassPlan indicates an implementation or execution plan.
	ClassPlan DocumentClass = "plan"
	// ClassWorkItem indicates a task, bug, or user story.
	ClassWorkItem DocumentClass = "work_item"
	// ClassDecision indicates an architecture decision record.
	ClassDecision DocumentClass = "decision"
	// ClassNote indicates a general note or documentation.
	ClassNote DocumentClass = "note"
	// ClassUnknown indicates the document type could not be determined.
	ClassUnknown DocumentClass = "unknown"
)

// MigrationItem is a generalized intermediate representation produced by adapters.
type MigrationItem struct {
	Title       string            `json:"title"`
	Body        string            `json:"body"`
	Status      string            `json:"status"`
	Metadata    map[string]string `json:"metadata"`
	SourceType  DocumentClass     `json:"source_type"`
	ParentRef   string            `json:"parent_ref"`
	SourcePath  string            `json:"source_path"`
	Depth       int               `json:"depth"`
	Priority    string            `json:"priority"`
	AssignedTo  string            `json:"assigned_to"`
	DateRef     string            `json:"date_ref"`
	Tags        []string          `json:"tags"`
	SprintGroup string            `json:"sprint_group"`
}

// MigrationAdapter defines the contract for pluggable migration sources.
type MigrationAdapter interface {
	// Name returns the adapter's unique identifier.
	Name() string
	// Detect reports whether the given path contains content this adapter can migrate.
	Detect(path string) bool
	// Parse reads the source at path and returns extracted migration items.
	Parse(ctx context.Context, path string) ([]MigrationItem, error)
}

// ClassificationResult holds the output of document classification.
type ClassificationResult struct {
	Class      DocumentClass `json:"class"`
	Confidence float64       `json:"confidence"`
	Path       string        `json:"path"`
}

// Classifier analyzes markdown documents to detect their type for migration routing.
type Classifier struct{}

// NewClassifier creates a new document classifier.
func NewClassifier() *Classifier {
	panic("not implemented: Worker: Create a Classifier struct. No configuration needed initially — the classifier uses built-in heuristic rules.")
}

// Classify analyzes a single markdown file and returns its detected class with confidence score.
//
// Worker: Implement heuristic detection using these signals:
//   - YAML frontmatter field analysis (type/status/priority → work_item)
//   - Heading structure (ADR-style numbered headings → decision)
//   - Checklist density (high ratio → work_item or plan)
//   - Keyword frequency (requirement, acceptance criteria → spec)
//   - Directory path hints (plans/, decisions/ → corresponding class)
func (c *Classifier) Classify(path string) (ClassificationResult, error) {
	panic("not implemented: Worker: Implement heuristic classification. Read the file, analyze frontmatter, heading structure, checklist density, keyword frequency, and directory path hints. Return ClassificationResult with class and confidence 0.0-1.0.")
}

// ClassifyDir scans a directory and classifies all markdown files found.
//
// Worker: Walk the directory tree, classify each .md file, and return results.
// Skip non-markdown files and binary files. Use filepath.WalkDir for efficient traversal.
func (c *Classifier) ClassifyDir(dirPath string) ([]ClassificationResult, error) {
	panic("not implemented: Worker: Walk directory tree with filepath.WalkDir, classify each .md file using c.Classify, collect results. Skip non-.md files.")
}

// adapterRegistry manages registered migration adapters.
var (
	registryMu sync.RWMutex
	adapters   = make(map[string]MigrationAdapter)
)

// Enhanced-parser extraction patterns.
var (
	priorityBracketRe = regexp.MustCompile(`^\[P[0-4]\]\s+`)
	priorityBangRe    = regexp.MustCompile(`^!(\w+)\s+`)
	assigneeRe        = regexp.MustCompile(`^@\w+\s+`)
	dateRe            = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}\s+`)
	tagRe             = regexp.MustCompile(`^\[[^\]]*,[^\]]*\]\s+`)
)

// RegisterAdapter adds a migration adapter to the global registry.
//
// Worker: Store the adapter in the adapters map under adapter.Name().
// Return an error if an adapter with the same name is already registered.
func RegisterAdapter(adapter MigrationAdapter) error {
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, exists := adapters[adapter.Name()]; exists {
		return fmt.Errorf("adapter %q already registered", adapter.Name())
	}
	adapters[adapter.Name()] = adapter
	return nil
}

// GetAdapter retrieves a registered adapter by name.
//
// Worker: Look up the adapter in the adapters map.
// Return (adapter, nil) if found, (nil, error) if not found.
func GetAdapter(name string) (MigrationAdapter, error) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	if a, ok := adapters[name]; ok {
		return a, nil
	}
	return nil, fmt.Errorf("adapter %q not found", name)
}

// ListAdapters returns the names of all registered adapters.
//
// Worker: Return a sorted slice of adapter names from the registry.
func ListAdapters() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	names := make([]string, 0, len(adapters))
	for name := range adapters {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// DetectAdapter finds the first registered adapter that can handle the given path.
//
// Worker: Iterate through all registered adapters, call Detect(path) on each.
// Return the first adapter that returns true. If none match, return an error.
func DetectAdapter(path string) (MigrationAdapter, error) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	for _, a := range adapters {
		if a.Detect(path) {
			return a, nil
		}
	}
	return nil, fmt.Errorf("no adapter detected for path %q", path)
}

// ResetRegistry clears all registered adapters. Used for testing.
func ResetRegistry() {
	registryMu.Lock()
	defer registryMu.Unlock()
	adapters = make(map[string]MigrationAdapter)
}

// BacklogMdAdapter implements MigrationAdapter for Backlog.md files.
type BacklogMdAdapter struct{}

// Name returns the adapter identifier.
func (a *BacklogMdAdapter) Name() string {
	return "backlog-md"
}

// Detect reports whether the path contains a Backlog.md-compatible file.
//
// Worker: Check if path points to a .md file that contains checklist items
// (regex match for `- [ ]` or `- [x]` patterns) indicating a legacy backlog format.
func (a *BacklogMdAdapter) Detect(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if checklistRe.MatchString(line) {
			return true
		}
	}
	return false
}

// Parse extracts migration items from a Backlog.md file.
//
// Worker: Use the enhanced ParseLegacy to parse the file, then convert
// each LegacyItem to a MigrationItem with appropriate field mapping.
func (a *BacklogMdAdapter) Parse(ctx context.Context, path string) ([]MigrationItem, error) {
	_ = ctx
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	legacy, err := ParseLegacy(string(data))
	if err != nil {
		return nil, fmt.Errorf("parse legacy: %w", err)
	}
	items := make([]MigrationItem, len(legacy))
	for i, li := range legacy {
		items[i] = MigrationItem{
			Title:      li.Title,
			Status:     li.Status,
			ParentRef:  li.ParentTitle,
			Depth:      li.Depth,
			Body:       li.Description,
			SourceType: ClassWorkItem,
			SourcePath: path,
		}
	}
	return items, nil
}

// Ensure BacklogMdAdapter implements MigrationAdapter at compile time.
var _ MigrationAdapter = (*BacklogMdAdapter)(nil)

// init registers the built-in BacklogMdAdapter.
func init() {
	// Registration happens at init time so the adapter is available
	// when the migrate command runs. ResetRegistry() in tests clears this.
	registryMu.Lock()
	defer registryMu.Unlock()
	adapters["backlog-md"] = &BacklogMdAdapter{}
}

// ParseLegacyEnhanced extends ParseLegacy with broader format coverage.
//
// It parses checklist items and enriches each LegacyItem with:
//   - Depth set from the heading level (H1=1 … H4=4) of the containing heading
//   - ParentTitle from the immediate heading title
//   - Title with priority markers ([P0]-[P4], !high), @mentions, dates (YYYY-MM-DD)
//     and tag annotations ([tag, tag]) stripped
//   - Description populated from paragraph text immediately following the item
//
// Status mapping and backwards compatibility are identical to ParseLegacy.
func ParseLegacyEnhanced(content string) ([]LegacyItem, error) {
	var items []LegacyItem

	var currentDepth int
	var currentSection string
	lastIdx := -1
	var descLines []string
	inDesc := false

	flushDesc := func() {
		if lastIdx >= 0 && len(descLines) > 0 {
			items[lastIdx].Description = strings.TrimSpace(strings.Join(descLines, "\n"))
			descLines = nil
		}
		inDesc = false
	}

	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "#") {
			flushDesc()
			i := 0
			for i < len(line) && line[i] == '#' {
				i++
			}
			currentDepth = i
			currentSection = strings.TrimSpace(line[i:])
			continue
		}

		if m := checklistRe.FindStringSubmatch(line); m != nil {
			flushDesc()
			checked := strings.ToLower(m[1]) == "x"
			rawTitle := strings.TrimSpace(m[2])

			status := "queued"
			if checked {
				status = "done"
			} else {
				switch strings.ToLower(currentSection) {
				case "in progress", "in_progress", "active":
					status = "active"
				case "blocked":
					status = "blocked"
				}
			}

			// Strip metadata prefixes from title.
			title := priorityBracketRe.ReplaceAllString(rawTitle, "")
			title = priorityBangRe.ReplaceAllString(title, "")
			title = assigneeRe.ReplaceAllString(title, "")
			title = dateRe.ReplaceAllString(title, "")
			title = tagRe.ReplaceAllString(title, "")
			title = strings.TrimSpace(title)

			items = append(items, LegacyItem{
				Title:       title,
				Status:      status,
				ParentTitle: currentSection,
				Depth:       currentDepth,
			})
			lastIdx = len(items) - 1
			inDesc = true
			continue
		}

		if inDesc {
			if trimmed := strings.TrimSpace(line); trimmed != "" {
				descLines = append(descLines, trimmed)
			}
		}
	}

	flushDesc()
	return items, nil
}

// MigrationReport holds the results of a migration operation.
type MigrationReport struct {
	ItemsMigrated int
	ItemsSkipped  int
	ItemsFailed   int
	Errors        []string
	Items         []MigrationItem
}

// MigrateWithOptions performs migration with configurable behavior.
//
// Worker: Implement migration with these options:
//   - dryRun: if true, parse and validate but do not write any files
//   - validate: if true, run validation checks and report issues
//   - adapter: use the specified adapter (or auto-detect if empty)
//   - configPath: path to migration.yaml configuration
//
// Return a MigrationReport summarizing the operation.
func MigrateWithOptions(ctx context.Context, sourcePath string, opts MigrateOptions) (*MigrationReport, error) {
	_ = ctx
	_ = sourcePath
	_ = opts
	panic("not implemented: Worker: Resolve adapter (auto-detect or use opts.Adapter). Parse source. If dryRun, return report without writing. If validate, check items against config constraints. Otherwise, process items through the workspace creation pipeline. Collect errors without aborting.")
}

// MigrateOptions configures migration behavior.
type MigrateOptions struct {
	DryRun     bool
	Validate   bool
	Adapter    string
	ConfigPath string
	Format     string
}

// FormatReport formats a MigrationReport as text or JSON.
//
// Worker: If format is "json", marshal report to JSON.
// If format is "text" (default), produce a human-readable summary with counts
// and any error details.
func FormatReport(report *MigrationReport, format string) (string, error) {
	_ = report
	_ = format
	panic("not implemented: Worker: Format the MigrationReport. For 'json', use json.MarshalIndent. For 'text', produce a summary like 'Migrated: N, Skipped: N, Failed: N' followed by error details.")
}
