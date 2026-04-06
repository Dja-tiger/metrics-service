package agent

import "sync"

// Store keeps metrics collected by the agent.
type Store struct {
	mu       sync.RWMutex
	gauges   map[string]float64
	counters map[string]int64
}

func NewStore() *Store {
	return &Store{
		gauges:   make(map[string]float64),
		counters: make(map[string]int64),
	}
}

func (s *Store) SetGauge(name string, value float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gauges[name] = value
}

func (s *Store) IncCounter(name string, delta int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counters[name] += delta
}

func (s *Store) SnapshotGauges() map[string]float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	copyMap := make(map[string]float64, len(s.gauges))
	for k, v := range s.gauges {
		copyMap[k] = v
	}
	return copyMap
}

func (s *Store) SnapshotCounters() map[string]int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	copyMap := make(map[string]int64, len(s.counters))
	for k, v := range s.counters {
		copyMap[k] = v
	}
	return copyMap
}
