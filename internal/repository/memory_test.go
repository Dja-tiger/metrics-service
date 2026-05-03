package repository

import (
	"path/filepath"
	"testing"
)

func TestMemStorageSaveAndLoadFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metrics.json")

	storage := NewMemStorage()
	storage.UpdateGauge("Alloc", 12.34)
	storage.UpdateCounter("PollCount", 5)

	if err := storage.SaveToFile(path); err != nil {
		t.Fatalf("save metrics: %v", err)
	}

	restored, err := NewMemStorageWithRestore(path, true)
	if err != nil {
		t.Fatalf("load metrics: %v", err)
	}

	gauge, ok := restored.GetGauge("Alloc")
	if !ok || gauge != 12.34 {
		t.Fatalf("unexpected gauge value: got %v exists %t", gauge, ok)
	}

	counter, ok := restored.GetCounter("PollCount")
	if !ok || counter != 5 {
		t.Fatalf("unexpected counter value: got %v exists %t", counter, ok)
	}
}

func TestMemStorageLoadMissingFile(t *testing.T) {
	_, err := NewMemStorageWithRestore(filepath.Join(t.TempDir(), "missing.json"), true)
	if err != nil {
		t.Fatalf("missing file should not fail: %v", err)
	}
}
