package cli

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// U4 / 211.004-T: behavior only; U4a owns the already-landed declarations.
// No test here skips. Windows runs native pipes and POSIX source guards;
// POSIX runs the same endpoint, framing, refusal, and identity scenarios natively.
func TestU4_DeriveMCPDaemonEndpointDeterministicAndWorkspaceIsolated(t *testing.T) {
	t.Run("native_determinism_storage_identity_and_workspace_isolation", func(t *testing.T) {
		base := u4TempDir(t)
		var endpoints [2]string
		calls := 0
		for i := range endpoints {
			workspace := filepath.Join(base, strconv.Itoa(i))
			storage := filepath.Join(workspace, "store")
			runtimeDir := filepath.Join(storage, "runtime")
			require.NoError(t, os.MkdirAll(runtimeDir, 0o700))
			canonical, err := filepath.EvalSymlinks(storage)
			require.NoError(t, err)
			for attempt := 0; attempt < 2; attempt++ {
				transport, endpoint, deriveErr := DeriveMCPDaemonEndpoint(workspace, storage, runtimeDir)
				calls++
				assert.NoError(t, deriveErr, "U4 endpoint derivation must implement D2, not return a declaration stub")
				if runtime.GOOS == "windows" {
					digest := sha256.Sum256([]byte(strings.ToLower(filepath.Clean(canonical))))
					assert.Equal(t, "named_pipe", transport)
					assert.Equal(t, `\\.\pipe\backlogit-mcp-`+hex.EncodeToString(digest[:8]), endpoint)
				} else {
					assert.Equal(t, "unix", transport)
					assert.Equal(t, filepath.Join(runtimeDir, "mcp.sock"), endpoint)
				}
				if attempt == 0 {
					endpoints[i] = endpoint
				} else {
					assert.Equal(t, endpoints[i], endpoint, "repeat derivation must be deterministic")
				}
			}
		}
		assert.NotEmpty(t, endpoints[0])
		assert.NotEqual(t, endpoints[0], endpoints[1], "SC7a: distinct workspaces must not share an endpoint")
		assert.Equal(t, 4, calls, "non-vacuity: both workspaces derived twice")
		t.Logf("U4_GUARD derivation_calls=%d workspace_count=2", calls)
	})
	t.Run("posix_100_byte_boundary_or_source_contract", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			u4POSIXSourceContract(t)
			t.Log("U4_COVERAGE POSIX=source-only; native POSIX boundary/bind execution requires a POSIX host")
			return
		}
		u4POSIXBoundaries(t)
	})
}

func TestU4_ListenAndDialMCPDaemonIPCMultiClient(t *testing.T) {
	t.Run("four_simultaneous_clients_framed_io_and_clean_close", func(t *testing.T) {
		root, runtimeDir := u4RuntimeRoot(t)
		transport, endpoint := u4ExpectedEndpoint(t, runtimeDir)
		ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
		defer cancel()
		listener, err := ListenMCPDaemonIPC(ctx, root, runtimeDir, transport, endpoint)
		require.NoError(t, err, "U4 must listen using the already-declared IPC entrypoint")
		require.NotNil(t, listener)
		u4ExchangeFour(t, ctx, listener, transport, endpoint)
		if runtime.GOOS != "windows" {
			info, statErr := root.Lstat(filepath.Base(endpoint))
			require.NoError(t, statErr, "nested os.Root must find the bound leaf handle-relatively")
			assert.NotZero(t, info.Mode()&os.ModeSocket)
			assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "UDS must be owner-only at bind")
		}
		require.NoError(t, listener.Close())
		_, err = listener.Accept()
		assert.ErrorIs(t, err, net.ErrClosed, "Close must unblock/refuse Accept")
	})
	t.Run("invalid_endpoints_fail_closed_without_writes", func(t *testing.T) {
		root, runtimeDir := u4RuntimeRoot(t)
		foreign := u4TempDir(t)
		transport, _ := u4ExpectedEndpoint(t, runtimeDir)
		cases := []struct {
			name     string
			endpoint string
		}{
			{"relative", "mcp.sock"},
			{"foreign_directory", filepath.Join(foreign, "mcp.sock")},
			{"disallowed_leaf", filepath.Join(runtimeDir, "not-a-socket")},
		}
		if runtime.GOOS == "windows" {
			cases[1].endpoint = `\\foreign-host\pipe\backlogit-mcp-0123456789abcdef`
			cases[2].endpoint = `\\.\pipe\not-backlogit`
		}
		executed := 0
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				for attempt := 0; attempt < 2; attempt++ {
					listener, err := ListenMCPDaemonIPC(t.Context(), root, runtimeDir, transport, tc.endpoint)
					executed++
					if listener != nil {
						assert.NoError(t, listener.Close(), "close unexpectedly admitted listener")
					}
					assert.Nil(t, listener)
					assert.ErrorIs(t, err, corerrors.ErrValidation, "invalid endpoint must be refused before binding")
				}
			})
		}
		assert.Equal(t, 6, executed, "non-vacuity: three refusal classes, each repeated")
		for _, dir := range []string{runtimeDir, foreign} {
			entries, err := os.ReadDir(dir)
			require.NoError(t, err)
			assert.Empty(t, entries, "refusal must not create endpoint artifacts")
		}
		t.Logf("U4_GUARD validation_calls=%d refusal_classes=3 skipped_fixtures=0", executed)
	})
	t.Run("platform_access_control", func(t *testing.T) {
		if runtime.GOOS != "windows" {
			u4POSIXSourceContract(t)
			return
		}
		u4WindowsPipeSecurity(t)
	})
}

func TestU4_IsVerifiedProcessAliveChecksIdentity(t *testing.T) {
	if os.Getenv("BACKLOGIT_U4_IDENTITY_CHILD") == "1" {
		_, err := fmt.Fprintln(os.Stdout, "u4-identity-ready")
		require.NoError(t, err)
		_, err = bufio.NewReader(os.Stdin).ReadByte()
		require.ErrorIs(t, err, io.EOF, "identity child exits cooperatively on stdin EOF")
		return
	}
	executable, err := os.Executable()
	require.NoError(t, err)
	start, err := processStartTime(os.Getpid())
	require.NoError(t, err)
	require.NotZero(t, start)
	cases := []struct {
		name  string
		pid   int
		start int64
		exe   string
		want  bool
	}{
		{"matching_current_process", os.Getpid(), start, executable, true},
		{"wrong_start_time", os.Getpid(), start + 1, executable, false},
		{"wrong_executable", os.Getpid(), start, filepath.Join(u4TempDir(t), "foreign-executable"), false},
		{"invalid_pid", -1, start, executable, false},
		{"zero_pid", 0, start, executable, false},
	}
	executed := 0
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			executed++
			assert.Equal(t, tc.want, isVerifiedProcessAlive(tc.pid, tc.start, tc.exe),
				"D5: liveness requires PID, creation time, and executable identity without signaling")
		})
	}
	// Capture a real child's matching identity while it is alive, then probe the
	// same tuple after cooperative exit. A mismatched tuple cannot prove death.
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	child := exec.CommandContext(ctx, executable, "-test.run=^TestU4_IsVerifiedProcessAliveChecksIdentity$")
	child.Env = append(os.Environ(), "BACKLOGIT_U4_IDENTITY_CHILD=1")
	stdin, err := child.StdinPipe()
	require.NoError(t, err)
	stdout, err := child.StdoutPipe()
	require.NoError(t, err)
	var stderr bytes.Buffer
	child.Stderr = &stderr
	require.NoError(t, child.Start())
	waited := false
	t.Cleanup(func() {
		_ = stdin.Close() // Idempotent cooperative child shutdown on assertion failure.
		if !waited {
			cancel()
			_ = child.Wait() // Cancellation may kill a child after a failed fixture assertion.
		}
	})
	line, err := bufio.NewReader(stdout).ReadString('\n')
	require.NoError(t, err, "identity child readiness")
	require.Equal(t, "u4-identity-ready\n", line)
	childStart, err := processStartTime(child.Process.Pid)
	require.NoError(t, err)
	assert.True(t, isVerifiedProcessAlive(child.Process.Pid, childStart, executable), "live child positive control")
	require.NoError(t, stdin.Close())
	err = child.Wait()
	waited = true
	require.NoError(t, err, "identity child exit: %s", stderr.String())
	assert.False(t, isVerifiedProcessAlive(child.ProcessState.Pid(), childStart, executable), "same identity must be dead after Wait")
	after, err := processStartTime(os.Getpid())
	require.NoError(t, err)
	assert.Equal(t, start, after, "identity probes must leave the process alive and unchanged")
	assert.True(t, isVerifiedProcessAlive(os.Getpid(), start, executable), "positive control after all negative probes")
	assert.Equal(t, 5, executed)
	t.Logf("U4_GUARD identity_table_cases=%d exited_child_cases=1 positive_controls=3", executed)
}

func u4RuntimeRoot(t *testing.T) (*os.Root, string) {
	t.Helper()
	storage := u4TempDir(t)
	runtimeDir := filepath.Join(storage, "runtime")
	require.NoError(t, os.Mkdir(runtimeDir, 0o700))
	storageRoot := u4OpenRelativeRoot(t, storage)
	t.Cleanup(func() { assert.NoError(t, storageRoot.Close()) })
	root, err := storageRoot.OpenRoot("runtime")
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, root.Close()) })
	require.False(t, filepath.IsAbs(root.Name()), "fixture must exercise the nested os.Root.Name regression")
	return root, runtimeDir
}

func u4OpenRelativeRoot(t *testing.T, dir string) *os.Root {
	t.Helper()
	// POSIX retains the parent's path when naming nested roots.
	cwd, err := os.Getwd()
	require.NoError(t, err)
	relative, err := filepath.Rel(cwd, dir)
	require.NoError(t, err)
	root, err := os.OpenRoot(relative)
	require.NoError(t, err)
	return root
}

func u4TempDir(t *testing.T) string {
	t.Helper()
	// t.TempDir includes the long test name, which itself exceeds sun_path.
	// Use a short owned directory under the package CWD, never system TEMP.
	cwd, err := os.Getwd()
	require.NoError(t, err)
	dir, err := os.MkdirTemp(cwd, ".u4-")
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, os.RemoveAll(dir)) })
	return dir
}

func u4ExpectedEndpoint(t *testing.T, runtimeDir string) (string, string) {
	t.Helper()
	// This independent oracle lets listener/refusal tests run even while Derive is RED.
	if runtime.GOOS == "windows" {
		storage, err := filepath.EvalSymlinks(filepath.Dir(runtimeDir))
		require.NoError(t, err)
		hash := sha256.Sum256([]byte(strings.ToLower(filepath.Clean(storage))))
		return "named_pipe", `\\.\pipe\backlogit-mcp-` + hex.EncodeToString(hash[:8])
	}
	endpoint := filepath.Join(runtimeDir, "mcp.sock")
	if len(endpoint) > 100 {
		endpoint = filepath.Join(runtimeDir, "s")
	}
	require.LessOrEqual(t, len(endpoint), 100, "native POSIX fixture needs a shorter workspace-contained TEMP")
	return "unix", endpoint
}

type u4Frame struct {
	Client int    `json:"client"`
	Seq    int    `json:"seq"`
	Body   string `json:"body"`
}

func u4ExchangeFour(t *testing.T, ctx context.Context, listener net.Listener, transport, endpoint string) {
	t.Helper()
	const clients, frames = 4, 3
	var workers sync.WaitGroup
	var accepted, connected, exchanged atomic.Int32
	results := make(chan error, clients*2+1)
	arrived := make(chan struct{}, clients)
	release := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(func() {
		unblock()
		// A failed dial must not strand Accept, and all workers join before root cleanup.
		_ = listener.Close() // Idempotent cleanup; the normal path asserts Close's result.
		workers.Wait()
	})
	workers.Add(1)
	go func() {
		defer workers.Done()
		for i := 0; i < clients; i++ {
			conn, err := listener.Accept()
			if err != nil {
				results <- fmt.Errorf("accept client: %w", err)
				return
			}
			accepted.Add(1)
			workers.Add(1)
			go func() {
				defer workers.Done()
				defer func() { _ = conn.Close() }() // Peer may already be closed on timeout.
				if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
					results <- fmt.Errorf("server deadline: %w", err)
					return
				}
				reader := bufio.NewReader(conn)
				for seq := 0; seq < frames; seq++ {
					line, err := reader.ReadBytes('\n')
					if err != nil {
						results <- fmt.Errorf("read frame: %w", err)
						return
					}
					var frame u4Frame
					if err := json.Unmarshal(line, &frame); err != nil {
						results <- fmt.Errorf("decode frame: %w", err)
						return
					}
					if frame.Seq != seq || frame.Client < 0 || frame.Client >= clients {
						results <- fmt.Errorf("misrouted frame %+v: %w", frame, corerrors.ErrValidation)
						return
					}
					if _, err := conn.Write(line); err != nil {
						results <- fmt.Errorf("echo frame: %w", err)
						return
					}
				}
				results <- nil
			}()
		}
	}()
	for id := 0; id < clients; id++ {
		workers.Add(1)
		go func(id int) {
			defer workers.Done()
			conn, err := DialMCPDaemonIPC(ctx, transport, endpoint, 5*time.Second)
			if err != nil {
				arrived <- struct{}{}
				results <- fmt.Errorf("dial client %d: %w", id, err)
				return
			}
			defer func() { _ = conn.Close() }() // Always close even after a failed frame.
			connected.Add(1)
			arrived <- struct{}{}
			select {
			case <-release:
			case <-ctx.Done():
				results <- fmt.Errorf("client barrier: %w", ctx.Err())
				return
			}
			if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
				results <- fmt.Errorf("client deadline: %w", err)
				return
			}
			reader := bufio.NewReader(conn)
			for seq := 0; seq < frames; seq++ {
				want := u4Frame{Client: id, Seq: seq, Body: strings.Repeat(strconv.Itoa(id), 4096)}
				data, err := json.Marshal(want)
				if err != nil {
					results <- fmt.Errorf("encode frame: %w", err)
					return
				}
				data = append(data, '\n')
				// Split writes force the transport to preserve framing across fragments.
				for _, part := range [][]byte{data[:7], data[7:]} {
					n, err := conn.Write(part)
					if err != nil {
						results <- fmt.Errorf("write fragment: %w", err)
						return
					}
					if n != len(part) {
						results <- fmt.Errorf("write fragment: %w", io.ErrShortWrite)
						return
					}
				}
				line, err := reader.ReadBytes('\n')
				if err != nil {
					results <- fmt.Errorf("read echo: %w", err)
					return
				}
				var got u4Frame
				if err := json.Unmarshal(line, &got); err != nil {
					results <- fmt.Errorf("decode echo: %w", err)
					return
				}
				if got != want {
					results <- fmt.Errorf("client %d response crossed frames: %w", id, corerrors.ErrValidation)
					return
				}
				exchanged.Add(1)
			}
			results <- nil
		}(id)
	}
	for i := 0; i < clients; i++ {
		select {
		case <-arrived:
		case <-ctx.Done():
			t.Fatal("four-client dial barrier did not complete")
		}
	}
	assert.EqualValues(t, clients, connected.Load(), "all four connections must coexist before sending frames")
	unblock()
	for i := 0; i < clients*2; i++ {
		select {
		case err := <-results:
			require.NoError(t, err)
		case <-ctx.Done():
			t.Fatal("framed IO did not drain before deadline")
		}
	}
	workers.Wait()
	assert.EqualValues(t, clients, accepted.Load())
	assert.EqualValues(t, clients*frames, exchanged.Load())
	t.Logf("U4_GUARD accepted=%d connected=%d echoed_frames=%d skipped_fixtures=0",
		accepted.Load(), connected.Load(), exchanged.Load())
}

func u4POSIXBoundaries(t *testing.T) {
	t.Helper()
	base := u4TempDir(t)
	fallback := filepath.Join(base, "outside-runtime-decoy")
	require.NoError(t, os.Mkdir(fallback, 0o700))
	for _, key := range []string{"TMPDIR", "XDG_RUNTIME_DIR", "HOME"} {
		t.Setenv(key, fallback)
	}
	cases := []struct {
		name       string
		shortBytes int
		leaf       string
	}{
		{"primary_exactly_100", 93, "mcp.sock"},
		{"primary_101_short_94", 94, "s"},
		{"short_exactly_100", 100, "s"},
		{"short_101_refused", 101, ""},
		{"utf8_byte_not_rune_limit", 102, ""},
	}
	executed := 0
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// runtime/s is shortBytes bytes; multibyte padding tests byte counting.
			padding := tc.shortBytes - len(filepath.Join(base, "runtime", "s")) - 1
			require.Positive(t, padding, "use a shorter workspace-contained TMPDIR for POSIX boundary fixtures")
			pad := strings.Repeat("x", padding)
			if tc.name == "utf8_byte_not_rune_limit" {
				require.GreaterOrEqual(t, padding, 2)
				pad = "é" + strings.Repeat("x", padding-2)
			}
			storage := filepath.Join(base, pad)
			runtimeDir := filepath.Join(storage, "runtime")
			require.NoError(t, os.MkdirAll(runtimeDir, 0o700))
			storageRoot := u4OpenRelativeRoot(t, storage)
			defer func() { assert.NoError(t, storageRoot.Close()) }()
			root, err := storageRoot.OpenRoot("runtime")
			require.NoError(t, err)
			defer func() { assert.NoError(t, root.Close()) }()
			require.False(t, filepath.IsAbs(root.Name()))
			require.Len(t, filepath.Join(runtimeDir, "s"), tc.shortBytes)
			for repeat := 0; repeat < 2; repeat++ {
				transport, endpoint, err := DeriveMCPDaemonEndpoint(base, storage, runtimeDir)
				executed++
				if tc.leaf == "" {
					assert.ErrorIs(t, err, corerrors.ErrValidation)
					assert.Empty(t, transport)
					assert.Empty(t, endpoint)
					if assert.Error(t, err) {
						assert.Contains(t, err.Error(), "100")
						assert.Contains(t, err.Error(), "shorter")
						assert.Contains(t, err.Error(), "BACKLOGIT_WORKSPACE_DIR")
					}
					listener, listenErr := ListenMCPDaemonIPC(t.Context(), root, runtimeDir, "unix", filepath.Join(runtimeDir, "s"))
					assert.Nil(t, listener)
					if listener != nil {
						assert.NoError(t, listener.Close())
					}
					assert.ErrorIs(t, listenErr, corerrors.ErrValidation)
				} else {
					require.NoError(t, err)
					assert.Equal(t, "unix", transport)
					assert.Equal(t, filepath.Join(runtimeDir, tc.leaf), endpoint)
					listener, err := ListenMCPDaemonIPC(t.Context(), root, runtimeDir, transport, endpoint)
					require.NoError(t, err)
					info, err := root.Lstat(tc.leaf)
					require.NoError(t, err)
					assert.NotZero(t, info.Mode()&os.ModeSocket)
					assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
					require.NoError(t, listener.Close())
				}
			}
			entries, err := root.Open(".")
			require.NoError(t, err)
			children, err := entries.ReadDir(-1)
			assert.NoError(t, err)
			assert.Empty(t, children)
			assert.NoError(t, entries.Close())
		})
	}
	entries, err := os.ReadDir(fallback)
	require.NoError(t, err)
	assert.Empty(t, entries, "no TMPDIR/XDG_RUNTIME_DIR/HOME fallback writes")
	assert.Equal(t, 10, executed)
	t.Logf("U4_GUARD posix_boundary_calls=%d boundary_cases=5 outside_fallback_artifacts=%d", executed, len(entries))
}

// Source guards complement, not substitute for, native POSIX execution. Parse
// without comments so a TODO or copied contract cannot satisfy these checks.
func u4POSIXSourceContract(t *testing.T) {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "mcp_ipc_unix.go", nil, 0)
	require.NoError(t, err)
	calls := make(map[string]int)
	var literals []string
	var names []string
	directoryEquality, lengthGuards := 0, 0
	ast.Inspect(file, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.BinaryExpr:
			var expression bytes.Buffer
			require.NoError(t, format.Node(&expression, token.NewFileSet(), n))
			text := expression.String()
			if (n.Op == token.EQL || n.Op == token.NEQ) &&
				strings.Contains(text, "filepath.Clean(filepath.Dir(endpoint))") &&
				strings.Contains(text, "filepath.Clean(runtimeDir)") {
				directoryEquality++
			}
			if strings.Contains(text, "len(") && strings.Contains(text, "100") &&
				(n.Op == token.GTR || n.Op == token.LEQ) {
				lengthGuards++
			}
		case *ast.CallExpr:
			if sel, ok := n.Fun.(*ast.SelectorExpr); ok {
				if ident, ok := sel.X.(*ast.Ident); ok {
					calls[ident.Name+"."+sel.Sel.Name]++
				}
				assert.NotEqual(t, "Name", sel.Sel.Name, "never use nested runtimeRoot.Name as an absolute directory")
				assert.NotEqual(t, "Chmod", sel.Sel.Name, "0600 must be atomic at bind, not a path-based chmod")
			}
		case *ast.BasicLit:
			literals = append(literals, n.Value)
		case *ast.Ident:
			names = append(names, n.Name)
		}
		return true
	})
	for _, call := range []string{"filepath.IsAbs", "filepath.Clean", "filepath.Dir", "filepath.Base", "net.Listen", "syscall.Umask", "runtimeRoot.Lstat", "runtimeRoot.Remove"} {
		assert.Positive(t, calls[call], "POSIX implementation must execute %s", call)
	}
	for _, call := range []string{"os.TempDir", "os.UserHomeDir", "os.Getenv", "os.LookupEnv", "os.MkdirTemp"} {
		assert.Zero(t, calls[call], "D2 prohibits outside-workspace fallback via %s", call)
	}
	assert.GreaterOrEqual(t, calls["syscall.Umask"], 2, "set owner-only mask and restore it")
	assert.Positive(t, directoryEquality, "compare cleaned endpoint directory to runtimeDir, never Root.Name")
	assert.GreaterOrEqual(t, lengthGuards, 2, "primary and short leaf need byte-length guards")
	for _, name := range []string{"Lock", "Unlock", "ModeSocket", "ErrValidation", "endpoint", "runtimeDir"} {
		assert.Contains(t, names, name)
	}
	for _, literal := range []string{`"mcp.sock"`, `"s"`, `"unix"`, "100"} {
		assert.Contains(t, literals, literal)
	}
	// Accept either modern or legacy octal spelling, never a decimal permission.
	assert.True(t, u4HasLiteral(literals, "0o177", "0177"), "bind under Umask(0o177)")
	assert.True(t, u4HasLiteral(literals, "0o600", "0600"), "verify bound socket permissions")
	allLiterals := strings.Join(literals, "\n")
	for _, literal := range []string{`"/tmp"`, `"/var/tmp"`} {
		assert.NotContains(t, literals, literal, "no hardcoded outside-workspace fallback")
	}
	assert.Contains(t, allLiterals, "shorter")
	assert.Contains(t, allLiterals, "BACKLOGIT_WORKSPACE_DIR")
	t.Log("U4_GUARD posix_source_required_calls=8 forbidden_fallback_calls=5 skipped_fixtures=0")
}

func u4HasLiteral(literals []string, alternatives ...string) bool {
	for _, literal := range literals {
		for _, alternative := range alternatives {
			if literal == alternative {
				return true
			}
		}
	}
	return false
}

func u4WindowsPipeSecurity(t *testing.T) {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "mcp_ipc_windows.go", nil, 0)
	require.NoError(t, err)
	var identifiers []string
	var stringsFound []string
	ast.Inspect(file, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.Ident:
			identifiers = append(identifiers, n.Name)
		case *ast.BasicLit:
			if n.Kind == token.STRING {
				value, err := strconv.Unquote(n.Value)
				require.NoError(t, err)
				stringsFound = append(stringsFound, value)
			}
		}
		return true
	})
	assert.Contains(t, identifiers, "CreateNamedPipe")
	assert.Contains(t, identifiers, "PIPE_REJECT_REMOTE_CLIENTS", "D2 requires kernel remote-client rejection")
	assert.Contains(t, identifiers, "GetTokenUser", "DACL must derive from current process owner SID")
	assert.Contains(t, identifiers, "SecurityDescriptorFromString")
	assert.True(t, strings.Contains(strings.Join(stringsFound, "\n"), "D:P(A;;GA;;;"),
		"protected current-user-only DACL, not Everyone/Administrators defaults")

	root, runtimeDir := u4RuntimeRoot(t)
	transport, endpoint := u4ExpectedEndpoint(t, runtimeDir)
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	listener, err := ListenMCPDaemonIPC(ctx, root, runtimeDir, transport, endpoint)
	require.NoError(t, err, "native security descriptor probe requires a real listener")
	done := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			err = conn.Close()
		}
		done <- err
	}()
	t.Cleanup(func() {
		_ = listener.Close() // Unblock Accept even if the native security probe fails.
		<-done
	})
	// Keep the test untagged/cross-compilable: query the live kernel pipe DACL
	// through a bounded native PowerShell probe, not production-private symbols.
	script := `
$ErrorActionPreference = 'Stop'
Add-Type -TypeDefinition @'
using System;
using System.Runtime.InteropServices;
public static class U4PipeSecurity {
 [DllImport("advapi32.dll", CharSet=CharSet.Unicode)]
 public static extern uint GetNamedSecurityInfo(string name, uint type, uint info,
   out IntPtr owner, out IntPtr group, out IntPtr dacl, out IntPtr sacl, out IntPtr sd);
 [DllImport("advapi32.dll")]
 public static extern uint GetSecurityDescriptorLength(IntPtr sd);
 [DllImport("kernel32.dll")]
 public static extern IntPtr LocalFree(IntPtr p);
}
'@
$owner = $group = $dacl = $sacl = $sd = [IntPtr]::Zero
$code = [U4PipeSecurity]::GetNamedSecurityInfo($env:BACKLOGIT_U4_PIPE, 1, 4,
  [ref]$owner, [ref]$group, [ref]$dacl, [ref]$sacl, [ref]$sd)
if ($code -ne 0) { throw "GetNamedSecurityInfo failed: $code" }
try {
 $bytes = [byte[]]::new([U4PipeSecurity]::GetSecurityDescriptorLength($sd))
 [Runtime.InteropServices.Marshal]::Copy($sd, $bytes, 0, $bytes.Length)
 $descriptor = [Security.AccessControl.RawSecurityDescriptor]::new($bytes, 0)
 $user = [Security.Principal.WindowsIdentity]::GetCurrent().User.Value
 if (($descriptor.ControlFlags -band [Security.AccessControl.ControlFlags]::DiscretionaryAclProtected) -eq 0) {
   throw 'pipe DACL is not protected'
 }
 if ($null -eq $descriptor.DiscretionaryAcl -or $descriptor.DiscretionaryAcl.Count -ne 1) {
   throw 'pipe must have exactly one current-user allow ACE'
 }
 $ace = $descriptor.DiscretionaryAcl[0]
 if ($ace.SecurityIdentifier.Value -ne $user -or
     $ace.AceQualifier -ne [Security.AccessControl.AceQualifier]::AccessAllowed -or
     (($ace.AccessMask -band 0x10000000) -eq 0 -and
      ($ace.AccessMask -band 0x1f01ff) -ne 0x1f01ff)) { throw 'pipe ACE is not current-user GA' }
 'U4_PIPE_DACL_CURRENT_USER_ONLY'
} finally { [void][U4PipeSecurity]::LocalFree($sd) }
`
	command := exec.CommandContext(ctx, "pwsh", "-NoProfile", "-NonInteractive", "-Command", script)
	command.Env = append(os.Environ(), "BACKLOGIT_U4_PIPE="+endpoint)
	output, err := command.CombinedOutput()
	require.NoError(t, err, "native pipe DACL probe: %s", output)
	assert.Contains(t, string(output), "U4_PIPE_DACL_CURRENT_USER_ONLY")
	t.Log("U4_GUARD windows_live_dacl_probes=1 remote_rejection_source_guards=1 skipped_fixtures=0")
}
