// Package pool provides typed reuse of objects that can reset their state.
package pool

import "sync"

// Pool stores objects of one type and resets them before reuse.
// It is safe for concurrent use but must not be copied after first use.
// Objects may be removed by the garbage collector at any time.
// Create a Pool with New; its zero value is not ready for use.
type Pool[T interface{ Reset() }] struct {
	items sync.Pool
}

// New creates a pool whose empty Get calls use newObject to allocate an object.
// newObject must be non-nil, safe for concurrent calls, and return a new,
// non-nil object in its initial state. Normally T is a pointer to a struct.
func New[T interface{ Reset() }](newObject func() T) *Pool[T] {
	return &Pool[T]{
		items: sync.Pool{
			New: func() any { return newObject() },
		},
	}
}

// Get returns an available object, creating one when the pool is empty.
// The caller owns the object until it is returned with Put.
func (p *Pool[T]) Get() T {
	return p.items.Get().(T)
}

// Put resets an object and makes it available for reuse.
// The object must be non-nil and exclusively owned by the caller.
// After Put, the caller must not access it or return it a second time.
func (p *Pool[T]) Put(object T) {
	object.Reset()
	p.items.Put(object)
}
