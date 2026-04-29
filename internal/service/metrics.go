package service

import "github.com/Dja-tiger/metrics-service/internal/repository"

// MetricsService contains metric business logic.
type MetricsService struct {
	repo         repository.MetricsRepository
	saveOnUpdate func()
}

func NewMetricsService(repo repository.MetricsRepository) *MetricsService {
	return &MetricsService{repo: repo}
}

func (s *MetricsService) SetSaveOnUpdate(save func()) {
	s.saveOnUpdate = save
}

func (s *MetricsService) UpdateGauge(name string, value float64) {
	s.repo.UpdateGauge(name, value)
	s.save()
}

func (s *MetricsService) UpdateCounter(name string, value int64) {
	s.repo.UpdateCounter(name, value)
	s.save()
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

func (s *MetricsService) save() {
	if s.saveOnUpdate != nil {
		s.saveOnUpdate()
	}
}
