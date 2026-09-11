package fl005good

import (
	"context"
	"sync"
	"time"
)

type contextAcquirer interface {
	Acquire(context.Context) error
}

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
