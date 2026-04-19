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

func (s *MetricsService) GetGauge(name string) (float64, bool) {
	return s.repo.GetGauge(name)
}

func (s *MetricsService) GetCounter(name string) (int64, bool) {
	return s.repo.GetCounter(name)
}

func (s *MetricsService) GetAllGauges() map[string]float64 {
	return s.repo.GetAllGauges()
}

func (s *MetricsService) GetAllCounters() map[string]int64 {
	return s.repo.GetAllCounters()
}
