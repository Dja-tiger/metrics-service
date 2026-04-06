package repository

// MetricsRepository abstracts metric storage.
type MetricsRepository interface {
	UpdateGauge(name string, value float64)
	UpdateCounter(name string, value int64)
}
