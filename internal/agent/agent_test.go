package agent

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

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

	paths := make(map[string]struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "text/plain" {
			t.Fatalf("unexpected content-type: %s", r.Header.Get("Content-Type"))
		}
		paths[r.URL.Path] = struct{}{}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	a, err := NewAgent(server.URL, time.Second, time.Second, server.Client(), store)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}
	a.ReportOnce()

	expected := []string{
		"/update/gauge/TestGauge/12.34",
		"/update/counter/TestCounter/7",
	}

	for _, p := range expected {
		found := false
		for got := range paths {
			if strings.HasPrefix(got, p) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected request path %q not found; got %#v", p, paths)
		}
	}
}
