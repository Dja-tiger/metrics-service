package agent

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	models "github.com/Dja-tiger/metrics-service/internal/model"
	"github.com/Dja-tiger/metrics-service/internal/signature"
)

type receivedMetric struct {
	ID    string   `json:"id"`
	MType string   `json:"type"`
	Delta *int64   `json:"delta,omitempty"`
	Value *float64 `json:"value,omitempty"`
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}

func TestPollOnceCollectsMetrics(t *testing.T) {
	store := NewStore()
	a, err := NewAgent("http://localhost:8080", time.Second, time.Second, nil, store)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	a.PollOnce()

	expectedGauges := []string{
		"Alloc",
		"BuckHashSys",
		"Frees",
		"GCCPUFraction",
		"GCSys",
		"HeapAlloc",
		"HeapIdle",
		"HeapInuse",
		"HeapObjects",
		"HeapReleased",
		"HeapSys",
		"LastGC",
		"Lookups",
		"MCacheInuse",
		"MCacheSys",
		"MSpanInuse",
		"MSpanSys",
		"Mallocs",
		"NextGC",
		"NumForcedGC",
		"NumGC",
		"OtherSys",
		"PauseTotalNs",
		"StackInuse",
		"StackSys",
		"Sys",
		"TotalAlloc",
		"RandomValue",
	}

	gauges := store.SnapshotGauges()
	for _, name := range expectedGauges {
		if _, ok := gauges[name]; !ok {
			t.Fatalf("expected gauge %q to be collected", name)
		}
	}

	counters := store.SnapshotAndResetCounters()
	if counters["PollCount"] != 1 {
		t.Fatalf("expected PollCount to be 1, got %d", counters["PollCount"])
	}
}

func TestReportOnceSendsMetrics(t *testing.T) {
	store := NewStore()
	store.SetGauge("TestGauge", 12.34)
	store.IncCounter("TestCounter", 7)

	received := make(map[string]receivedMetric)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/updates/" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("unexpected content-type: %s", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("Content-Encoding") != "gzip" {
			t.Fatalf("unexpected content-encoding: %s", r.Header.Get("Content-Encoding"))
		}

		var metrics []receivedMetric
		gzipReader, err := gzip.NewReader(r.Body)
		if err != nil {
			t.Fatalf("failed to create gzip reader: %v", err)
		}
		defer gzipReader.Close()

		if err = json.NewDecoder(gzipReader).Decode(&metrics); err != nil {
			t.Fatalf("failed to decode metrics body: %v", err)
		}
		for _, metric := range metrics {
			received[metric.MType+":"+metric.ID] = metric
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	a, err := NewAgent(server.URL, time.Second, time.Second, server.Client(), store)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}
	a.ReportOnce()

	gaugeMetric, ok := received["gauge:TestGauge"]
	if !ok || gaugeMetric.Value == nil || *gaugeMetric.Value != 12.34 {
		t.Fatalf("expected gauge metric with value 12.34, got %#v", gaugeMetric)
	}

	counterMetric, ok := received["counter:TestCounter"]
	if !ok || counterMetric.Delta == nil || *counterMetric.Delta != 7 {
		t.Fatalf("expected counter metric with delta 7, got %#v", counterMetric)
	}
}

func TestReportOnceSignsRequest(t *testing.T) {
	const key = "secret"

	store := NewStore()
	store.SetGauge("TestGauge", 12.34)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gzipReader, err := gzip.NewReader(r.Body)
		if err != nil {
			t.Fatalf("failed to create gzip reader: %v", err)
		}
		defer gzipReader.Close()

		body, err := io.ReadAll(gzipReader)
		if err != nil {
			t.Fatalf("failed to read metric body: %v", err)
		}
		if !signature.Verify(body, key, r.Header.Get(signature.Header)) {
			t.Fatal("request signature is invalid")
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	a, err := NewAgentWithKey(server.URL, time.Second, time.Second, server.Client(), store, key)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}
	a.ReportOnce()
}

func TestReportOnceSkipsEmptyBatch(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	a, err := NewAgent(server.URL, time.Second, time.Second, server.Client(), NewStore())
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}
	a.ReportOnce()

	if requests != 0 {
		t.Fatalf("expected no requests for empty batch, got %d", requests)
	}
}

func TestSendMetricsRetriesTemporaryConnectionError(t *testing.T) {
	attempts := 0
	client := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			attempts++
			if attempts < 4 {
				return nil, errors.New("connection refused")
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     make(http.Header),
			}, nil
		}),
	}

	a, err := NewAgent("http://localhost:8080", time.Second, time.Second, client, NewStore())
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	var delays []time.Duration
	a.retrySleep = func(delay time.Duration) {
		delays = append(delays, delay)
	}

	value := 1.23
	err = a.sendMetrics([]models.Metrics{
		{
			ID:    "TestGauge",
			MType: models.Gauge,
			Value: &value,
		},
	})
	if err != nil {
		t.Fatalf("unexpected send error: %v", err)
	}
	if attempts != 4 {
		t.Fatalf("unexpected attempts count: got %d want 4", attempts)
	}

	wantDelays := []time.Duration{time.Second, 3 * time.Second, 5 * time.Second}
	if !reflect.DeepEqual(delays, wantDelays) {
		t.Fatalf("unexpected delays: got %v want %v", delays, wantDelays)
	}
}
