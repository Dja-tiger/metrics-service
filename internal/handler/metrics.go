package handler

import (
	"fmt"
	"html"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	models "github.com/Dja-tiger/metrics-service/internal/model"
)

// MetricsService describes metric operations required by handlers.
type MetricsService interface {
	UpdateGauge(name string, value float64)
	UpdateCounter(name string, value int64)
	GetGauge(name string) (float64, bool)
	GetCounter(name string) (int64, bool)
	GetAllGauges() map[string]float64
	GetAllCounters() map[string]int64
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

	metricType := chi.URLParam(r, "type")
	metricName := chi.URLParam(r, "name")
	metricValue := chi.URLParam(r, "value")

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

func (h *MetricsHandler) GetValue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	metricType := chi.URLParam(r, "type")
	metricName := chi.URLParam(r, "name")

	if metricName == "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	switch metricType {
	case models.Gauge:
		value, ok := h.service.GetGauge(metricName)
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		_, _ = fmt.Fprintf(w, "%s", strconv.FormatFloat(value, 'f', -1, 64))

	case models.Counter:
		value, ok := h.service.GetCounter(metricName)
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		_, _ = fmt.Fprintf(w, "%s", strconv.FormatInt(value, 10))

	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (h *MetricsHandler) ListMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprint(w, "<html><body><ul>")

	for name, value := range h.service.GetAllGauges() {
		_, _ = fmt.Fprintf(w, "<li>%s: %s</li>", html.EscapeString(name), strconv.FormatFloat(value, 'f', -1, 64))
	}
	for name, value := range h.service.GetAllCounters() {
		_, _ = fmt.Fprintf(w, "<li>%s: %s</li>", html.EscapeString(name), strconv.FormatInt(value, 10))
	}

	_, _ = fmt.Fprint(w, "</ul></body></html>")
}
