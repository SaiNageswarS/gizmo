// core_registry.go
// Central registry that maps string aliases → Processor factories.
// Adapters register themselves via an init() side‑effect so that applications
// can simply import the adapter package for self‑registration.

package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
)

// ----------------------------------------------------------------------------
// Processor abstraction
// ----------------------------------------------------------------------------

// Processor is the minimal contract every adapter must fulfil.
// It performs a media transformation (PDF → text, video → MP4 …) and streams
// from an io.Reader to an io.Writer. All calls must be thread‑safe; adapters
// are expected to be stateless.

type Processor interface {
	Do(ctx context.Context, in io.Reader, out io.Writer, opts ...Option) error
}

// ----------------------------------------------------------------------------
// Registry implementation
// ----------------------------------------------------------------------------

var (
	mu       sync.RWMutex
	registry = make(map[string]func() Processor)
)

// ErrNotFound is returned when a Processor alias is unknown.
var ErrNotFound = errors.New("gizmo: processor not found")

// Register installs a factory under the given alias. It panics on duplicates.
// Adapters should call Register from an init() function:
//
//	func init() { core.Register("mupdf-text", NewTextExtractor) }
func Register(alias string, factory func() Processor) {
	mu.Lock()
	defer mu.Unlock()
	if alias == "" {
		panic("gizmo: empty alias in Register")
	}
	if factory == nil {
		panic(fmt.Sprintf("gizmo: nil factory for alias %q", alias))
	}
	if _, dup := registry[alias]; dup {
		panic(fmt.Sprintf("gizmo: duplicate registration for alias %q", alias))
	}
	registry[alias] = factory
}

// Get returns a fresh Processor from the registry or an error if missing.
func Get(alias string) (Processor, error) {
	mu.RLock()
	factory, ok := registry[alias]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, alias)
	}
	return factory(), nil
}

// Must is like Get but panics when the alias is missing – convenient for
// package‑level vars in apps that expect the registration to succeed.
func Must(alias string) Processor {
	p, err := Get(alias)
	if err != nil {
		panic(err)
	}
	return p
}
