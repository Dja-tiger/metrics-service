package agent

import (
	"compress/gzip"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type receivedMetric struct {
	ID    string   `json:"id"`
	MType string   `json:"type"`
	Delta *int64   `json:"delta,omitempty"`
	Value *float64 `json:"value,omitempty"`
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
		if r.URL.Path != "/update" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("unexpected content-type: %s", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("Content-Encoding") != "gzip" {
			t.Fatalf("unexpected content-encoding: %s", r.Header.Get("Content-Encoding"))
		}

		var metric receivedMetric
		gzipReader, err := gzip.NewReader(r.Body)
		if err != nil {
			t.Fatalf("failed to create gzip reader: %v", err)
		}
		defer gzipReader.Close()

		if err = json.NewDecoder(gzipReader).Decode(&metric); err != nil {
			t.Fatalf("failed to decode metric body: %v", err)
		}
		received[metric.MType+":"+metric.ID] = metric

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
