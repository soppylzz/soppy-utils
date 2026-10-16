// Package registry holds what core can do, and how usable each thing is against a
// given session environment.
//
// It is the single source of truth for the command surface as well as for what exists:
// help text, usage errors, and the list a client renders all come from here rather
// than from a table written out a second time. The dispatcher's case list, the help
// text, and the helper's command switch are exactly the parallel tables this replaces.
//
// Nothing here decides what is permitted. A capability reports what the environment
// supports; whether an operation may proceed is a separate question, and the operation
// itself is what has the final say.
package registry

import (
	"fmt"
	"sync"
)

// Registry is a set of capabilities.
//
// It is an ordinary value rather than a package-level one, so a test can build the
// registry it needs. Capability evaluation is precisely what tests have to drive with
// a controlled environment, and a global registry makes them order-dependent and
// impossible to isolate.
type Registry struct {
	mu    sync.Mutex
	items []Capability
	byID  map[string]int
}

func New() *Registry {
	return &Registry{byID: make(map[string]int)}
}

// Register adds a capability.
//
// A duplicate id is an error rather than a replacement. Two packages claiming one name
// means one of them is silently unreachable, and which one won would depend on the
// order the assembly happened to use.
func (r *Registry) Register(c Capability) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.byID[c.ID]; exists {
		return fmt.Errorf("capability %q is already registered", c.ID)
	}
	r.byID[c.ID] = len(r.items)
	r.items = append(r.items, c)
	return nil
}

// List returns the capabilities in registration order.
func (r *Registry) List() []Capability {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Capability, len(r.items))
	copy(out, r.items)
	return out
}

// Lookup finds a capability by id.
func (r *Registry) Lookup(id string) (Capability, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	i, ok := r.byID[id]
	if !ok {
		return Capability{}, false
	}
	return r.items[i], true
}
