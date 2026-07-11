package mcp

import (
	"context"
	"encoding/json"
	"os"
	"runtime"
	"strings"
	"time"

	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/softwaresalt/backlogit/internal/release"
	"github.com/softwaresalt/backlogit/internal/version"
)

const mcpUpdateCheckTimeout = 1500 * time.Millisecond

// handleGetVersion handles the backlogit_get_version MCP tool.
// Returns version, latest release check, commit, build_date, and go_version fields.
// The tool must function without a workspace (safe for pre-init calls).
func (s *Server) handleGetVersion(ctx context.Context, _ mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	current := version.Resolve()
	data := map[string]any{
		"version":          current,
		"current":          current,
		"latest":           "",
		"update_available": false,
		"update_check":     "unavailable",
		"commit":           version.Commit,
		"build_date":       version.BuildDate,
		"go_version":       runtime.Version(),
	}
	if mcpUpdateCheckSkipped() {
		data["update_check"] = "skipped"
	} else {
		checkCtx, cancel := context.WithTimeout(ctx, mcpUpdateCheckTimeout)
		defer cancel()
		latest, err := release.Client{Token: os.Getenv("GITHUB_TOKEN")}.Latest(checkCtx)
		if err == nil {
			data["latest"] = latest
			if cmp, err := release.CompareVersions(current, latest); err == nil {
				data["update_available"] = cmp < 0
				data["update_check"] = "ok"
			}
		}
	}
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return makeErrorResult("internal", err.Error()), nil
	}
	return mcplib.NewToolResultText(string(b)), nil
}

func mcpUpdateCheckSkipped() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("BACKLOGIT_NO_UPDATE_CHECK"))) {
	case "1", "true", "t", "yes", "y", "on":
		return true
	default:
		return false
	}
}
