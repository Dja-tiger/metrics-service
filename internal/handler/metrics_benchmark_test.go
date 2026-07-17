package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/go-chi/chi/v5"

	models "github.com/Dja-tiger/metrics-service/internal/model"
	"github.com/Dja-tiger/metrics-service/internal/repository"
	"github.com/Dja-tiger/metrics-service/internal/service"
)

func BenchmarkUpdateMetricsJSON(b *testing.B) {
	payload, err := json.Marshal(benchmarkMetricsPayload(128))
	if err != nil {
		b.Fatalf("marshal payload: %v", err)
	}

	storage := repository.NewMemStorage()
	svc := service.NewMetricsService(storage)
	h := NewMetricsHandler(svc)

	router := chi.NewRouter()
	router.Post("/updates/", h.UpdateMetricsJSON)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/updates/", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			b.Fatalf("unexpected status: %d", rec.Code)
		}
	}
}

func benchmarkMetricsPayload(count int) []models.Metrics {
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
