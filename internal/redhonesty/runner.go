package redhonesty

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/softwaresalt/backlogit/internal/faultline"
)

type goTestEvent struct {
	Action  string `json:"Action"`
	Package string `json:"Package"`
	Test    string `json:"Test"`
}

// parseGoTestJSON reduces `go test -json` output to a per-top-level-test
// result. A failure is sticky; a skip is not a verifiable outcome and maps to
// ResultUnrunnable. Lines that are not JSON events (build chatter) are ignored,
// and a test with no terminal event is simply absent. Input is bounded per line
// and in total.
func parseGoTestJSON(r io.Reader) (map[string]Result, error) {
	cr := &countingReader{r: r, max: maxTestOutputBytes}
	sc := bufioLineScanner(cr)
	out := map[string]Result{}
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		var ev goTestEvent
		if err := json.Unmarshal(line, &ev); err != nil || ev.Test == "" || strings.Contains(ev.Test, "/") {
			continue
		}
		switch ev.Action {
		case "pass":
			if out[ev.Test] != ResultFail {
				out[ev.Test] = ResultPass
			}
		case "fail":
			out[ev.Test] = ResultFail
		case "skip":
			if _, seen := out[ev.Test]; !seen {
				out[ev.Test] = ResultUnrunnable
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("redhonesty: read go test output: %w", err)
	}
	if cr.exceeded {
		return nil, fmt.Errorf("redhonesty: go test output exceeds %d bytes", maxTestOutputBytes)
	}
	return out, nil
}

type countingReader struct {
	r        io.Reader
	n, max   int64
	exceeded bool
}

var errOutputLimit = errors.New("redhonesty: output limit exceeded")

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	if c.n > c.max {
		c.exceeded = true
		return n, errOutputLimit
	}
	return n, err
}

// RunTests runs the tests grouped by package through ex and returns a result
// for every test keyed by NewTest.Key(). A test the executor could not observe
// (build failure, no such test, skip, timeout) is ResultUnrunnable, never
// silently absent or passing.
func RunTests(ctx context.Context, ex TestExecutor, dir string, tests []NewTest) (map[string]Result, error) {
	byPkg := map[string][]string{}
	for _, t := range tests {
		if err := validPackageDir(t.Package); err != nil {
			return nil, err
		}
		if !faultline.ValidTestName(t.Name) {
			return nil, fmt.Errorf("redhonesty: unsafe test name %q", truncate(t.Name))
		}
		byPkg[t.Package] = append(byPkg[t.Package], t.Name)
	}
	pkgs := make([]string, 0, len(byPkg))
	for p := range byPkg {
		pkgs = append(pkgs, p)
	}
	sort.Strings(pkgs)
	observed := map[string]Result{}
	for _, p := range pkgs {
		names := dedupSorted(byPkg[p])
		res, err := ex.Run(ctx, dir, p, names)
		if err != nil {
			return nil, fmt.Errorf("redhonesty: run %s: %w", p, err)
		}
		for n, r := range res {
			observed[NewTest{Package: p, Name: n}.Key()] = r
		}
	}
	out := make(map[string]Result, len(tests))
	for _, t := range tests {
		r, ok := observed[t.Key()]
		if !ok || (r != ResultPass && r != ResultFail) {
			r = ResultUnrunnable
		}
		out[t.Key()] = r
	}
	return out, nil
}

func dedupSorted(in []string) []string {
	sort.Strings(in)
	out := in[:0]
	for _, s := range in {
		if len(out) == 0 || out[len(out)-1] != s {
			out = append(out, s)
		}
	}
	return out
}
