package mcp

import (
	"context"
	"encoding/json"
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
			mcplib.WithDescription("Validate in-scope documentation frontmatter against the docline base schema. "+
				"Returns a success envelope {valid, violation_count, findings} even when violations exist. A "+
				"per-file frontmatter decode failure (malformed YAML) is one such violation: it is reported as a "+
				"finding with rule decode_error in this same successful result, and the scan continues over the "+
				"rest of the corpus, rather than the call itself failing."),
			mcplib.WithString("path", mcplib.Description("Optional repo-relative sub-path to limit the scan")),
			mcplib.WithString("profile", mcplib.Description("Validation profile: authoring (default) or ingestion")),
		),
		s.handleDocsLint,
	)
	s.addTool(
		mcplib.NewTool("backlogit_docs_migrate",
			mcplib.WithDescription("Plan (default) or apply an idempotent, body-preserving frontmatter migration. apply=true is gated server-side and requires an explicit scoped path."),
			mcplib.WithString("path", mcplib.Description("Repo-relative sub-path (required when apply=true)")),
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
	prof, err := docline.ParseProfile(profile)
	if err != nil {
		return ValidationFailed(err.Error()), nil
	}

	findings, err := docline.LintTree(docline.Options{
		Root:    s.RootPath,
		Path:    path,
		Profile: prof,
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
		if err := docline.ValidateApplyPath(s.RootPath, path); err != nil {
			return ValidationFailed(err.Error()), nil
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
			if errors.Is(err, docline.ErrPlanHasFindings) {
				// Return a structured, non-InternalError result with the findings.
				// The distinct error-type "plan_has_findings" lets agents
				// disambiguate a corpus-content rejection (findings in plan)
				// from a --path validation failure.
				return planHasFindingsResult(plan), nil
			}
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

// planHasFindingsResult builds a structured MCP error result for a plan that
// carries per-file findings. The error_type is "plan_has_findings" (distinct
// from "validation_failed" so agents can disambiguate a corpus-content rejection
// from a --path/validation rejection) and the response carries a discrete
// top-level findings array via a dedicated struct — not flattened into message.
func planHasFindingsResult(plan docline.MigrationPlan) *mcplib.CallToolResult {
	type planHasFindingsResponse struct {
		Error    string                  `json:"error"`
		Message  string                  `json:"message"`
		Findings []docline.FindingReport `json:"findings"`
	}
	findings := make([]docline.FindingReport, 0, len(plan.Findings))
	for _, f := range plan.Findings {
		findings = append(findings, docline.FindingReport{
			File:     f.File,
			Field:    f.Field,
			Rule:     f.Rule,
			Severity: string(f.Severity),
			Fix:      f.Fix,
		})
	}
	resp := planHasFindingsResponse{
		Error: "plan_has_findings",
		Message: fmt.Sprintf(
			"migration plan carries %d per-file finding(s); apply refused to preserve corpus all-or-nothing guarantee",
			len(plan.Findings),
		),
		Findings: findings,
	}
	data, err := json.Marshal(resp)
	if err != nil {
		return InternalError(fmt.Sprintf("marshal plan_has_findings response: %v", err))
	}
	return mcplib.NewToolResultError(string(data))
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
