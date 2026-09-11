package fl005good

import (
	"context"
	"sync"
	"time"
)

type contextAcquirer interface {
	Acquire(context.Context) error
}

type customLock struct{}

func (customLock) Lock() {}

func noTimeout(mu *sync.Mutex) {
	mu.Lock()
}

func contextAware(parent context.Context, lock contextAcquirer) error {
	ctx, cancel := context.WithTimeout(parent, time.Second)
	defer cancel()
	return lock.Acquire(ctx)
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

func unrelatedCustomLock(parent context.Context, lock customLock) {
	ctx, cancel := context.WithTimeout(parent, time.Second)
	defer cancel()
	_ = ctx
	lock.Lock()
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
