package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

type mockService struct {
	lastGaugeName   string
	lastGaugeValue  float64
	lastCounterName string
	lastCounterVal  int64
	gaugeCalls      int
	counterCalls    int
	gauges          map[string]float64
	counters        map[string]int64
}

func (m *mockService) UpdateGauge(name string, value float64) {
	m.lastGaugeName = name
	m.lastGaugeValue = value
	m.gaugeCalls++
}

func (m *mockService) UpdateCounter(name string, value int64) {
	m.lastCounterName = name
	m.lastCounterVal = value
	m.counterCalls++
}

func (m *mockService) GetGauge(name string) (float64, bool) {
	value, ok := m.gauges[name]
	return value, ok
}

func (m *mockService) GetCounter(name string) (int64, bool) {
	value, ok := m.counters[name]
	return value, ok
}

func (m *mockService) GetAllGauges() map[string]float64 {
	copyMap := make(map[string]float64, len(m.gauges))
	for k, v := range m.gauges {
		copyMap[k] = v
	}
	return copyMap
}

func (m *mockService) GetAllCounters() map[string]int64 {
	copyMap := make(map[string]int64, len(m.counters))
	for k, v := range m.counters {
		copyMap[k] = v
	}
	return copyMap
}

func TestUpdateMetric(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		path           string
		wantStatusCode int
		wantGaugeCall  bool
		wantCountCall  bool
	}{
		{
			name:           "valid gauge",
			method:         http.MethodPost,
			path:           "/update/gauge/Alloc/12.34",
			wantStatusCode: http.StatusOK,
			wantGaugeCall:  true,
		},
		{
			name:           "valid counter",
			method:         http.MethodPost,
			path:           "/update/counter/PollCount/5",
			wantStatusCode: http.StatusOK,
			wantCountCall:  true,
		},
		{
			name:           "wrong method",
			method:         http.MethodGet,
			path:           "/update/gauge/Alloc/12.34",
			wantStatusCode: http.StatusMethodNotAllowed,
		},
		{
			name:           "bad path",
			method:         http.MethodPost,
			path:           "/update/gauge/Alloc",
			wantStatusCode: http.StatusNotFound,
		},
		{
			name:           "bad type",
			method:         http.MethodPost,
			path:           "/update/unknown/Alloc/10",
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name:           "bad value",
			method:         http.MethodPost,
			path:           "/update/gauge/Alloc/not-a-number",
			wantStatusCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &mockService{
				gauges:   map[string]float64{},
				counters: map[string]int64{},
			}
			h := NewMetricsHandler(svc)

			router := chi.NewRouter()
			router.Post("/update/{type}/{name}/{value}", h.UpdateMetric)

			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatusCode {
				t.Fatalf("status code mismatch: got %d want %d", rec.Code, tt.wantStatusCode)
			}

			if tt.wantGaugeCall && svc.gaugeCalls != 1 {
				t.Fatalf("expected gauge update to be called once, got %d", svc.gaugeCalls)
			}

			if tt.wantCountCall && svc.counterCalls != 1 {
				t.Fatalf("expected counter update to be called once, got %d", svc.counterCalls)
			}
		})
	}
}

func TestGetValue(t *testing.T) {
	svc := &mockService{
		gauges: map[string]float64{
			"Alloc": 12.34,
		},
		counters: map[string]int64{
			"PollCount": 5,
		},
	}
	h := NewMetricsHandler(svc)

	router := chi.NewRouter()
	router.Get("/value/{type}/{name}", h.GetValue)

	tests := []struct {
		name           string
		path           string
		wantStatusCode int
		wantBody       string
	}{
		{
			name:           "gauge value",
			path:           "/value/gauge/Alloc",
			wantStatusCode: http.StatusOK,
			wantBody:       "12.34",
		},
		{
			name:           "counter value",
			path:           "/value/counter/PollCount",
			wantStatusCode: http.StatusOK,
			wantBody:       "5",
		},
		{
			name:           "unknown metric",
			path:           "/value/gauge/Unknown",
			wantStatusCode: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatusCode {
				t.Fatalf("status code mismatch: got %d want %d", rec.Code, tt.wantStatusCode)
			}

			if tt.wantBody != "" && strings.TrimSpace(rec.Body.String()) != tt.wantBody {
				t.Fatalf("body mismatch: got %q want %q", rec.Body.String(), tt.wantBody)
			}
		})
	}
}

func TestListMetrics(t *testing.T) {
	svc := &mockService{
		gauges: map[string]float64{
			"Alloc": 12.34,
		},
		counters: map[string]int64{
			"PollCount": 5,
		},
	}
	h := NewMetricsHandler(svc)

	router := chi.NewRouter()
	router.Get("/", h.ListMetrics)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code mismatch: got %d want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Alloc") || !strings.Contains(body, "12.34") {
		t.Fatalf("expected gauge in response body, got %q", body)
	}
	if !strings.Contains(body, "PollCount") || !strings.Contains(body, "5") {
		t.Fatalf("expected counter in response body, got %q", body)
	}
}
