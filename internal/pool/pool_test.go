package pool_test

import (
	"fmt"
	"sync"
	"testing"

	models "github.com/Dja-tiger/metrics-service/internal/model"
	"github.com/Dja-tiger/metrics-service/internal/pool"
)

type buffer struct {
	data   []byte
	resets int
}

func (b *buffer) Reset() {
	b.data = b.data[:0]
	b.resets++
}

func TestGetCreatesObjects(t *testing.T) {
	calls := 0
	p := pool.New(func() *buffer {
		calls++
		return &buffer{data: make([]byte, 0, 64)}
	})
	first, second := p.Get(), p.Get()
	if calls != 2 || first == nil || second == nil || first == second {
		t.Fatalf("expected two independently allocated objects, factory calls: %d", calls)
	}
	if len(first.data) != 0 || cap(first.data) != 64 {
		t.Fatal("factory state was not preserved")
	}
}

func TestPutResetsBeforePublishing(t *testing.T) {
	var data []byte
	resets := 0
	p := pool.New(func() *observedReset {
		return &observedReset{reset: func() {
			data = data[:0]
			resets++
		}}
	})
	object := p.Get()
	data = append(data, 1, 2, 3)
	capacity := cap(data)
	p.Put(object)
	// Observe the callback rather than accessing the object after Put.
	if resets != 1 || len(data) != 0 || cap(data) != capacity {
		t.Fatal("Put must synchronously reset the object")
	}
}

type observedReset struct{ reset func() }

func (o *observedReset) Reset() { o.reset() }

func TestConcurrentReuse(t *testing.T) {
	p := pool.New(func() *buffer { return &buffer{data: make([]byte, 0, 64)} })
	var workers sync.WaitGroup
	for range 16 {
		workers.Go(func() {
			for range 100 {
				object := p.Get()
				if len(object.data) != 0 {
					t.Error("Get returned dirty state")
				}
				object.data = append(object.data, 1, 2, 3)
				p.Put(object)
			}
		})
	}
	workers.Wait()
}

func TestGeneratedReset(t *testing.T) {
	p := pool.New(func() *models.Metrics { return &models.Metrics{} })
	metric := p.Get()
	delta, value := int64(42), 3.5
	metric.ID, metric.MType, metric.Hash = "Alloc", models.Gauge, "hash"
	metric.Delta, metric.Value = &delta, &value
	p.Put(metric)
	if delta != 0 || value != 0 {
		t.Fatal("generated Reset did not clear pointed-to values")
	}
	// sync.Pool may discard objects, so identity/reuse is not guaranteed.
	clean := p.Get()
	if clean.ID != "" || clean.MType != "" || clean.Hash != "" {
		t.Fatal("Get returned an uncleared metric")
	}
	p.Put(clean)
}

func ExampleNew() {
	p := pool.New(func() *models.Metrics { return &models.Metrics{} })
	metric := p.Get()
	metric.ID = "Alloc"
	p.Put(metric)

	clean := p.Get()
	fmt.Printf("metric name: %q\n", clean.ID)
	p.Put(clean)
	// Output: metric name: ""
}

// A value type verifies that a factory-free generic pool returns T's zero,
// not only nil for pointer types.
type resetValue struct{ Count int }

func (resetValue) Reset() {}

func TestNilFactory(t *testing.T) {
	if got := pool.New[*buffer](nil).Get(); got != nil {
		t.Errorf("empty pointer pool: got %v, want nil", got)
	}
	if got := pool.New[resetValue](nil).Get(); got != (resetValue{}) {
		t.Errorf("empty value pool: got %+v, want zero", got)
	}
	if got := pool.New[interface{ Reset() }](nil).Get(); got != nil {
		t.Errorf("empty interface pool: got %v, want nil", got)
	}
}

func TestZeroValuePool(t *testing.T) {
	var p pool.Pool[*buffer]
	if got := p.Get(); got != nil {
		t.Errorf("zero-value pool: got %v, want nil", got)
	}
}

func TestFactoryReturningNilInterface(t *testing.T) {
	p := pool.New(func() interface{ Reset() } { return nil })
	if got := p.Get(); got != nil {
		t.Errorf("nil factory result: got %v, want nil", got)
	}
}

func TestNilFactoryPut(t *testing.T) {
	p := pool.New[*buffer](nil)
	p.Put(&buffer{data: []byte{1, 2}})
	// The runtime can discard pooled objects, so nil is also a valid result.
	if got := p.Get(); got != nil && (len(got.data) != 0 || got.resets != 1) {
		t.Errorf("object was not reset: %+v", got)
	}
}
