package parser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/softwaresalt/backlogit/internal/models"
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
	Title        string            `json:"title"`
	Body         string            `json:"body"`
	Status       string            `json:"status"`
	Metadata     map[string]string `json:"metadata"`
	Fields       map[string]any    `json:"fields,omitempty"`
	SourceType   DocumentClass     `json:"source_type"`
	ParentRef    string            `json:"parent_ref"`
	SourcePath   string            `json:"source_path"`
	SourceID     string            `json:"source_id,omitempty"`
	ArtifactType string            `json:"artifact_type,omitempty"`
	Depth        int               `json:"depth"`
	Priority     string            `json:"priority"`
	AssignedTo   string            `json:"assigned_to"`
	DateRef      string            `json:"date_ref"`
	Tags         []string          `json:"tags"`
	Dependencies []string          `json:"dependencies,omitempty"`
	References   []string          `json:"references,omitempty"`
	SprintGroup  string            `json:"sprint_group"`
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
	return &Classifier{}
}

// Classify analyzes a single markdown file and returns its detected class with confidence score.
//
// It uses path hints, YAML frontmatter fields, content keywords, and checklist density
// to determine the document class. The highest-confidence signal wins.
func (c *Classifier) Classify(path string) (ClassificationResult, error) {
	result := ClassificationResult{Path: path, Class: ClassUnknown}

	data, err := os.ReadFile(path)
	if err != nil {
		return result, fmt.Errorf("read file: %w", err)
	}
	content := string(data)
	lower := strings.ToLower(content)

	type candidate struct {
		class      DocumentClass
		confidence float64
	}
	var candidates []candidate

	// Path-based hints.
	norm := filepath.ToSlash(path)
	switch {
	case strings.Contains(norm, "/decisions/") || strings.Contains(norm, "/adrs/"):
		candidates = append(candidates, candidate{ClassDecision, 0.6})
	case strings.Contains(norm, "/specs/") || strings.Contains(norm, "/requirements/"):
		candidates = append(candidates, candidate{ClassSpec, 0.6})
	case strings.Contains(norm, "/plans/"):
		candidates = append(candidates, candidate{ClassPlan, 0.6})
	}

	// Frontmatter field hints.
	if strings.Contains(lower, "\ntype: task") || strings.Contains(lower, "\ntype: bug") ||
		strings.HasPrefix(lower, "type: task") {
		candidates = append(candidates, candidate{ClassWorkItem, 0.7})
	}

	// Content keyword hints.
	if strings.Contains(lower, "acceptance criteria") || strings.Contains(lower, "user story") {
		candidates = append(candidates, candidate{ClassSpec, 0.7})
	}
	if strings.Contains(lower, "## status") &&
		(strings.Contains(lower, "## context") || strings.Contains(lower, "## decision")) {
		candidates = append(candidates, candidate{ClassDecision, 0.7})
	}
	if strings.Contains(lower, "implementation units") ||
		(strings.Contains(lower, "## timeline") && strings.Contains(lower, "plan")) {
		candidates = append(candidates, candidate{ClassPlan, 0.7})
	}

	// Checklist density hint.
	checklistCount := 0
	for _, line := range strings.Split(content, "\n") {
		if checklistRe.MatchString(line) {
			checklistCount++
		}
	}
	if checklistCount > 0 {
		candidates = append(candidates, candidate{ClassWorkItem, 0.5 + float64(checklistCount)*0.05})
	}

	// Select highest-confidence candidate.
	best := candidate{ClassUnknown, 0.0}
	for _, cand := range candidates {
		if cand.confidence > best.confidence {
			best = cand
		}
	}

	result.Class = best.class
	result.Confidence = best.confidence
	return result, nil
}

// ClassifyDir scans a directory and classifies all markdown files found.
// Non-markdown files are skipped.
func (c *Classifier) ClassifyDir(dirPath string) ([]ClassificationResult, error) {
	var results []ClassificationResult
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		r, classErr := c.Classify(path)
		if classErr != nil {
			return classErr
		}
		results = append(results, r)
		return nil
	})
	return results, err
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

var errSkipStructuredFile = errors.New("skip structured backlog file")

var structuredDirArtifactTypes = map[string]string{
	"tasks":      "task",
	"drafts":     "task",
	"completed":  "task",
	"archive":    "task",
	"milestones": "feature",
}

// Name returns the adapter identifier.
func (a *BacklogMdAdapter) Name() string {
	return "backlog-md"
}

// Detect reports whether the path contains a Backlog.md-compatible file.
//
// Worker: Check if path points to a .md file that contains checklist items
// (regex match for `- [ ]` or `- [x]` patterns) indicating a legacy backlog format.
func (a *BacklogMdAdapter) Detect(path string) bool {
	if _, ok := resolveStructuredBacklogRoot(path); ok {
		return true
	}

	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		if item, parseErr := parseStructuredBacklogFile(path, "", "task"); parseErr == nil && item != nil {
			return true
		}
	}

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
	if structuredRoot, ok := resolveStructuredBacklogRoot(path); ok {
		return parseStructuredBacklogWorkspace(structuredRoot)
	}

	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		if item, parseErr := parseStructuredBacklogFile(path, "", "task"); parseErr == nil && item != nil {
			return []MigrationItem{*item}, nil
		}
	}

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
			Title:        li.Title,
			Status:       li.Status,
			ParentRef:    li.ParentTitle,
			Depth:        li.Depth,
			Body:         li.Description,
			ArtifactType: "task",
			SourceType:   ClassWorkItem,
			SourcePath:   path,
		}
	}
	return items, nil
}

func resolveStructuredBacklogRoot(path string) (string, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return "", false
	}

	if info.IsDir() {
		if isStructuredBacklogRoot(path) {
			return path, true
		}

		for _, candidate := range []string{"backlog", ".backlog"} {
			child := filepath.Join(path, candidate)
			if isStructuredBacklogRoot(child) {
				return child, true
			}
		}
	}

	return "", false
}

func isStructuredBacklogRoot(path string) bool {
	if _, err := os.Stat(filepath.Join(path, "config.yml")); err == nil {
		for dir := range structuredDirArtifactTypes {
			if _, dirErr := os.Stat(filepath.Join(path, dir)); dirErr == nil {
				return true
			}
		}
	}
	return false
}

func parseStructuredBacklogWorkspace(root string) ([]MigrationItem, error) {
	var items []MigrationItem

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		parts := strings.Split(rel, "/")
		if len(parts) == 0 {
			return nil
		}

		defaultType, ok := structuredDirArtifactTypes[parts[0]]
		if !ok {
			return nil
		}

		item, err := parseStructuredBacklogFile(path, parts[0], defaultType)
		if err != nil {
			if errors.Is(err, errSkipStructuredFile) {
				return nil
			}
			return err
		}
		items = append(items, *item)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Depth == items[j].Depth {
			return items[i].SourceID < items[j].SourceID
		}
		return items[i].Depth < items[j].Depth
	})

	return items, nil
}

func parseStructuredBacklogFile(path, sourceDir, defaultType string) (*MigrationItem, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read structured backlog file: %w", err)
	}

	fm, body, err := models.ParseFrontmatter(string(data))
	if err != nil {
		return nil, fmt.Errorf("parse structured frontmatter: %w", err)
	}
	if fm == nil {
		return nil, errSkipStructuredFile
	}

	title, _ := fm["title"].(string)
	sourceID, _ := fm["id"].(string)
	if strings.TrimSpace(title) == "" || strings.TrimSpace(sourceID) == "" {
		return nil, errSkipStructuredFile
	}

	status, _ := fm["status"].(string)
	priority, _ := fm["priority"].(string)
	milestone, _ := fm["milestone"].(string)
	reporter := firstStringValue(fm["reporter"])
	assigned := firstStringValue(fm["assignee"])
	labels := stringSliceValue(fm["labels"])
	dependencies := stringSliceValue(fm["dependencies"])
	references := uniqueStrings(append(stringSliceValue(fm["references"]), stringSliceValue(fm["documentation"])...))

	artifactType := resolveStructuredArtifactType(defaultType, fm, sourceID)
	parentRef := deriveParentRefFromSourceID(sourceID)
	depth := 1
	if parentRef != "" {
		depth = strings.Count(sourceID, ".") + 1
	}

	fields := make(map[string]any)
	fields["backlog_md_id"] = sourceID
	fields["backlog_md_source_path"] = filepath.ToSlash(path)
	if reporter != "" {
		fields["backlog_md_reporter"] = reporter
	}
	if created, ok := fm["created_date"]; ok {
		fields["backlog_md_created_date"] = fmt.Sprintf("%v", created)
	}
	if updated, ok := fm["updated_date"]; ok {
		fields["backlog_md_updated_date"] = fmt.Sprintf("%v", updated)
	}
	if completed, ok := fm["completed_date"]; ok {
		fields["backlog_md_completed_date"] = fmt.Sprintf("%v", completed)
	}
	if rawType := firstStringValue(fm["type"]); rawType != "" {
		fields["backlog_md_type"] = rawType
	}
	if rawTaskType := firstStringValue(fm["task_type"]); rawTaskType != "" {
		fields["backlog_md_task_type"] = rawTaskType
	}

	for key, value := range fm {
		switch key {
		case "id", "title", "status", "assignee", "reporter", "created_date", "updated_date", "completed_date", "labels", "dependencies", "priority", "milestone", "references", "documentation", "type", "task_type":
			continue
		default:
			fields["backlog_md_"+key] = value
		}
	}

	return &MigrationItem{
		Title:        title,
		Body:         strings.TrimSpace(body),
		Status:       mapStructuredStatus(status, sourceDir),
		Fields:       fields,
		SourceType:   ClassWorkItem,
		ParentRef:    parentRef,
		SourcePath:   path,
		SourceID:     sourceID,
		ArtifactType: artifactType,
		Depth:        depth,
		Priority:     priority,
		AssignedTo:   assigned,
		Tags:         labels,
		Dependencies: dependencies,
		References:   references,
		SprintGroup:  milestone,
		Metadata: map[string]string{
			"source_dir": sourceDir,
		},
	}, nil
}

func mapStructuredStatus(status, sourceDir string) string {
	if sourceDir == "archive" {
		return "archived"
	}
	if sourceDir == "completed" {
		return "done"
	}

	normalized := strings.ToLower(strings.TrimSpace(status))
	normalized = strings.ReplaceAll(normalized, "-", "")
	normalized = strings.ReplaceAll(normalized, "_", "")
	normalized = strings.ReplaceAll(normalized, " ", "")

	switch normalized {
	case "", "todo", "open", "draft":
		return "queued"
	case "inprogress", "active", "doing":
		return "active"
	case "blocked":
		return "blocked"
	case "review", "inreview":
		return "review"
	case "done", "completed", "closed":
		return "done"
	case "accepted":
		return "accepted"
	case "rejected":
		return "rejected"
	case "archived":
		return "archived"
	default:
		if sourceDir == "drafts" {
			return "queued"
		}
		return "queued"
	}
}

func resolveStructuredArtifactType(defaultType string, fm map[string]any, sourceID string) string {
	rawType := strings.ToLower(strings.TrimSpace(firstStringValue(fm["task_type"])))
	if rawType == "" {
		rawType = strings.ToLower(strings.TrimSpace(firstStringValue(fm["type"])))
	}

	hierarchyType := defaultType
	dotCount := strings.Count(sourceID, ".")
	switch {
	case dotCount == 0:
		hierarchyType = "feature"
	case dotCount == 1:
		hierarchyType = "task"
	case dotCount >= 2:
		hierarchyType = "subtask"
	}

	switch rawType {
	case "feature", "enhancement", "epic", "milestone":
		return "feature"
	case "subtask", "sub-task", "sub_task":
		return "subtask"
	case "task", "chore", "spike", "story", "userstory", "user_story", "bug", "":
		return hierarchyType
	default:
		return hierarchyType
	}
}

func deriveParentRefFromSourceID(sourceID string) string {
	lastDot := strings.LastIndex(sourceID, ".")
	if lastDot == -1 {
		return ""
	}
	return sourceID[:lastDot]
}

func stringSliceValue(v any) []string {
	if v == nil {
		return nil
	}
	if s, ok := v.([]string); ok {
		return s
	}
	if s, ok := v.([]any); ok {
		result := make([]string, 0, len(s))
		for _, item := range s {
			str := strings.TrimSpace(fmt.Sprintf("%v", item))
			if str != "" {
				result = append(result, str)
			}
		}
		return result
	}
	if s, ok := v.(string); ok {
		s = strings.TrimSpace(s)
		if s == "" {
			return nil
		}
		return []string{s}
	}
	return nil
}

func firstStringValue(v any) string {
	values := stringSliceValue(v)
	if len(values) > 0 {
		return values[0]
	}
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
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
// It resolves an adapter (by name or auto-detect), parses the source file,
// and returns a MigrationReport with counts and any errors encountered.
// When DryRun is true, items are counted but no files are written.
func MigrateWithOptions(ctx context.Context, sourcePath string, opts MigrateOptions) (*MigrationReport, error) {
	var adapter MigrationAdapter
	if opts.Adapter != "" {
		a, err := GetAdapter(opts.Adapter)
		if err != nil {
			return nil, fmt.Errorf("adapter %q: %w", opts.Adapter, err)
		}
		adapter = a
	} else {
		a, err := DetectAdapter(sourcePath)
		if err != nil {
			adapter = &BacklogMdAdapter{}
		} else {
			adapter = a
		}
	}

	items, err := adapter.Parse(ctx, sourcePath)
	if err != nil {
		return nil, fmt.Errorf("parse %q: %w", sourcePath, err)
	}

	return &MigrationReport{
		Items:         items,
		ItemsMigrated: len(items),
	}, nil
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
// When format is "json", the report is marshalled to indented JSON.
// For all other values (including ""), a human-readable text summary is produced.
func FormatReport(report *MigrationReport, format string) (string, error) {
	if format == "json" {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return "", fmt.Errorf("marshal report: %w", err)
		}
		return string(data), nil
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "Migrated: %d, Skipped: %d, Failed: %d\n",
		report.ItemsMigrated, report.ItemsSkipped, report.ItemsFailed)
	if len(report.Items) > 0 {
		sb.WriteString("Items:\n")
		for _, item := range report.Items {
			label := item.Title
			if item.SourceID != "" {
				label = item.SourceID + " - " + item.Title
			}
			if item.ArtifactType != "" {
				fmt.Fprintf(&sb, "  - %s [%s -> %s]\n", label, item.Status, item.ArtifactType)
			} else {
				fmt.Fprintf(&sb, "  - %s [%s]\n", label, item.Status)
			}
		}
	}
	if len(report.Errors) > 0 {
		sb.WriteString("Errors:\n")
		for _, e := range report.Errors {
			fmt.Fprintf(&sb, "  %s\n", e)
		}
	}
	return sb.String(), nil
}
