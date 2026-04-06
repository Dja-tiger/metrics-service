package handler

import (
	"net/http"
	"strconv"
	"strings"

	models "github.com/Dja-tiger/metrics-service/internal/model"
)

// MetricsService describes metric operations required by handlers.
type MetricsService interface {
	UpdateGauge(name string, value float64)
	UpdateCounter(name string, value int64)
}

// MetricsHandler handles HTTP requests for metrics.
type MetricsHandler struct {
	service MetricsService
}

func NewMetricsHandler(service MetricsService) *MetricsHandler {
	return &MetricsHandler{service: service}
}

func (h *MetricsHandler) UpdateMetric(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")

	// ожидаем путь вида:
	// /update/<type>/<name>/<value>
	if len(parts) != 4 {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if parts[0] != "update" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	metricType := parts[1]
	metricName := parts[2]
	metricValue := parts[3]

	if metricName == "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	switch metricType {
	case models.Gauge:
		value, err := strconv.ParseFloat(metricValue, 64)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		h.service.UpdateGauge(metricName, value)
		w.WriteHeader(http.StatusOK)

	case models.Counter:
		value, err := strconv.ParseInt(metricValue, 10, 64)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		h.service.UpdateCounter(metricName, value)
		w.WriteHeader(http.StatusOK)

	default:
		w.WriteHeader(http.StatusBadRequest)
	}
}
