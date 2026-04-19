package repository

import "sync"

// MemStorage stores metrics in memory.
type MemStorage struct {
	mu       sync.RWMutex
	gauges   map[string]float64
	counters map[string]int64
}

func NewMemStorage() *MemStorage {
	return &MemStorage{
		gauges:   make(map[string]float64),
		counters: make(map[string]int64),
	}
}

func (s *MemStorage) UpdateGauge(name string, value float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gauges[name] = value
}

func (s *MemStorage) UpdateCounter(name string, value int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counters[name] += value
}

func (s *MemStorage) GetGauge(name string) (float64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.gauges[name]
	return value, ok
}

func (s *MemStorage) GetCounter(name string) (int64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.counters[name]
	return value, ok
}

func (s *MemStorage) GetAllGauges() map[string]float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	copyMap := make(map[string]float64, len(s.gauges))
	for k, v := range s.gauges {
		copyMap[k] = v
	}
	return copyMap
}

func (s *MemStorage) GetAllCounters() map[string]int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	copyMap := make(map[string]int64, len(s.counters))
	for k, v := range s.counters {
		copyMap[k] = v
	}
	return copyMap
}
