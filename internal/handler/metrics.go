package handler

import (
	"encoding/json"
	"html/template"
	"log"
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

func (h *MetricsHandler) UpdateMetricJSON(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var metric models.Metrics
	if err := json.NewDecoder(r.Body).Decode(&metric); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if metric.ID == "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	switch metric.MType {
	case models.Gauge:
		if metric.Value == nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		h.service.UpdateGauge(metric.ID, *metric.Value)
	case models.Counter:
		if metric.Delta == nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		h.service.UpdateCounter(metric.ID, *metric.Delta)
	default:
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(metric); err != nil {
		log.Printf("encode update response: %v", err)
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
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

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
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

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
