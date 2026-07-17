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

	"github.com/Dja-tiger/metrics-service/internal/audit"
	models "github.com/Dja-tiger/metrics-service/internal/model"
)

// MetricsService describes metric operations required by handlers.
type MetricsService interface {
	// UpdateGauge stores the latest gauge value.
	UpdateGauge(name string, value float64)
	// UpdateCounter increments a counter value.
	UpdateCounter(name string, value int64)
	// UpdateMetrics applies several metric updates at once.
	UpdateMetrics(metrics []models.Metrics)
	// GetGauge returns a gauge value by name.
	GetGauge(name string) (float64, bool)
	// GetCounter returns a counter value by name.
	GetCounter(name string) (int64, bool)
	// GetAllGauges returns all known gauge metrics.
	GetAllGauges() map[string]float64
	// GetAllCounters returns all known counter metrics.
	GetAllCounters() map[string]int64
}

// DatabasePinger describes database health-check behavior required by Ping.
type DatabasePinger interface {
	// PingContext checks database availability using the provided context.
	PingContext(ctx context.Context) error
}

// AuditPublisher publishes audit events created after successful metric updates.
type AuditPublisher interface {
	// Notify publishes an audit event.
	Notify(ctx context.Context, event audit.Event) error
}

// MetricsHandler handles HTTP requests for metrics.
type MetricsHandler struct {
	service MetricsService
	db      DatabasePinger
	auditor AuditPublisher
}

// NewMetricsHandler creates a metrics handler without database or audit support.
func NewMetricsHandler(service MetricsService) *MetricsHandler {
	return &MetricsHandler{service: service}
}

// NewMetricsHandlerWithDB creates a metrics handler with database ping support.
func NewMetricsHandlerWithDB(service MetricsService, db DatabasePinger) *MetricsHandler {
	return &MetricsHandler{
		service: service,
		db:      db,
	}
}

// NewMetricsHandlerWithDBAndAudit creates a metrics handler with database and audit support.
func NewMetricsHandlerWithDBAndAudit(service MetricsService, db DatabasePinger, auditor AuditPublisher) *MetricsHandler {
	return &MetricsHandler{
		service: service,
		db:      db,
		auditor: auditor,
	}
}

// UpdateMetric handles legacy URL-based metric updates.
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
		h.audit(r, []string{metricName})
		w.WriteHeader(http.StatusOK)

	case models.Counter:
		value, err := strconv.ParseInt(metricValue, 10, 64)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		h.service.UpdateCounter(metricName, value)
		h.audit(r, []string{metricName})
		w.WriteHeader(http.StatusOK)

	default:
		w.WriteHeader(http.StatusBadRequest)
	}
}

// UpdateMetricJSON handles a single JSON metric update.
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
	h.audit(r, []string{metric.ID})

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(metric); err != nil {
		log.Printf("encode update response: %v", err)
	}
}

// UpdateMetricsJSON handles a batch JSON metric update.
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
		h.audit(r, metricNames(metrics))
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(metrics); err != nil {
		log.Printf("encode batch update response: %v", err)
	}
}

// GetValue handles legacy URL-based metric value lookups.
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

// GetValueJSON handles JSON metric value lookups.
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

// ListMetrics renders an HTML page with all known metrics.
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

// Ping checks database availability for health checks.
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

func (h *MetricsHandler) audit(r *http.Request, metricNames []string) {
	if h.auditor == nil || len(metricNames) == 0 {
		return
	}
	if err := h.auditor.Notify(r.Context(), audit.NewEvent(metricNames, r.RemoteAddr)); err != nil {
		log.Printf("publish audit event: %v", err)
	}
}

func metricNames(metrics []models.Metrics) []string {
	names := make([]string, 0, len(metrics))
	for _, metric := range metrics {
		names = append(names, metric.ID)
	}
	return names
}
