package fl005good

import (
	"context"
	"sync"
	"time"
)

type contextAcquirer interface {
	Acquire(context.Context) error
}

type variadicContextAcquirer interface {
	Acquire(...context.Context) error
}

type contextLocker interface {
	Lock(context.Context) error
}

type functionFields struct {
	Acquire func()
	Lock    func()
}

func Acquire() {}

func Lock() {}

func noTimeout(mu *sync.Mutex) {
	mu.Lock()
}

func contextAware(parent context.Context, lock contextAcquirer) error {
	ctx, cancel := context.WithTimeout(parent, time.Second)
	defer cancel()
	return lock.Acquire(ctx)
}

func variadicContextAware(parent context.Context, lock variadicContextAcquirer) error {
	ctx, cancel := context.WithTimeout(parent, time.Second)
	defer cancel()
	return lock.Acquire(ctx)
}

func contextAwareLock(parent context.Context, lock contextLocker) error {
	ctx, cancel := context.WithTimeout(parent, time.Second)
	defer cancel()
	return lock.Lock(ctx)
}

func suppressed(parent context.Context, mu *sync.Mutex) {
	ctx, cancel := context.WithTimeout(parent, time.Second)
	defer cancel()
	_ = ctx
	mu.Lock() // faultline:lock-nonctx-ok
}

func suppressedPrecedingLine(parent context.Context, mu *sync.Mutex) {
	ctx, cancel := context.WithTimeout(parent, time.Second)
	defer cancel()
	_ = ctx
	// faultline:lock-nonctx-ok
	mu.Lock()
}

func siblingBlock(parent context.Context, mu *sync.Mutex) {
	{
		ctx, cancel := context.WithTimeout(parent, time.Second)
		defer cancel()
		_ = ctx
	}
	{
		mu.Lock()
	}
}

func shadowedInDescendant(parent context.Context, mu *sync.Mutex) {
	ctx, cancel := context.WithTimeout(parent, time.Second)
	defer cancel()
	_ = ctx
	{
		ctx := context.Background()
		_ = ctx
		mu.Lock()
	}
}

func reversed(parent context.Context, mu *sync.Mutex) {
	mu.Lock()
	ctx, cancel := context.WithTimeout(parent, time.Second)
	defer cancel()
	_ = ctx
}

func reassigned(parent context.Context, mu *sync.Mutex) {
	ctx, cancel := context.WithTimeout(parent, time.Second)
	defer cancel()
	ctx = context.Background()
	_ = ctx
	mu.Lock()
}

func withCancel(parent context.Context, mu *sync.Mutex) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	_ = ctx
	mu.Lock()
}

func unrelatedFunctions(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, time.Second)
	defer cancel()
	_ = ctx
	Acquire()
	Lock()
}

func unrelatedFunctionFields(parent context.Context, fields functionFields) {
	ctx, cancel := context.WithTimeout(parent, time.Second)
	defer cancel()
	_ = ctx
	fields.Acquire()
	fields.Lock()
}

func functionLiteralIsolation(parent context.Context, mu *sync.Mutex) {
	ctx, cancel := context.WithTimeout(parent, time.Second)
	defer cancel()
	_ = ctx
	func() {
		mu.Lock()
	}()
}

func functionLiteralSourceIsolation(parent context.Context, mu *sync.Mutex) {
	func() {
		ctx, cancel := context.WithTimeout(parent, time.Second)
		defer cancel()
		_ = ctx
	}()
	mu.Lock()
}

func reassignedInsideConditionalPath(parent context.Context, mu *sync.Mutex, reset bool) {
	ctx, cancel := context.WithTimeout(parent, time.Second)
	defer cancel()
	if reset {
		ctx = context.Background()
		mu.Lock()
	}
	_ = ctx
}

func reassignedInsideUnconditionalNestedBlock(parent context.Context, mu *sync.Mutex) {
	ctx, cancel := context.WithTimeout(parent, time.Second)
	defer cancel()
	{
		ctx = context.Background()
		mu.Lock()
	}
	_ = ctx
}

func reassignedInsideSwitchCase(parent context.Context, mu *sync.Mutex, value int) {
	ctx, cancel := context.WithTimeout(parent, time.Second)
	defer cancel()
	switch value {
	case 1:
		ctx = context.Background()
		mu.Lock()
	}
	_ = ctx
}

func reassignedInsideTypeSwitchCase(parent context.Context, mu *sync.Mutex, value interface{}) {
	ctx, cancel := context.WithTimeout(parent, time.Second)
	defer cancel()
	switch value.(type) {
	case string:
		ctx = context.Background()
		mu.Lock()
	}
	_ = ctx
}

func reassignedInsideSelectClause(parent context.Context, mu *sync.Mutex, ready <-chan struct{}) {
	ctx, cancel := context.WithTimeout(parent, time.Second)
	defer cancel()
	select {
	case <-ready:
		ctx = context.Background()
		mu.Lock()
	}
	_ = ctx
}
