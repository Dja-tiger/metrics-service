package handler

import (
	"context"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	models "github.com/Dja-tiger/metrics-service/internal/model"
)

// MetricsService describes metric operations required by handlers.
type MetricsService interface {
	UpdateGauge(name string, value float64)
	UpdateCounter(name string, value int64)
	UpdateMetrics(metrics []models.Metrics)
	GetGauge(name string) (float64, bool)
	GetCounter(name string) (int64, bool)
	GetAllGauges() map[string]float64
	GetAllCounters() map[string]int64
}

type DatabasePinger interface {
	PingContext(ctx context.Context) error
}

// MetricsHandler handles HTTP requests for metrics.
type MetricsHandler struct {
	service MetricsService
	db      DatabasePinger
}

func NewMetricsHandler(service MetricsService) *MetricsHandler {
	return &MetricsHandler{service: service}
}

func NewMetricsHandlerWithDB(service MetricsService, db DatabasePinger) *MetricsHandler {
	return &MetricsHandler{
		service: service,
		db:      db,
	}
}

func (h *MetricsHandler) UpdateMetric(w http.ResponseWriter, r *http.Request) {
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

func (h *MetricsHandler) UpdateMetricJSON(w http.ResponseWriter, r *http.Request) {
	var metric models.Metrics
	if err := json.NewDecoder(r.Body).Decode(&metric); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if statusCode := validateUpdateMetric(metric); statusCode != http.StatusOK {
		w.WriteHeader(statusCode)
		return
	}

	h.service.UpdateMetrics([]models.Metrics{metric})

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(metric); err != nil {
		log.Printf("encode update response: %v", err)
	}
}

func (h *MetricsHandler) UpdateMetricsJSON(w http.ResponseWriter, r *http.Request) {
	var metrics []models.Metrics
	if err := json.NewDecoder(r.Body).Decode(&metrics); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	for _, metric := range metrics {
		if statusCode := validateUpdateMetric(metric); statusCode != http.StatusOK {
			w.WriteHeader(statusCode)
			return
		}
	}

	if len(metrics) > 0 {
		h.service.UpdateMetrics(metrics)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(metrics); err != nil {
		log.Printf("encode batch update response: %v", err)
	}
}

func (h *MetricsHandler) GetValue(w http.ResponseWriter, r *http.Request) {
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
		_, _ = w.Write([]byte(strconv.FormatFloat(value, 'f', -1, 64)))

	case models.Counter:
		value, ok := h.service.GetCounter(metricName)
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(strconv.FormatInt(value, 10)))

	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (h *MetricsHandler) GetValueJSON(w http.ResponseWriter, r *http.Request) {
	var requestMetric models.Metrics
	if err := json.NewDecoder(r.Body).Decode(&requestMetric); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if requestMetric.ID == "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	responseMetric := models.Metrics{
		ID:    requestMetric.ID,
		MType: requestMetric.MType,
	}

	switch requestMetric.MType {
	case models.Gauge:
		value, ok := h.service.GetGauge(requestMetric.ID)
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		responseMetric.Value = &value
	case models.Counter:
		value, ok := h.service.GetCounter(requestMetric.ID)
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		responseMetric.Delta = &value
	default:
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(responseMetric); err != nil {
		log.Printf("encode value response: %v", err)
	}
}

func (h *MetricsHandler) ListMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	data := struct {
		Gauges   map[string]float64
		Counters map[string]int64
	}{
		Gauges:   h.service.GetAllGauges(),
		Counters: h.service.GetAllCounters(),
	}

	if err := metricsTemplate.Execute(w, data); err != nil {
		log.Printf("render metrics page: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (h *MetricsHandler) Ping(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()

	if err := h.db.PingContext(ctx); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

var metricsTemplate = template.Must(template.New("metrics").Parse(`
<html>
  <body>
    <ul>
      {{- range $name, $value := .Gauges }}
        <li>{{ $name }}: {{ $value }}</li>
      {{- end }}
      {{- range $name, $value := .Counters }}
        <li>{{ $name }}: {{ $value }}</li>
      {{- end }}
    </ul>
  </body>
</html>
`))

func validateUpdateMetric(metric models.Metrics) int {
	if metric.ID == "" {
		return http.StatusNotFound
	}

	switch metric.MType {
	case models.Gauge:
		if metric.Value == nil {
			return http.StatusBadRequest
		}
	case models.Counter:
		if metric.Delta == nil {
			return http.StatusBadRequest
		}
	default:
		return http.StatusBadRequest
	}

	return http.StatusOK
}
