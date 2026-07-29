// Package fsutil provides general-purpose filesystem primitives shared across
// the repository. Helpers in this package return neutral errors (plain
// fmt.Errorf wraps); callers are responsible for mapping those errors onto the
// durability sentinels in internal/errors (e.g. blerrors.ErrWriteNotApplied).
// This neutral convention mirrors the append primitive in
// internal/events/fsutil.go (syncAppendResult), where the caller maps onto
// blerrors, and is the inverse of internal/atomicfile, which self-classifies.
// The neutral convention avoids importing blerrors and keeps this package a
// pure stdlib leaf.
//
// # Leaf package
//
// fsutil imports only the standard library (fmt, os, path/filepath). It has no
// internal imports and may be consumed by any internal package without
// introducing import cycles.
package fsutil
