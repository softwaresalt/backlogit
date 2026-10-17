package redhonesty

import (
	"bufio"
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
	br := bufio.NewReaderSize(cr, maxJSONLineBytes)
	out := map[string]Result{}
	terminal := map[string]bool{}
	for {
		line, isPrefix, err := br.ReadLine()
		if err != nil {
			if cr.exceeded {
				return nil, fmt.Errorf("redhonesty: go test output exceeds %d bytes", maxTestOutputBytes)
			}
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("redhonesty: read go test output: %w", err)
		}
		if isPrefix {
			// An over-long line is test chatter, never a result event (those are
			// tiny): discard it so one noisy test cannot abort the whole run.
			for isPrefix && err == nil {
				_, isPrefix, err = br.ReadLine()
			}
			if cr.exceeded {
				return nil, fmt.Errorf("redhonesty: go test output exceeds %d bytes", maxTestOutputBytes)
			}
			continue
		}
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		var ev goTestEvent
		if err := json.Unmarshal(line, &ev); err != nil || ev.Test == "" || strings.Contains(ev.Test, "/") {
			continue
		}
		switch ev.Action {
		case "pass", "fail", "skip":
		default:
			continue
		}
		// A second terminal event for one top-level name means two tests share it
		// (an internal and an external test package, or a rerun): the outcome is
		// not attributable to a single test, so it is unverifiable, never merged.
		if terminal[ev.Test] {
			out[ev.Test] = ResultUnrunnable
			continue
		}
		terminal[ev.Test] = true
		switch ev.Action {
		case "pass":
			out[ev.Test] = ResultPass
		case "fail":
			out[ev.Test] = ResultFail
		case "skip":
			out[ev.Test] = ResultUnrunnable
		}
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
