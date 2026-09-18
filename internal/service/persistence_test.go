package service

import (
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Dja-tiger/metrics-service/internal/repository"
)

func TestCloseSavesFinalMetrics(t *testing.T) {
	for _, interval := range []time.Duration{0, time.Hour} {
		t.Run(interval.String(), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "metrics.json")
			store := repository.NewMemStorage()
			svc := NewMetricsServiceWithPersistence(store, store, path, interval, nil)
			svc.UpdateGauge("Alloc", 12.5)
			svc.UpdateCounter("PollCount", 7)
			if err := svc.Close(); err != nil {
				t.Fatal(err)
			}
			if err := svc.Close(); err != nil {
				t.Fatal(err)
			}
			restored, err := repository.NewMemStorageWithRestore(path, true)
			if err != nil {
				t.Fatal(err)
			}
			if got, ok := restored.GetGauge("Alloc"); !ok || got != 12.5 {
				t.Fatal("missing gauge")
			}
			if got, ok := restored.GetCounter("PollCount"); !ok || got != 7 {
				t.Fatal("missing counter")
			}
		})
	}
}

type blockingStorage struct {
	started, release chan struct{}
	calls            int
	err              error
}

func (s *blockingStorage) SaveToFile(string) error {
	s.calls++
	if s.calls == 1 {
		close(s.started)
		<-s.release
	}
	return s.err
}

func TestCloseWaitsForPeriodicSave(t *testing.T) {
	storage := &blockingStorage{started: make(chan struct{}), release: make(chan struct{})}
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(storage.release) }) })
	svc := NewMetricsServiceWithPersistence(repository.NewMemStorage(), storage, "unused", time.Millisecond, nil)
	select {
	case <-storage.started:
	case <-time.After(time.Second):
		t.Fatal("periodic save not started")
	}
	done := make(chan error, 1)
	go func() { done <- svc.Close() }()
	select {
	case <-done:
		t.Fatal("Close skipped ongoing save")
	case <-time.After(20 * time.Millisecond):
	}
	once.Do(func() { close(storage.release) })
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("Close stuck")
	}
	calls := storage.calls
	if calls < 2 {
		t.Fatal("no final save")
	}
	if err := svc.Close(); err != nil {
		t.Fatal(err)
	}
	if storage.calls != calls {
		t.Fatal("Close saved twice")
	}
}

func TestCloseReturnsSaveError(t *testing.T) {
	want := errors.New("disk unavailable")
	release := make(chan struct{})
	close(release)
	storage := &blockingStorage{started: make(chan struct{}), release: release, err: want}
	svc := NewMetricsServiceWithPersistence(repository.NewMemStorage(), storage, "unused", time.Hour, nil)
	if err := svc.Close(); !errors.Is(err, want) {
		t.Fatalf("got %v", err)
	}
}
