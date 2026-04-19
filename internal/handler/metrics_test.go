package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/Dja-tiger/metrics-service/internal/repository"
	"github.com/Dja-tiger/metrics-service/internal/service"
)

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
			storage := repository.NewMemStorage()
			svc := service.NewMetricsService(storage)
			h := NewMetricsHandler(svc)

			router := chi.NewRouter()
			router.Post("/update/{type}/{name}/{value}", h.UpdateMetric)

			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatusCode {
				t.Fatalf("status code mismatch: got %d want %d", rec.Code, tt.wantStatusCode)
			}

			if tt.wantGaugeCall {
				if _, ok := storage.GetGauge("Alloc"); !ok {
					t.Fatalf("expected gauge to be stored")
				}
			}

			if tt.wantCountCall {
				if value, ok := storage.GetCounter("PollCount"); !ok || value != 5 {
					t.Fatalf("expected counter to be stored with value 5, got %d", value)
				}
			}
		})
	}
}

func TestGetValue(t *testing.T) {
	storage := repository.NewMemStorage()
	storage.UpdateGauge("Alloc", 12.34)
	storage.UpdateCounter("PollCount", 5)
	svc := service.NewMetricsService(storage)
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
	storage := repository.NewMemStorage()
	storage.UpdateGauge("Alloc", 12.34)
	storage.UpdateCounter("PollCount", 5)
	svc := service.NewMetricsService(storage)
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
