package audit

import (
	"context"
	"path/filepath"
	"testing"
)

func BenchmarkFileObserverNotify(b *testing.B) {
	observer := NewFileObserver(filepath.Join(b.TempDir(), "audit.log"))
	event := Event{
		Timestamp: 12345678,
		Metrics:   []string{"Alloc", "Frees", "HeapAlloc", "TotalMemory"},
		IPAddress: "192.168.0.42",
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := observer.Notify(context.Background(), event); err != nil {
			b.Fatalf("notify observer: %v", err)
		}
	}
}

func BenchmarkNotifierNotify(b *testing.B) {
	notifier := NewNotifier(noopObserver{}, noopObserver{})
	event := Event{
		Timestamp: 12345678,
		Metrics:   []string{"Alloc", "Frees", "HeapAlloc", "TotalMemory"},
		IPAddress: "192.168.0.42",
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := notifier.Notify(context.Background(), event); err != nil {
			b.Fatalf("notify observers: %v", err)
		}
	}
}

type noopObserver struct{}

func (noopObserver) Notify(ctx context.Context, event Event) error {
	return nil
}
