package mutation

import (
	"errors"
	"fmt"
	"sort"
	"sync"
)

// ErrDuplicateOp is returned by Register when the op name is already
// registered.
var ErrDuplicateOp = errors.New("mutation: op already registered")

// ErrOpNotRegistered is returned by Lookup-dependent functions when the
// requested op name has not been registered.
var ErrOpNotRegistered = errors.New("mutation: op not registered")

var (
	mu  sync.RWMutex
	reg = map[string]RepresentationSet{}
)

// Register binds a RepresentationSet to its op name in the package-level
// registry. It returns an error if the set fails schema validation or if the
// op name has already been registered. Register is safe for concurrent use.
func Register(set RepresentationSet) error {
	if err := set.Validate(); err != nil {
		return fmt.Errorf("mutation.Register: %w", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if _, exists := reg[set.Op]; exists {
		return fmt.Errorf("mutation.Register %q: %w", set.Op, ErrDuplicateOp)
	}
	reg[set.Op] = set
	return nil
}

// Lookup returns the RepresentationSet for the given op name. The second
// return value is false when the op has not been registered. The returned
// RepresentationSet's Representations slice is a defensive copy; callers must
// not mutate it. Lookup is safe for concurrent use.
func Lookup(opName string) (RepresentationSet, bool) {
	mu.RLock()
	defer mu.RUnlock()
	set, ok := reg[opName]
	if !ok {
		return RepresentationSet{}, false
	}
	// Defensive copy: prevent a caller mutation from corrupting the stored entry.
	cp := make([]RepresentationKind, len(set.Representations))
	copy(cp, set.Representations)
	set.Representations = cp
	return set, true
}

// Registered returns the names of all registered ops in sorted order.
// Registered is safe for concurrent use.
func Registered() []string {
	mu.RLock()
	defer mu.RUnlock()
	names := make([]string, 0, len(reg))
	for name := range reg {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
