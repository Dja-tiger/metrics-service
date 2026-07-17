package repository

import models "github.com/Dja-tiger/metrics-service/internal/model"

// MetricsRepository abstracts metric storage.
type MetricsRepository interface {
	// UpdateGauge stores the latest gauge value.
	UpdateGauge(name string, value float64)
	// UpdateCounter increments a counter value.
	UpdateCounter(name string, value int64)
	// GetGauge returns a gauge value by name.
	GetGauge(name string) (float64, bool)
	// GetCounter returns a counter value by name.
	GetCounter(name string) (int64, bool)
	// GetAllGauges returns all known gauge metrics.
	GetAllGauges() map[string]float64
	// GetAllCounters returns all known counter metrics.
	GetAllCounters() map[string]int64
}

// MetricsBatchRepository can update multiple metrics in one storage-specific operation.
type MetricsBatchRepository interface {
	// UpdateMetrics applies several metric updates at once.
	UpdateMetrics(metrics []models.Metrics)
}
