package repository

import (
	"path/filepath"
	"strconv"
	"testing"

	models "github.com/Dja-tiger/metrics-service/internal/model"
)

func BenchmarkMemStorageUpdateMetrics(b *testing.B) {
	metrics := benchmarkMetrics(128)
	storage := NewMemStorage()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		storage.UpdateMetrics(metrics)
	}
}

func BenchmarkMemStorageSaveToFile(b *testing.B) {
	storage := NewMemStorage()
	storage.UpdateMetrics(benchmarkMetrics(512))
	path := filepath.Join(b.TempDir(), "metrics.json")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := storage.SaveToFile(path); err != nil {
			b.Fatalf("save metrics: %v", err)
		}
	}
}

func benchmarkMetrics(count int) []models.Metrics {
	metrics := make([]models.Metrics, 0, count*2)
	for i := 0; i < count; i++ {
		gaugeValue := float64(i) * 1.25
		counterValue := int64(i)
		metrics = append(metrics, models.Metrics{
			ID:    "gauge_metric_" + strconv.Itoa(i),
			MType: models.Gauge,
			Value: &gaugeValue,
		})
		metrics = append(metrics, models.Metrics{
			ID:    "counter_metric_" + strconv.Itoa(i),
			MType: models.Counter,
			Delta: &counterValue,
		})
	}
	return metrics
}
