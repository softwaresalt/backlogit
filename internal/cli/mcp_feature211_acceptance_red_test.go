package cli

import (
	"go/ast"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestU2i_Feature211MultiSessionDaemonShimAndDocsAcceptance pins the U3-U7
// deliverables for 211.015-T. U7 keeps this acceptance RED through Waves 3-7;
// green-makers 211.003-T through 211.007-T close it at Wave 8.
func TestU2i_Feature211MultiSessionDaemonShimAndDocsAcceptance(t *testing.T) {
	_, source, _, ok := runtime.Caller(0)
	require.True(t, ok, "locate acceptance source")
	cliDir := filepath.Dir(source)
	docsDir := filepath.Join(cliDir, "..", "..", "docs", "cli-reference")

	t.Run("U3_MultiSessionDeclarations", func(t *testing.T) {
		path := filepath.Join(cliDir, "..", "mcp", "session_mux.go")
		file := parseSourceFile(t, path)
		if file == nil {
			return
		}
		conn := false
		methods := map[string]bool{}
		ast.Inspect(file, func(node ast.Node) bool {
			switch decl := node.(type) {
			case *ast.TypeSpec:
				if decl.Name.Name == "MultiSessionConn" {
					conn = true
				}
			case *ast.FuncDecl:
				methods[decl.Name.Name] = true
			}
			return true
		})
		assert.True(t, conn, "session_mux.go must declare MultiSessionConn")
		for _, name := range []string{"ServeSessionStream", "ActiveSessionCount", "CloseAllSessions"} {
			assert.True(t, methods[name], "session_mux.go must declare %s", name)
		}
	})

	t.Run("U4_PlatformIPCDeclarations", func(t *testing.T) {
		for _, filename := range []string{"mcp_ipc_windows.go", "mcp_ipc_unix.go"} {
			t.Run(filename, func(t *testing.T) {
				file := parseSourceFile(t, filepath.Join(cliDir, filename))
				if file == nil {
					return
				}
				for _, name := range []string{
					"DeriveMCPDaemonEndpoint", "ListenMCPDaemonIPC",
					"DialMCPDaemonIPC", "isVerifiedProcessAlive",
				} {
					assert.NotNil(t, findFunc(file, name), "%s must declare func %s", filename, name)
				}
			})
		}
	})

	t.Run("U5_DaemonAttachmentAndMetadata", func(t *testing.T) {
		file := parseSourceFile(t, filepath.Join(cliDir, "mcp_daemon.go"))
		if file == nil {
			return
		}
		for _, name := range []string{"VerifyMCPDaemonAttachment", "RunMCPDaemon"} {
			assert.NotNil(t, findFunc(file, name), "mcp_daemon.go must declare func %s", name)
		}
		for _, name := range []string{"MCPDaemonInfo", "MCPDaemonStatus"} {
			t.Run(name, func(t *testing.T) {
				fields := map[string]bool{}
				found := false
				ast.Inspect(file, func(node ast.Node) bool {
					decl, ok := node.(*ast.TypeSpec)
					if !ok || decl.Name.Name != name {
						return true
					}
					st, ok := decl.Type.(*ast.StructType)
					if !ok {
						return false
					}
					found = true
					for _, field := range st.Fields.List {
						for _, id := range field.Names {
							fields[id.Name] = true
						}
					}
					return false
				})
				assert.True(t, found, "mcp_daemon.go must declare struct %s", name)
				for _, field := range []string{"IPCTransport", "Endpoint", "ActiveSessions", "ProtocolVersion"} {
					assert.True(t, fields[field], "%s must declare field %s", name, field)
				}
			})
		}
	})

	t.Run("U6_ShimPrewarmAndCommands", func(t *testing.T) {
		file := parseSourceFile(t, filepath.Join(cliDir, "mcp_daemon.go"))
		if file != nil {
			for _, name := range []string{"PrewarmMCPDaemon", "RunMCPSessionShim"} {
				assert.NotNil(t, findFunc(file, name), "mcp_daemon.go must declare func %s", name)
			}
		}
		for _, name := range []string{"daemon", "prewarm"} {
			t.Run(name, func(t *testing.T) {
				cmd, remaining, err := NewRootCommand().Find([]string{"mcp", name})
				if assert.NoError(t, err, "mcp %s must be registered", name) &&
					assert.NotNil(t, cmd, "mcp %s command", name) {
					assert.Equal(t, "backlogit mcp "+name, cmd.CommandPath())
					assert.Empty(t, remaining, "mcp %s must resolve completely", name)
				}
			})
		}
	})

	t.Run("U7_GeneratedReferenceDocsAndLinks", func(t *testing.T) {
		index, err := os.ReadFile(filepath.Join(docsDir, "backlogit_mcp.md"))
		assert.NoError(t, err, "read parent MCP reference")
		for _, name := range []string{"daemon", "prewarm"} {
			t.Run(name, func(t *testing.T) {
				filename := "backlogit_mcp_" + name + ".md"
				content, err := os.ReadFile(filepath.Join(docsDir, filename))
				if assert.NoError(t, err, "U7 must generate %s", filename) {
					assert.Contains(t, strings.Split(string(content), "\n"),
						"## backlogit mcp "+name, "%s must contain its command heading", filename)
				}
				assert.Contains(t, string(index), "]("+filename+")",
					"backlogit_mcp.md must link to %s", filename)
			})
		}
	})
}
