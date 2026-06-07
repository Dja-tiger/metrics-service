package agent

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"runtime"
	"sync"
	"time"

	models "github.com/Dja-tiger/metrics-service/internal/model"
	"github.com/Dja-tiger/metrics-service/internal/retry"
	"github.com/Dja-tiger/metrics-service/internal/signature"
)

var errRetriableSend = errors.New("retriable send error")

// Agent collects runtime metrics and reports them via HTTP.
type Agent struct {
	pollInterval   time.Duration
	reportInterval time.Duration
	serverURL      string
	client         *http.Client
	store          *Store
	key            string
	rateLimit      int
	retrySleep     func(time.Duration)
}

func NewAgent(serverURL string, pollInterval, reportInterval time.Duration, client *http.Client, store *Store) (*Agent, error) {
	return NewAgentWithKey(serverURL, pollInterval, reportInterval, client, store, "")
}

func NewAgentWithKey(serverURL string, pollInterval, reportInterval time.Duration, client *http.Client, store *Store, key string) (*Agent, error) {
	return NewAgentWithKeyAndRateLimit(serverURL, pollInterval, reportInterval, client, store, key, 1)
}

func NewAgentWithKeyAndRateLimit(serverURL string, pollInterval, reportInterval time.Duration, client *http.Client, store *Store, key string, rateLimit int) (*Agent, error) {
	if serverURL == "" {
		return nil, fmt.Errorf("server URL is required")
	}
	if pollInterval <= 0 {
		return nil, fmt.Errorf("poll interval must be positive")
	}
	if reportInterval <= 0 {
		return nil, fmt.Errorf("report interval must be positive")
	}
	if rateLimit <= 0 {
		return nil, fmt.Errorf("rate limit must be positive")
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
		key:            key,
		rateLimit:      rateLimit,
		retrySleep:     time.Sleep,
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
	_ = a.sendMetrics(a.store.SnapshotMetrics())
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
	jobs := make(chan []models.Metrics, a.rateLimit)
	a.startReportWorkers(jobs)

	ticker := time.NewTicker(a.reportInterval)
	defer ticker.Stop()

	a.enqueueReport(jobs)
	for range ticker.C {
		a.enqueueReport(jobs)
	}
}

func (a *Agent) enqueueReport(jobs chan<- []models.Metrics) {
	metrics := a.store.SnapshotMetrics()
	if len(metrics) == 0 {
		return
	}
	jobs <- metrics
}

func (a *Agent) startReportWorkers(jobs <-chan []models.Metrics) *sync.WaitGroup {
	var workers sync.WaitGroup
	workers.Add(a.rateLimit)

	for range a.rateLimit {
		go func() {
			defer workers.Done()
			for metrics := range jobs {
				_ = a.sendMetrics(metrics)
			}
		}()
	}

	return &workers
}

func (a *Agent) sendMetrics(metrics []models.Metrics) error {
	if len(metrics) == 0 {
		return nil
	}

	body, err := json.Marshal(metrics)
	if err != nil {
		return fmt.Errorf("marshal metrics: %w", err)
	}

	var compressedBody bytes.Buffer
	gzipWriter := gzip.NewWriter(&compressedBody)
	if _, err = gzipWriter.Write(body); err != nil {
		return fmt.Errorf("compress metrics: %w", err)
	}
	if err = gzipWriter.Close(); err != nil {
		return fmt.Errorf("close compressor: %w", err)
	}

	hash := ""
	if a.key != "" {
		hash = signature.Calculate(body, a.key)
	}

	return retry.DoWithSleeper(func() error {
		return a.sendCompressedMetrics(compressedBody.Bytes(), hash)
	}, isRetriableSendError, a.retrySleep)
}

func (a *Agent) sendCompressedMetrics(body []byte, hash string) error {
	url := fmt.Sprintf("%s/updates/", a.serverURL)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("Accept-Encoding", "gzip")
	if hash != "" {
		req.Header.Set(signature.Header, hash)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: send request: %w", errRetriableSend, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

func isRetriableSendError(err error) bool {
	return errors.Is(err, errRetriableSend)
}
