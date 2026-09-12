package compatcorpus

import "context"

// Locker is the concurrency-fixture seam for lock acquisition and release.
type Locker interface {
	Acquire(ctx context.Context) error
	Release()
}
