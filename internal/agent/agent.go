package agent

import (
	"bytes"
	"compress/gzip"
	"context"
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

// Agent collects runtime and system metrics and reports them to a metrics server.
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

// Option configures an Agent during construction.
type Option func(*Agent)

// WithHTTPClient configures the HTTP client used for metric delivery.
func WithHTTPClient(client *http.Client) Option {
	return func(a *Agent) {
		if client != nil {
			a.client = client
		}
	}
}

// WithStore configures the in-memory metric store used by the agent.
func WithStore(store *Store) Option {
	return func(a *Agent) {
		if store != nil {
			a.store = store
		}
	}
}

// WithKey configures the SHA256 signing key for outgoing requests.
func WithKey(key string) Option {
	return func(a *Agent) {
		a.key = key
	}
}

// WithRateLimit configures the maximum number of concurrent report workers.
func WithRateLimit(rateLimit int) Option {
	return func(a *Agent) {
		a.rateLimit = rateLimit
	}
}

// WithRetrySleep configures the sleep function used between retriable send attempts.
func WithRetrySleep(sleep func(time.Duration)) Option {
	return func(a *Agent) {
		if sleep != nil {
			a.retrySleep = sleep
		}
	}
}

// NewAgent creates a metrics agent with validated polling and reporting settings.
func NewAgent(serverURL string, pollInterval, reportInterval time.Duration, options ...Option) (*Agent, error) {
	agent := &Agent{
		pollInterval:   pollInterval,
		reportInterval: reportInterval,
		serverURL:      serverURL,
		client:         &http.Client{Timeout: 5 * time.Second},
		store:          NewStore(),
		rateLimit:      1,
		retrySleep:     time.Sleep,
	}

	for _, option := range options {
		if option != nil {
			option(agent)
		}
	}

	if err := agent.validate(); err != nil {
		return nil, err
	}

	return agent, nil
}

func (a *Agent) validate() error {
	if a.serverURL == "" {
		return fmt.Errorf("server URL is required")
	}
	if a.pollInterval <= 0 {
		return fmt.Errorf("poll interval must be positive")
	}
	if a.reportInterval <= 0 {
		return fmt.Errorf("report interval must be positive")
	}
	if a.rateLimit <= 0 {
		return fmt.Errorf("rate limit must be positive")
	}
	return nil
}

// PollOnce collects runtime metrics and stores them in the agent store.
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

// ReportOnce sends a single snapshot of currently collected metrics.
func (a *Agent) ReportOnce() {
	_ = a.sendMetrics(a.store.SnapshotMetrics())
}

// PollLoop collects runtime metrics periodically until the context is canceled.
func (a *Agent) PollLoop(ctx context.Context) {
	ticker := time.NewTicker(a.pollInterval)
	defer ticker.Stop()

	a.PollOnce()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.PollOnce()
		}
	}
}

// ReportLoop sends metric snapshots periodically using a bounded worker pool.
func (a *Agent) ReportLoop(ctx context.Context) {
	jobs := make(chan []models.Metrics, a.rateLimit)
	workers := a.startReportWorkers(jobs)
	defer func() {
		close(jobs)
		workers.Wait()
	}()

	ticker := time.NewTicker(a.reportInterval)
	defer ticker.Stop()

	a.enqueueReport(ctx, jobs)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.enqueueReport(ctx, jobs)
		}
	}
}

func (a *Agent) enqueueReport(ctx context.Context, jobs chan<- []models.Metrics) {
	metrics := a.store.SnapshotMetrics()
	if len(metrics) == 0 {
		return
	}

	select {
	case jobs <- metrics:
	case <-ctx.Done():
	}
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
