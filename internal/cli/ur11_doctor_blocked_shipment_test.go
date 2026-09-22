package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
	mcpinternal "github.com/softwaresalt/backlogit/internal/mcp"
)

type ur11DoctorFinding struct {
	Type        string `json:"type"`
	Severity    string `json:"severity"`
	Code        string `json:"code"`
	ArtifactID  string `json:"artifact_id"`
	Description string `json:"description"`
	Message     string `json:"message"`
}

type ur11DoctorReport struct {
	Findings []ur11DoctorFinding `json:"findings"`
}

func TestUR11_DoctorBlockedShipmentIntegrityContract(t *testing.T) {
	tests := []struct {
		name string
		seed func(*testing.T, string)
		want []string
	}{
		{
			name: "active count invariant",
			seed: func(t *testing.T, root string) {
				writeUR11Artifact(t, root, "901-S", "shipment", "active", "")
				writeUR11Artifact(t, root, "902-S", "shipment", "active", "")
			},
			want: []string{
				"multiple_active_shipments:901-S",
				"multiple_active_shipments:902-S",
			},
		},
		{
			name: "malformed blocked shipment",
			seed: func(t *testing.T, root string) {
				writeUR11Artifact(t, root, "903-S", "shipment", "blocked", "blocked_at: not-a-timestamp\n")
				writeUR11Artifact(t, root, "903.001-T", "task", "blocked", "")
			},
			want: []string{"malformed_blocked_shipment:903-S"},
		},
		{
			name: "torn lifecycle intent",
			seed: func(t *testing.T, root string) {
				writeUR11Artifact(t, root, "904-S", "shipment", "queued", "")
				writeUR11LifecycleEvents(t, root, "904-S")
			},
			want: []string{"torn_shipment_lifecycle_intent:904-S"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, ws := newUR11DoctorFixture(t)
			tt.seed(t, root)

			report, err := core.Doctor(context.Background(), ws, &core.DoctorOptions{
				CheckOrphans:    false,
				CheckDuplicates: false,
			})
			require.NoError(t, err)
			corePayload, err := json.Marshal(report)
			require.NoError(t, err)
			coreView := decodeUR11DoctorReport(t, corePayload)
			assertUR11ErrorFindings(t, coreView, tt.want)

			cliPayload, cliErr := runUR11DoctorCLI(t, root)
			var exitErr *ExitError
			require.True(t, errors.As(cliErr, &exitErr), "error findings must produce an ExitError")
			assert.Equal(t, 1, exitErr.ExitCode(), "doctor error findings must use exit code 1")
			cliView := decodeUR11DoctorReport(t, cliPayload)
			assertUR11ErrorFindings(t, cliView, tt.want)

			mcpView := runUR11DoctorMCP(t, ws)
			assertUR11ErrorFindings(t, mcpView, tt.want)
			assert.Equal(t, cliView.Findings, mcpView.Findings,
				"CLI and MCP must return identical structured doctor findings")
		})
	}
}

func newUR11DoctorFixture(t *testing.T) (string, *core.Workspace) {
	t.Helper()

	root := t.TempDir()
	storageRoot := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(filepath.Join(storageRoot, "queue"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(storageRoot, "logs"), 0o755))
	require.NoError(t, config.WriteDefaults(storageRoot))

	ws, err := core.NewWorkspace(context.Background(), root)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, ws.Close()) })
	return root, ws
}

func writeUR11Artifact(t *testing.T, root, id, artifactType, status, customFields string) {
	t.Helper()

	content := "---\n" +
		"id: " + id + "\n" +
		"title: R11 isolated fixture\n" +
		"artifact_type: " + artifactType + "\n" +
		"status: " + status + "\n"
	if customFields != "" {
		content += "custom_fields:\n"
		for _, line := range bytes.Split([]byte(customFields), []byte("\n")) {
			if len(line) > 0 {
				content += "    " + string(line) + "\n"
			}
		}
	}
	content += "---\n"
	path := filepath.Join(root, ".backlogit", "queue", id+".md")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func writeUR11LifecycleEvents(t *testing.T, root, shipmentID string) {
	t.Helper()

	events := []map[string]any{
		{
			"timestamp":  "2026-09-22T00:00:00Z",
			"actor":      "r11-test",
			"item_id":    shipmentID,
			"event_type": "shipment_lifecycle",
			"delta": map[string]any{
				"correlation_id": "torn-correlation",
				"operation":      "block",
				"phase":          "intent",
			},
		},
		{
			"timestamp":  "2026-09-22T00:01:00Z",
			"actor":      "r11-test",
			"item_id":    shipmentID,
			"event_type": "shipment_lifecycle",
			"delta": map[string]any{
				"correlation_id": "closed-correlation",
				"operation":      "unblock",
				"phase":          "intent",
			},
		},
		{
			"timestamp":  "2026-09-22T00:02:00Z",
			"actor":      "r11-test",
			"item_id":    shipmentID,
			"event_type": "shipment_lifecycle",
			"delta": map[string]any{
				"correlation_id": "closed-correlation",
				"operation":      "unblock",
				"phase":          "committed",
			},
		},
	}

	var content bytes.Buffer
	encoder := json.NewEncoder(&content)
	for _, event := range events {
		require.NoError(t, encoder.Encode(event))
	}
	path := filepath.Join(root, ".backlogit", "logs", shipmentID+".jsonl")
	require.NoError(t, os.WriteFile(path, content.Bytes(), 0o644))
}

func runUR11DoctorCLI(t *testing.T, root string) ([]byte, error) {
	t.Helper()

	output := new(bytes.Buffer)
	command := NewRootCommand()
	command.SetOut(output)
	command.SetErr(new(bytes.Buffer))
	command.SetArgs([]string{
		"doctor",
		"--cwd", root,
		"--format", "json",
		"--check-orphans=false",
		"--check-duplicates=false",
	})
	err := command.Execute()
	return output.Bytes(), err
}

func runUR11DoctorMCP(t *testing.T, ws *core.Workspace) ur11DoctorReport {
	t.Helper()

	server := mcpinternal.NewServer(ws)
	request := mcplib.CallToolRequest{}
	request.Params.Arguments = map[string]any{
		"check_orphans":    false,
		"check_duplicates": false,
	}
	result, err := server.InvokeTool(context.Background(), "backlogit_doctor", request)
	require.NoError(t, err)
	require.False(t, result.IsError, "doctor findings are a successful structured MCP result")
	require.NotEmpty(t, result.Content)
	text, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok)
	return decodeUR11DoctorReport(t, []byte(text.Text))
}

func decodeUR11DoctorReport(t *testing.T, payload []byte) ur11DoctorReport {
	t.Helper()

	var report ur11DoctorReport
	require.NoError(t, json.Unmarshal(payload, &report))
	sort.Slice(report.Findings, func(i, j int) bool {
		if report.Findings[i].Code == report.Findings[j].Code {
			return report.Findings[i].ArtifactID < report.Findings[j].ArtifactID
		}
		return report.Findings[i].Code < report.Findings[j].Code
	})
	return report
}

func assertUR11ErrorFindings(t *testing.T, report ur11DoctorReport, want []string) {
	t.Helper()

	got := make([]string, 0, len(report.Findings))
	for _, finding := range report.Findings {
		require.Equal(t, "error", finding.Severity)
		require.Equal(t, finding.Type, finding.Code, "code must preserve the existing finding type")
		require.Equal(t, finding.Description, finding.Message, "message must preserve the existing description")
		got = append(got, finding.Code+":"+finding.ArtifactID)
	}
	assert.Equal(t, want, got)
}
