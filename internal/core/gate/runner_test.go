package gate

import (
	"context"
	stderrors "errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	bkerrors "github.com/softwaresalt/backlogit/internal/errors"
)

func TestBuildArgs_GoldenAndNoShell(t *testing.T) {
	got := BuildArgs(GateCheckRequest{
		ItemID:        "082.002-T",
		BaseRef:       "origin/main",
		HeadRef:       HeadRef,
		WorkspaceRoot: "/repo",
	})
	want := []string{
		"gate", "check", "--json",
		"--base", "origin/main",
		"--head", "HEAD",
		"--workspace", "/repo",
		"--task", "082.002-T",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("argv = %v, want %v", got, want)
	}

	// Force appends --force; NoCount appends --no-count.
	forced := BuildArgs(GateCheckRequest{BaseRef: "main", HeadRef: HeadRef, WorkspaceRoot: ".", Force: true})
	if !containsArg(forced, "--force") {
		t.Fatalf("expected --force in %v", forced)
	}
	adv := BuildArgs(GateCheckRequest{BaseRef: "main", HeadRef: HeadRef, WorkspaceRoot: ".", NoCount: true})
	if !containsArg(adv, "--no-count") {
		t.Fatalf("expected --no-count in %v", adv)
	}

	// A base ref carrying shell metacharacters is a single discrete argv element,
	// never split or interpreted (argv-array exec, no shell string).
	evil := "main; rm -rf / && echo pwned `whoami`"
	args := BuildArgs(GateCheckRequest{BaseRef: evil, HeadRef: HeadRef, WorkspaceRoot: "."})
	found := false
	for i, a := range args {
		if a == "--base" && i+1 < len(args) {
			if args[i+1] != evil {
				t.Fatalf("base ref mangled: %q", args[i+1])
			}
			found = true
		}
	}
	if !found {
		t.Fatal("base ref not present as a single arg")
	}
	// No argv element may itself contain a shell-joined command string.
	for _, a := range args {
		if strings.Contains(a, "gate check --json") {
			t.Fatalf("argv element looks shell-joined: %q", a)
		}
	}
}

func containsArg(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

func TestExecRunner_BinaryNotFound(t *testing.T) {
	r := ExecRunner{Binary: filepath.Join(t.TempDir(), "does-not-exist-autoharness")}
	_, err := r.Run(context.Background(), []string{"gate", "check"}, t.TempDir(), MinimalEnv())
	if !stderrors.Is(err, bkerrors.ErrGateBinaryNotFound) {
		t.Fatalf("err = %v, want ErrGateBinaryNotFound", err)
	}
}

func TestExecRunner_RanAndExited(t *testing.T) {
	res, err := runHelper(t, "exit1", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ExitCode != 1 {
		t.Fatalf("exit code = %d, want 1", res.ExitCode)
	}
	if !strings.Contains(string(res.Stdout), "repeated_failure") {
		t.Fatalf("stdout not captured: %q", res.Stdout)
	}

	res0, err := runHelper(t, "exit0", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res0.ExitCode != 0 {
		t.Fatalf("exit code = %d, want 0", res0.ExitCode)
	}
}

func TestExecRunner_Timeout(t *testing.T) {
	_, err := runHelper(t, "sleep", 100*time.Millisecond)
	if !stderrors.Is(err, bkerrors.ErrGateTimeout) {
		t.Fatalf("err = %v, want ErrGateTimeout", err)
	}
}

// runHelper invokes this test binary as the gate subprocess via TestHelperProcess.
func runHelper(t *testing.T, behavior string, timeout time.Duration) (GateResult, error) {
	t.Helper()
	ctx := context.Background()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	r := ExecRunner{Binary: os.Args[0]}
	args := []string{"-test.run=TestHelperProcess", "--", behavior}
	env := append(os.Environ(), "GO_GATE_HELPER=1")
	return r.Run(ctx, args, t.TempDir(), env)
}

// TestHelperProcess is not a real test; it is re-executed as the gate subprocess.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_GATE_HELPER") != "1" {
		return
	}
	behavior := ""
	for i, a := range os.Args {
		if a == "--" && i+1 < len(os.Args) {
			behavior = os.Args[i+1]
			break
		}
	}
	switch behavior {
	case "exit1":
		fmt.Fprint(os.Stdout, `{"repeated_failure":{"count":1,"threshold":3,"reached":false,"action":"block"}}`)
		os.Exit(1)
	case "exit0":
		fmt.Fprint(os.Stdout, "ok")
		os.Exit(0)
	case "sleep":
		time.Sleep(5 * time.Second)
		os.Exit(0)
	default:
		os.Exit(0)
	}
}
