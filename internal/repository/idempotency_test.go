package repository

import (
	"errors"
	"path/filepath"
	"sync"
	"testing"

	"github.com/Dja-tiger/metrics-service/internal/delivery"
	models "github.com/Dja-tiger/metrics-service/internal/model"
)

func checkDeduplication(t *testing.T, store IdempotentRepository, get func(string) (int64, bool)) {
	t.Helper()
	delta := int64(5)
	batch := []models.Metrics{{ID: "DedupCounter", MType: models.Counter, Delta: &delta}}
	var wg sync.WaitGroup
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := store.UpdateMetricsOnce("batch-one", "payload-one", batch); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if got, _ := get("DedupCounter"); got != 5 {
		t.Fatalf("concurrent duplicate count=%d", got)
	}
	if _, err := store.UpdateMetricsOnce("batch-one", "different-payload", batch); !errors.Is(err, delivery.ErrConflict) {
		t.Fatalf("collision: %v", err)
	}
	if applied, err := store.UpdateMetricsOnce("batch-two", "payload-one", batch); !applied || err != nil {
		t.Fatalf("new batch %v %v", applied, err)
	}
	if got, _ := get("DedupCounter"); got != 10 {
		t.Fatalf("independent identical batch lost: %d", got)
	}
}

func TestMemoryDeduplication(t *testing.T) {
	store := NewMemStorage()
	checkDeduplication(t, store, store.GetCounter)
}

func TestFileRestoresReceiptsWithCounters(t *testing.T) {
	store := NewMemStorage()
	checkDeduplication(t, store, store.GetCounter)
	path := filepath.Join(t.TempDir(), "metrics.json")
	if err := store.SaveToFile(path); err != nil {
		t.Fatal(err)
	}
	restored, err := NewMemStorageWithRestore(path, true)
	if err != nil {
		t.Fatal(err)
	}
	delta := int64(5)
	applied, err := restored.UpdateMetricsOnce("batch-one", "payload-one", []models.Metrics{{ID: "DedupCounter", MType: models.Counter, Delta: &delta}})
	if err != nil || applied {
		t.Fatalf("restored receipt missing: %v %v", applied, err)
	}
	if got, _ := restored.GetCounter("DedupCounter"); got != 10 {
		t.Fatalf("restored duplicate changed count: %d", got)
	}
}
