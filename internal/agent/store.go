package agent

import (
	"sync"

	models "github.com/Dja-tiger/metrics-service/internal/model"
)

// Store keeps metrics collected by the agent.
type Store struct {
	mu       sync.Mutex
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
	s.mu.Lock()
	defer s.mu.Unlock()
	copyMap := make(map[string]float64, len(s.gauges))
	for k, v := range s.gauges {
		copyMap[k] = v
	}
	return copyMap
}

func (s *Store) SnapshotAndResetCounters() map[string]int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	copyMap := make(map[string]int64, len(s.counters))
	for k, v := range s.counters {
		copyMap[k] = v
	}
	for k := range s.counters {
		s.counters[k] = 0
	}
	return copyMap
}

func (s *Store) SnapshotMetrics() []models.Metrics {
	s.mu.Lock()
	defer s.mu.Unlock()

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
		delete(s.counters, name)
	}
	return metrics
}
