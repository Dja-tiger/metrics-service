package agent

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"runtime"
	"time"

	models "github.com/Dja-tiger/metrics-service/internal/model"
)

// Agent collects runtime metrics and reports them via HTTP.
type Agent struct {
	pollInterval   time.Duration
	reportInterval time.Duration
	serverURL      string
	client         *http.Client
	store          *Store
}

func NewAgent(serverURL string, pollInterval, reportInterval time.Duration, client *http.Client, store *Store) (*Agent, error) {
	if serverURL == "" {
		return nil, fmt.Errorf("server URL is required")
	}
	if pollInterval <= 0 {
		return nil, fmt.Errorf("poll interval must be positive")
	}
	if reportInterval <= 0 {
		return nil, fmt.Errorf("report interval must be positive")
	}
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	if store == nil {
		store = NewStore()
	}

	return &Agent{
		pollInterval:   pollInterval,
		reportInterval: reportInterval,
		serverURL:      serverURL,
		client:         client,
		store:          store,
	}, nil
}

func (a *Agent) PollOnce() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	a.store.SetGauge("Alloc", float64(memStats.Alloc))
	a.store.SetGauge("BuckHashSys", float64(memStats.BuckHashSys))
	a.store.SetGauge("Frees", float64(memStats.Frees))
	a.store.SetGauge("GCCPUFraction", memStats.GCCPUFraction)
	a.store.SetGauge("GCSys", float64(memStats.GCSys))
	a.store.SetGauge("HeapAlloc", float64(memStats.HeapAlloc))
	a.store.SetGauge("HeapIdle", float64(memStats.HeapIdle))
	a.store.SetGauge("HeapInuse", float64(memStats.HeapInuse))
	a.store.SetGauge("HeapObjects", float64(memStats.HeapObjects))
	a.store.SetGauge("HeapReleased", float64(memStats.HeapReleased))
	a.store.SetGauge("HeapSys", float64(memStats.HeapSys))
	a.store.SetGauge("LastGC", float64(memStats.LastGC))
	a.store.SetGauge("Lookups", float64(memStats.Lookups))
	a.store.SetGauge("MCacheInuse", float64(memStats.MCacheInuse))
	a.store.SetGauge("MCacheSys", float64(memStats.MCacheSys))
	a.store.SetGauge("MSpanInuse", float64(memStats.MSpanInuse))
	a.store.SetGauge("MSpanSys", float64(memStats.MSpanSys))
	a.store.SetGauge("Mallocs", float64(memStats.Mallocs))
	a.store.SetGauge("NextGC", float64(memStats.NextGC))
	a.store.SetGauge("NumForcedGC", float64(memStats.NumForcedGC))
	a.store.SetGauge("NumGC", float64(memStats.NumGC))
	a.store.SetGauge("OtherSys", float64(memStats.OtherSys))
	a.store.SetGauge("PauseTotalNs", float64(memStats.PauseTotalNs))
	a.store.SetGauge("StackInuse", float64(memStats.StackInuse))
	a.store.SetGauge("StackSys", float64(memStats.StackSys))
	a.store.SetGauge("Sys", float64(memStats.Sys))
	a.store.SetGauge("TotalAlloc", float64(memStats.TotalAlloc))

	a.store.SetGauge("RandomValue", rand.Float64())
	a.store.IncCounter("PollCount", 1)
}

func (a *Agent) ReportOnce() {
	gauges := a.store.SnapshotGauges()
	for name, value := range gauges {
		valueCopy := value
		_ = a.sendMetric(models.Metrics{
			ID:    name,
			MType: models.Gauge,
			Value: &valueCopy,
		})
	}

	counters := a.store.SnapshotAndResetCounters()
	for name, value := range counters {
		valueCopy := value
		_ = a.sendMetric(models.Metrics{
			ID:    name,
			MType: models.Counter,
			Delta: &valueCopy,
		})
	}
}

func (a *Agent) PollLoop() {
	ticker := time.NewTicker(a.pollInterval)
	defer ticker.Stop()

	a.PollOnce()
	for range ticker.C {
		a.PollOnce()
	}
}

func (a *Agent) ReportLoop() {
	ticker := time.NewTicker(a.reportInterval)
	defer ticker.Stop()

	a.ReportOnce()
	for range ticker.C {
		a.ReportOnce()
	}
}

func (a *Agent) sendMetric(metric models.Metrics) error {
	body, err := json.Marshal(metric)
	if err != nil {
		return fmt.Errorf("marshal metric: %w", err)
	}

	var compressedBody bytes.Buffer
	gzipWriter := gzip.NewWriter(&compressedBody)
	if _, err = gzipWriter.Write(body); err != nil {
		return fmt.Errorf("compress metric: %w", err)
	}
	if err = gzipWriter.Close(); err != nil {
		return fmt.Errorf("close compressor: %w", err)
	}

	url := fmt.Sprintf("%s/update", a.serverURL)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(compressedBody.Bytes()))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	return nil
}
