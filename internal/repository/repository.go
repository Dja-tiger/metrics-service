package repository

import models "github.com/Dja-tiger/metrics-service/internal/model"

// MetricsRepository abstracts metric storage.
type MetricsRepository interface {
	UpdateGauge(name string, value float64)
	UpdateCounter(name string, value int64)
	GetGauge(name string) (float64, bool)
	GetCounter(name string) (int64, bool)
	GetAllGauges() map[string]float64
	GetAllCounters() map[string]int64
}

// MetricsBatchRepository can update multiple metrics in one storage-specific operation.
type MetricsBatchRepository interface {
	UpdateMetrics(metrics []models.Metrics)
}
