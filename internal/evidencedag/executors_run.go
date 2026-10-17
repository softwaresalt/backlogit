package evidencedag

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"sync"
	"time"
)

// DefaultNodeTimeout bounds each executor call when ExecOptions.NodeTimeout is
// zero.
const DefaultNodeTimeout = 30 * time.Second

// maxAbandonedExecutors caps timed-out executors still running; it is a
// variable only so tests can lower it.
var maxAbandonedExecutors = 16

// ExecOptions tunes RunExecutorsWith.
type ExecOptions struct {
	// NodeTimeout bounds each executor call. Zero means DefaultNodeTimeout;
	// negative is invalid. Executors must honour the context they are given.
	// One that does not is abandoned on timeout (its goroutine keeps running
	// until it returns, possibly forever) and the node fails closed. Once
	// maxAbandonedExecutors such goroutines are alive, RunExecutorsWith stops
	// with ErrExecutorsAbandoned instead of leaking more.
	NodeTimeout time.Duration
}

// RunExecutors runs the bound executor of every applicable node, in sorted
// node order, and returns bound evidence records plus bounded per-node failure
// reasons. Executors only produce artifacts; verification happens in the
// evaluator. A panic (in Identity or Execute), error, nil or typed-nil
// binding, identity mismatch or non-conformant artifact is a failure for that
// node and never evidence. A run with no applicable bound node is vacuous: it
// yields no evidence and no failures, which the evaluator reports as unmet,
// never as satisfied.
func RunExecutors(ctx context.Context, d *Declaration, s Shipment, bindings map[string]NodeExecutor) (ExecutionResult, error) {
	return RunExecutorsWith(ctx, d, s, bindings, ExecOptions{})
}

// RunExecutorsWith is RunExecutors with options.
func RunExecutorsWith(ctx context.Context, d *Declaration, s Shipment, bindings map[string]NodeExecutor, opts ExecOptions) (ExecutionResult, error) {
	res := ExecutionResult{Evidence: map[string]EvidenceRecord{}, Failures: map[string]string{}}
	if ctx == nil {
		return res, fmt.Errorf("%w: nil context", ErrInvalidInput)
	}
	if d == nil {
		return res, fmt.Errorf("%w: nil declaration", ErrInvalidDeclaration)
	}
	if err := d.Validate(); err != nil {
		return res, err
	}
	if err := validateShipment(s); err != nil {
		return res, err
	}
	timeout := opts.NodeTimeout
	if timeout < 0 {
		return res, fmt.Errorf("%w: negative node timeout", ErrInvalidInput)
	}
	if timeout == 0 {
		timeout = DefaultNodeTimeout
	}
	nodes := make(map[string]Node, len(d.Nodes))
	for _, n := range d.Nodes {
		nodes[n.ID] = n
	}
	ids := make([]string, 0, len(bindings))
	for id := range bindings {
		if _, ok := nodes[id]; !ok {
			return res, fmt.Errorf("%w: executor bound to undeclared node %q", ErrInvalidInput, truncate(id))
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return res, fmt.Errorf("evidencedag: executors interrupted: %w", err)
		}
		n, x := nodes[id], bindings[id]
		applicable, err := ResolveApplicability(n, s)
		if err != nil {
			res.Failures[id] = truncate(err.Error())
			continue
		}
		if !applicable {
			continue
		}
		if isNilExecutor(x) {
			res.Failures[id] = "executor binding is nil"
			continue
		}
		identity, err := safeIdentity(x)
		if err != nil {
			res.Failures[id] = truncate(err.Error())
			continue
		}
		if identity != n.Validator {
			res.Failures[id] = "executor identity does not match the node validator"
			continue
		}
		if abandonedCount() >= maxAbandonedExecutors {
			return res, fmt.Errorf("%w: %d still running", ErrExecutorsAbandoned, abandonedCount())
		}
		rec, err := runOne(ctx, x, ExecRequest{Shipment: s, Node: n}, timeout)
		if err != nil {
			res.Failures[id] = truncate(err.Error())
			continue
		}
		res.Evidence[id] = rec
	}
	return res, nil
}

func isNilExecutor(x NodeExecutor) bool {
	if x == nil {
		return true
	}
	v := reflect.ValueOf(x)
	switch v.Kind() {
	case reflect.Pointer, reflect.Map, reflect.Slice, reflect.Func, reflect.Chan, reflect.Interface:
		return v.IsNil()
	}
	return false
}

func safeIdentity(x NodeExecutor) (id string, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("executor identity panicked: %v", p)
		}
	}()
	return x.Identity(), nil
}

type runOutcome struct {
	rec EvidenceRecord
	err error
}

// abandonedExecutors counts executors whose node timed out or was interrupted
// but whose goroutine has not returned yet. Executors that ignore their
// context cannot be killed, so the count is capped to bound leaked goroutines.
var abandonedExecutors struct {
	mu sync.Mutex
	n  int
}

func abandonedCount() int {
	abandonedExecutors.mu.Lock()
	defer abandonedExecutors.mu.Unlock()
	return abandonedExecutors.n
}

type runState struct {
	mu        sync.Mutex
	done      bool
	abandoned bool
}

func (s *runState) abandon() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.done {
		return
	}
	s.abandoned = true
	abandonedExecutors.mu.Lock()
	abandonedExecutors.n++
	abandonedExecutors.mu.Unlock()
}

func (s *runState) finish() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.done = true
	if s.abandoned {
		abandonedExecutors.mu.Lock()
		abandonedExecutors.n--
		abandonedExecutors.mu.Unlock()
	}
}

// runOne always runs the executor in its own goroutine and waits for its
// result, the timeout or cancellation of ctx, whichever comes first.
func runOne(ctx context.Context, x NodeExecutor, req ExecRequest, timeout time.Duration) (EvidenceRecord, error) {
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ch := make(chan runOutcome, 1)
	st := &runState{}
	go func() {
		defer st.finish()
		rec, err := runGuarded(cctx, x, req)
		ch <- runOutcome{rec, err}
	}()
	select {
	case o := <-ch:
		return o.rec, o.err
	case <-cctx.Done():
		st.abandon()
		if errors.Is(cctx.Err(), context.DeadlineExceeded) {
			return EvidenceRecord{}, fmt.Errorf("executor timed out after %s", timeout)
		}
		return EvidenceRecord{}, fmt.Errorf("executor interrupted: %w", cctx.Err())
	}
}

func runGuarded(ctx context.Context, x NodeExecutor, req ExecRequest) (rec EvidenceRecord, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("executor panicked: %v", p)
		}
	}()
	a, err := x.Execute(ctx, req)
	if err != nil {
		return EvidenceRecord{}, err
	}
	return NewEvidenceRecord(req.Shipment, req.Node.ID, a)
}
