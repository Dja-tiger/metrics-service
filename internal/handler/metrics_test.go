package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

type mockService struct {
	lastGaugeName   string
	lastGaugeValue  float64
	lastCounterName string
	lastCounterVal  int64
	gaugeCalls      int
	counterCalls    int
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
			svc := &mockService{}
			h := NewMetricsHandler(svc)

			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()
			h.UpdateMetric(rec, req)

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
