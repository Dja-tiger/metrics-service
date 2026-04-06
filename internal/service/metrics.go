package service

import "github.com/Dja-tiger/metrics-service/internal/repository"

// MetricsService contains metric business logic.
type MetricsService struct {
	repo repository.MetricsRepository
}

func NewMetricsService(repo repository.MetricsRepository) *MetricsService {
	return &MetricsService{repo: repo}
}

func (s *MetricsService) UpdateGauge(name string, value float64) {
	s.repo.UpdateGauge(name, value)
}

func (s *MetricsService) UpdateCounter(name string, value int64) {
	s.repo.UpdateCounter(name, value)
}
