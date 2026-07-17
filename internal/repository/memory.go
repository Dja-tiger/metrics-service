package repository

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	models "github.com/Dja-tiger/metrics-service/internal/model"
)

// MemStorage stores metrics in memory.
type MemStorage struct {
	mu       sync.RWMutex
	gauges   map[string]float64
	counters map[string]int64
}

// NewMemStorage creates an empty in-memory metric storage.
func NewMemStorage() *MemStorage {
	return &MemStorage{
		gauges:   make(map[string]float64),
		counters: make(map[string]int64),
	}
}

// NewMemStorageWithRestore creates in-memory storage and optionally restores metrics from a file.
func NewMemStorageWithRestore(path string, restore bool) (*MemStorage, error) {
	storage := NewMemStorage()
	if !restore {
		return storage, nil
	}
	if err := storage.loadFromFile(path); err != nil {
		return nil, err
	}
	return storage, nil
}

// UpdateGauge stores the latest gauge value.
func (s *MemStorage) UpdateGauge(name string, value float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gauges[name] = value
}

// UpdateCounter increments a counter value.
func (s *MemStorage) UpdateCounter(name string, value int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counters[name] += value
}

// UpdateMetrics applies a batch of gauge and counter updates.
func (s *MemStorage) UpdateMetrics(metrics []models.Metrics) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, metric := range metrics {
		switch metric.MType {
		case models.Gauge:
			if metric.Value != nil {
				s.gauges[metric.ID] = *metric.Value
			}
		case models.Counter:
			if metric.Delta != nil {
				s.counters[metric.ID] += *metric.Delta
			}
		}
	}
}

// GetGauge returns a gauge value by name.
func (s *MemStorage) GetGauge(name string) (float64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.gauges[name]
	return value, ok
}

// GetCounter returns a counter value by name.
func (s *MemStorage) GetCounter(name string) (int64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.counters[name]
	return value, ok
}

// GetAllGauges returns a copy of all gauge metrics.
func (s *MemStorage) GetAllGauges() map[string]float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	copyMap := make(map[string]float64, len(s.gauges))
	for k, v := range s.gauges {
		copyMap[k] = v
	}
	return copyMap
}

// GetAllCounters returns a copy of all counter metrics.
func (s *MemStorage) GetAllCounters() map[string]int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	copyMap := make(map[string]int64, len(s.counters))
	for k, v := range s.counters {
		copyMap[k] = v
	}
	return copyMap
}

// SaveToFile atomically writes all metrics to a JSON file.
func (s *MemStorage) SaveToFile(path string) error {
	dir := filepath.Dir(path)
	if dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("failed to create storage directory: %w", err)
		}
	}

	tempFile, err := os.CreateTemp(dir, "metrics-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tempName := tempFile.Name()
	defer os.Remove(tempName)

	metrics := s.snapshotMetrics()
	if err = json.NewEncoder(tempFile).Encode(metrics); err != nil {
		if closeErr := tempFile.Close(); closeErr != nil {
			return fmt.Errorf("failed to encode and close temp file: %w", errors.Join(err, closeErr))
		}
		return fmt.Errorf("failed to encode metrics: %w", err)
	}
	if err = tempFile.Chmod(0o644); err != nil {
		if closeErr := tempFile.Close(); closeErr != nil {
			return fmt.Errorf("failed to chmod and close temp file: %w", errors.Join(err, closeErr))
		}
		return fmt.Errorf("failed to chmod temp file: %w", err)
	}
	if err = tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close the temp file: %w", err)
	}

	if err = os.Rename(tempName, path); err != nil {
		return fmt.Errorf("failed to replace storage file: %w", err)
	}
	return nil
}

func (s *MemStorage) loadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("failed to read storage file: %w", err)
	}

	var metrics []models.Metrics
	if err = json.Unmarshal(data, &metrics); err != nil {
		return fmt.Errorf("failed to unmarshal metrics: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.gauges = make(map[string]float64)
	s.counters = make(map[string]int64)
	for _, metric := range metrics {
		switch metric.MType {
		case models.Gauge:
			if metric.Value != nil {
				s.gauges[metric.ID] = *metric.Value
			}
		case models.Counter:
			if metric.Delta != nil {
				s.counters[metric.ID] = *metric.Delta
			}
		}
	}

	return nil
}

func (s *MemStorage) snapshotMetrics() []models.Metrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	metrics := make([]models.Metrics, 0, len(s.gauges)+len(s.counters))
	gaugeValues := make([]float64, len(s.gauges))
	gaugeIndex := 0
	for name, value := range s.gauges {
		gaugeValues[gaugeIndex] = value
		metrics = append(metrics, models.Metrics{
			ID:    name,
			MType: models.Gauge,
			Value: &gaugeValues[gaugeIndex],
		})
		gaugeIndex++
	}

	counterValues := make([]int64, len(s.counters))
	counterIndex := 0
	for name, value := range s.counters {
		counterValues[counterIndex] = value
		metrics = append(metrics, models.Metrics{
			ID:    name,
			MType: models.Counter,
			Delta: &counterValues[counterIndex],
		})
		counterIndex++
	}

	return metrics
}
