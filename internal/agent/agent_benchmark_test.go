package agent

import (
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	models "github.com/Dja-tiger/metrics-service/internal/model"
)

func BenchmarkAgentSendMetrics(b *testing.B) {
	client := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     make(http.Header),
			}, nil
		}),
	}
	agent, err := NewAgent("http://localhost:8080", time.Second, time.Second, WithHTTPClient(client), WithKey("secret"))
	if err != nil {
		b.Fatalf("create agent: %v", err)
	}
	metrics := benchmarkAgentMetrics(128)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err = agent.sendMetrics(metrics); err != nil {
			b.Fatalf("send metrics: %v", err)
		}
	}
}

func BenchmarkStoreSnapshotMetrics(b *testing.B) {
	store := NewStore()
	for _, metric := range benchmarkAgentMetrics(128) {
		switch metric.MType {
		case models.Gauge:
			store.SetGauge(metric.ID, *metric.Value)
		case models.Counter:
			store.IncCounter(metric.ID, *metric.Delta)
		}
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		metrics := store.SnapshotMetrics()
		for _, metric := range metrics {
			if metric.Delta != nil {
				store.IncCounter(metric.ID, *metric.Delta)
			}
		}
	}
}

func benchmarkAgentMetrics(count int) []models.Metrics {
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
