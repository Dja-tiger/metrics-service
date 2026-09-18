package service

import (
	"fmt"
	"github.com/Dja-tiger/metrics-service/internal/delivery"
	"time"

	models "github.com/Dja-tiger/metrics-service/internal/model"
	"github.com/Dja-tiger/metrics-service/internal/repository"
)

// MetricsService contains metric business logic.
type MetricsService struct {
	repo         repository.MetricsRepository
	saveOnUpdate func() error
	persistence  *persistence
}

// Close stops background persistence and saves the final snapshot once.
// Call it after all handlers and other metric writers have finished.
func (s *MetricsService) Close() error {
	if s.persistence == nil {
		return nil
	}
	return s.persistence.close()
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
func (s *MetricsService) SetSaveOnUpdate(save func() error) {
	s.saveOnUpdate = save
}

// UpdateGauge stores the latest gauge value.
func (s *MetricsService) UpdateGauge(name string, value float64) error {
	if err := s.repo.UpdateGauge(name, value); err != nil {
		return err
	}
	return s.save()
}

// UpdateCounter increments a counter value.
func (s *MetricsService) UpdateCounter(name string, value int64) error {
	if err := s.repo.UpdateCounter(name, value); err != nil {
		return err
	}
	return s.save()
}

// UpdateMetrics applies several metric updates and triggers persistence once.
func (s *MetricsService) UpdateMetrics(metrics []models.Metrics) error {
	if batchRepo, ok := s.repo.(repository.MetricsBatchRepository); ok {
		if err := batchRepo.UpdateMetrics(metrics); err != nil {
			return err
		}
		return s.save()
	}

	for _, metric := range metrics {
		switch metric.MType {
		case models.Gauge:
			if metric.Value != nil {
				if err := s.repo.UpdateGauge(metric.ID, *metric.Value); err != nil {
					return err
				}
			}
		case models.Counter:
			if metric.Delta != nil {
				if err := s.repo.UpdateCounter(metric.ID, *metric.Delta); err != nil {
					return err
				}
			}
		}
	}
	return s.save()
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

func (s *MetricsService) save() error {
	if s.saveOnUpdate != nil {
		return s.saveOnUpdate()
	}
	return nil
}

// UpdateMetricsOnce deduplicates a keyed batch; empty keys preserve legacy behavior.
func (s *MetricsService) UpdateMetricsOnce(key string, metrics []models.Metrics) (bool, error) {
	if len(metrics) == 0 {
		return false, nil
	}
	if key == "" {
		return true, s.UpdateMetrics(metrics)
	}
	if !delivery.ValidKey(key) {
		return false, fmt.Errorf("invalid idempotency key")
	}
	repo, ok := s.repo.(repository.IdempotentRepository)
	if !ok {
		return false, fmt.Errorf("storage does not support idempotent batches")
	}
	fingerprint, err := delivery.Fingerprint(metrics)
	if err != nil {
		return false, fmt.Errorf("fingerprint metrics: %w", err)
	}
	applied, err := repo.UpdateMetricsOnce(key, fingerprint, metrics)
	if err != nil {
		return false, err
	}
	// Repeat persistence even for a duplicate: an earlier synchronous save may have failed.
	return applied, s.save()
}
