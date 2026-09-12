package fl005bad

import (
	"context"
	"sync"
	"time"
)

type acquirer struct{}

func (acquirer) Acquire() {}

type variadicAcquirer struct{}

func (variadicAcquirer) Acquire(...context.Context) {}

type customLock struct{}

func (customLock) Lock() {}

func mutex(parent context.Context, mu *sync.Mutex) {
	ctx, cancel := context.WithTimeout(parent, time.Second)
	defer cancel()
	_ = ctx
	mu.Lock() // want "FL005"
}

func rwMutex(parent context.Context, mu *sync.RWMutex) {
	ctx, cancel := context.WithDeadline(parent, time.Now().Add(time.Second))
	defer cancel()
	_ = ctx
	mu.Lock() // want "FL005"
}

func rwRead(parent context.Context, mu *sync.RWMutex) {
	ctx, cancel := context.WithTimeout(parent, time.Second)
	defer cancel()
	_ = ctx
	mu.RLock() // want "FL005"
}

func namedAcquire(parent context.Context, lock acquirer) {
	ctx, cancel := context.WithTimeout(parent, time.Second)
	defer cancel()
	_ = ctx
	lock.Acquire() // want "FL005"
}

func zeroArgumentVariadicAcquire(parent context.Context, lock variadicAcquirer) {
	ctx, cancel := context.WithTimeout(parent, time.Second)
	defer cancel()
	_ = ctx
	lock.Acquire() // want "FL005"
}

func namedLock(parent context.Context, lock customLock) {
	ctx, cancel := context.WithTimeout(parent, time.Second)
	defer cancel()
	_ = ctx
	lock.Lock() // want "FL005"
}

func nestedDescendant(parent context.Context, mu *sync.Mutex) {
	ctx, cancel := context.WithTimeout(parent, time.Second)
	defer cancel()
	_ = ctx
	if true {
		{
			mu.Lock() // want "FL005"
		}
	}
}

func valueSpec(parent context.Context, mu *sync.Mutex) {
	var ctx, cancel = context.WithTimeout(parent, time.Second)
	defer cancel()
	_ = ctx
	mu.Lock() // want "FL005"
}

func unrelatedTrailingDirective(parent context.Context, mu *sync.Mutex) {
	ctx, cancel := context.WithTimeout(parent, time.Second)
	defer cancel()
	_ = ctx
	for mu.Lock(); /* want "FL005" */ false; mu.Lock() { // faultline:lock-nonctx-ok
	}
}

func nearMissPrecedingDirective(parent context.Context, mu *sync.Mutex) {
	ctx, cancel := context.WithTimeout(parent, time.Second)
	defer cancel()
	_ = ctx
	// faultline:lock-nonctx-ok

	mu.Lock() // want "FL005"
}

func conditionalReset(parent context.Context, mu *sync.Mutex, reset bool) {
	ctx, cancel := context.WithTimeout(parent, time.Second)
	defer cancel()
	if reset {
		ctx = context.Background()
	}
	_ = ctx
	mu.Lock() // want "FL005"
}

func selfAssignment(parent context.Context, mu *sync.Mutex) {
	ctx, cancel := context.WithTimeout(parent, time.Second)
	defer cancel()
	ctx = ctx
	_ = ctx
	mu.Lock() // want "FL005"
}

func optionalLoopReset(parent context.Context, mu *sync.Mutex, reset bool) {
	ctx, cancel := context.WithTimeout(parent, time.Second)
	defer cancel()
	for reset {
		ctx = context.Background()
		break
	}
	_ = ctx
	mu.Lock() // want "FL005"
}

func valuePreservingAssignmentInsideConditionalPath(
	parent context.Context,
	mu *sync.Mutex,
	reset bool,
) {
	ctx, cancel := context.WithTimeout(parent, time.Second)
	defer cancel()
	if reset {
		ctx = context.WithValue(ctx, struct{}{}, "value")
		mu.Lock() // want "FL005"
	}
	_ = ctx
}

func sourceInsideSwitchCaseWithoutReset(
	parent context.Context,
	mu *sync.Mutex,
	value int,
) {
	switch value {
	case 1:
		ctx, cancel := context.WithTimeout(parent, time.Second)
		defer cancel()
		_ = ctx
		mu.Lock() // want "FL005"
	}
}

func sourceInsideSelectClauseWithoutReset(
	parent context.Context,
	mu *sync.Mutex,
	ready <-chan struct{},
) {
	select {
	case <-ready:
		ctx, cancel := context.WithTimeout(parent, time.Second)
		defer cancel()
		_ = ctx
		mu.Lock() // want "FL005"
	}
}

func clauseLocalSuppressionNearMisses(parent context.Context, mu *sync.Mutex, value int) {
	ctx, cancel := context.WithTimeout(parent, time.Second)
	defer cancel()
	_ = ctx
	switch value {
	case 1:
		// faultline:lock-nonctx-ok
		mu.Lock()
		mu.Lock() // want "FL005"
	case 2:
		// faultline:lock-nonctx-ok
		_ = value
		mu.Lock() // want "FL005"
	case 3:
		// faultline:lock-nonctx-ok
	case 4:
		mu.Lock() // want "FL005"
	}
}
