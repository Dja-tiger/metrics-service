package service

import (
	"os"
	"path/filepath"
	"testing"

	models "github.com/Dja-tiger/metrics-service/internal/model"
	"github.com/Dja-tiger/metrics-service/internal/repository"
)

func TestRetryAfterFileSaveFailureDoesNotAddCounterAgain(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(parent, []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(parent, "metrics.json")
	store := repository.NewMemStorage()
	svc := NewMetricsServiceWithPersistence(store, store, path, 0, nil)
	defer svc.Close()
	delta := int64(5)
	batch := []models.Metrics{{ID: "PollCount", MType: models.Counter, Delta: &delta}}
	if _, err := svc.UpdateMetricsOnce("request-one", batch); err == nil {
		t.Fatal("expected save failure")
	}
	if err := os.Remove(parent); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(parent, 0700); err != nil {
		t.Fatal(err)
	}
	if applied, err := svc.UpdateMetricsOnce("request-one", batch); applied || err != nil {
		t.Fatalf("retry: %v %v", applied, err)
	}
	loaded, err := repository.NewMemStorageWithRestore(path, true)
	if err != nil {
		t.Fatal(err)
	}
	restarted := NewMetricsService(loaded)
	if applied, err := restarted.UpdateMetricsOnce("request-one", batch); applied || err != nil {
		t.Fatalf("restart: %v %v", applied, err)
	}
	if value, _ := loaded.GetCounter("PollCount"); value != 5 {
		t.Fatalf("counter duplicated: %d", value)
	}
}
