package service

import (
	"time"

	models "github.com/Dja-tiger/metrics-service/internal/model"
	"github.com/Dja-tiger/metrics-service/internal/repository"
)

// MetricsService contains metric business logic.
type MetricsService struct {
	repo         repository.MetricsRepository
	saveOnUpdate func()
}

// NewMetricsService creates a metric service backed by a repository.
func NewMetricsService(repo repository.MetricsRepository) *MetricsService {
	return &MetricsService{repo: repo}
}

// NewMetricsServiceWithPersistence creates a metric service with periodic or synchronous persistence.
func NewMetricsServiceWithPersistence(repo repository.MetricsRepository, storage PersistentStorage, path string, interval time.Duration, handleError func(error)) *MetricsService {
	service := NewMetricsService(repo)
	configurePersistence(service, storage, path, interval, handleError)
	return service
}

// SetSaveOnUpdate configures a callback called after metric updates.
func (s *MetricsService) SetSaveOnUpdate(save func()) {
	s.saveOnUpdate = save
}

// UpdateGauge stores the latest gauge value.
func (s *MetricsService) UpdateGauge(name string, value float64) {
	s.repo.UpdateGauge(name, value)
	s.save()
}

// UpdateCounter increments a counter value.
func (s *MetricsService) UpdateCounter(name string, value int64) {
	s.repo.UpdateCounter(name, value)
	s.save()
}

// UpdateMetrics applies several metric updates and triggers persistence once.
func (s *MetricsService) UpdateMetrics(metrics []models.Metrics) {
	if batchRepo, ok := s.repo.(repository.MetricsBatchRepository); ok {
		batchRepo.UpdateMetrics(metrics)
		s.save()
		return
	}

	for _, metric := range metrics {
		switch metric.MType {
		case models.Gauge:
			if metric.Value != nil {
				s.repo.UpdateGauge(metric.ID, *metric.Value)
			}
		case models.Counter:
			if metric.Delta != nil {
				s.repo.UpdateCounter(metric.ID, *metric.Delta)
			}
		}
	}
	s.save()
}

// GetGauge returns a gauge value by name.
func (s *MetricsService) GetGauge(name string) (float64, bool) {
	return s.repo.GetGauge(name)
}

// GetCounter returns a counter value by name.
func (s *MetricsService) GetCounter(name string) (int64, bool) {
	return s.repo.GetCounter(name)
}

// GetAllGauges returns all known gauge metrics.
func (s *MetricsService) GetAllGauges() map[string]float64 {
	return s.repo.GetAllGauges()
}

// GetAllCounters returns all known counter metrics.
func (s *MetricsService) GetAllCounters() map[string]int64 {
	return s.repo.GetAllCounters()
}

func (s *MetricsService) save() {
	if s.saveOnUpdate != nil {
		s.saveOnUpdate()
	}
}
