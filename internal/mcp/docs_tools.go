package mcp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/softwaresalt/backlogit/internal/docline"
)

// docsApplyAllowEnv gates server-side apply for backlogit_docs_migrate. Bulk
// document migration is a high-blast-radius operation, so the MCP surface only
// performs writes when this environment variable is explicitly enabled.
const docsApplyAllowEnv = "BACKLOGIT_DOCS_ALLOW_APPLY"

// registerDocsTools registers the docline parity tools as fixed, unconditional
// MCP tools (discoverable via ListTools / get_metadata_catalog).
func (s *Server) registerDocsTools() {
	s.addTool(
		mcplib.NewTool("backlogit_docs_lint",
			mcplib.WithDescription("Validate in-scope documentation frontmatter against the docline base schema. Returns a success envelope {valid, violation_count, findings} even when violations exist."),
			mcplib.WithString("path", mcplib.Description("Optional repo-relative sub-path to limit the scan")),
			mcplib.WithString("profile", mcplib.Description("Validation profile: authoring (default) or ingestion")),
		),
		s.handleDocsLint,
	)
	s.addTool(
		mcplib.NewTool("backlogit_docs_migrate",
			mcplib.WithDescription("Plan (default) or apply an idempotent, body-preserving frontmatter migration. apply=true is gated server-side and requires an explicit scoped path."),
			mcplib.WithString("path", mcplib.Description("Repo-relative sub-path (required when apply=true)")),
			mcplib.WithString("profile", mcplib.Description("Validation profile (reserved; planning uses the authoring contract)")),
			mcplib.WithBoolean("apply", mcplib.Description("Write changes (default false). Gated by the BACKLOGIT_DOCS_ALLOW_APPLY environment flag.")),
		),
		s.handleDocsMigrate,
	)
	s.addTool(
		mcplib.NewTool("backlogit_docs_scope",
			mcplib.WithDescription("Return the active docline scope globs, taxonomy, path map, and validation profiles."),
		),
		s.handleDocsScope,
	)
}

func (s *Server) handleDocsLint(_ context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	path, _ := request.Params.Arguments["path"].(string)
	profile, _ := request.Params.Arguments["profile"].(string)
	if profile == "" {
		profile = string(docline.ProfileAuthoring)
	}

	findings, err := docline.LintTree(docline.Options{
		Root:    s.RootPath,
		Path:    path,
		Profile: docline.Profile(profile),
	})
	if err != nil {
		if errors.Is(err, docline.ErrPathEscapesWorkspace) {
			return ValidationFailed(err.Error()), nil
		}
		return InternalError(fmt.Sprintf("docs lint: %v", err)), nil
	}
	return toolResultJSON(docline.NewLintReport(findings))
}

func (s *Server) handleDocsMigrate(_ context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	path, _ := request.Params.Arguments["path"].(string)
	apply, _ := request.Params.Arguments["apply"].(bool)
	opts := docline.Options{Root: s.RootPath, Path: path, Now: time.Now().UTC()}

	if apply {
		// Apply is gated server-side: disabled by default, requires an explicit
		// scoped path, and refuses a whole-tree apply.
		if !docsApplyEnabled() {
			return applyNotPermitted(fmt.Sprintf("docs migrate apply is disabled; set %s=1 to enable server-side writes", docsApplyAllowEnv)), nil
		}
		if path == "" {
			return ValidationFailed("docs migrate apply requires an explicit path (whole-tree apply is refused)"), nil
		}
		plan, err := docline.PlanMigration(opts)
		if err != nil {
			if errors.Is(err, docline.ErrPathEscapesWorkspace) {
				return ValidationFailed(err.Error()), nil
			}
			return InternalError(fmt.Sprintf("docs migrate plan: %v", err)), nil
		}
		res, err := docline.ApplyMigration(plan, opts)
		if err != nil {
			if errors.Is(err, docline.ErrPathEscapesWorkspace) {
				return ValidationFailed(err.Error()), nil
			}
			return InternalError(fmt.Sprintf("docs migrate apply: %v", err)), nil
		}
		return toolResultJSON(docline.NewMigrateReport(plan, &res, false))
	}

	plan, err := docline.PlanMigration(opts)
	if err != nil {
		if errors.Is(err, docline.ErrPathEscapesWorkspace) {
			return ValidationFailed(err.Error()), nil
		}
		return InternalError(fmt.Sprintf("docs migrate plan: %v", err)), nil
	}
	return toolResultJSON(docline.NewMigrateReport(plan, nil, true))
}

func (s *Server) handleDocsScope(_ context.Context, _ mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	return toolResultJSON(docline.Scope())
}

// applyNotPermitted returns a structured MCP error for a gated apply.
func applyNotPermitted(detail string) *mcplib.CallToolResult {
	return makeErrorResult("apply_not_permitted", detail)
}

// docsApplyEnabled reports whether server-side apply is enabled via env flag.
func docsApplyEnabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(docsApplyAllowEnv)))
	return v == "1" || v == "true" || v == "yes"
}
