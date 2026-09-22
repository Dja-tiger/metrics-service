package agent

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"runtime"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/Dja-tiger/metrics-service/internal/delivery"
	"github.com/Dja-tiger/metrics-service/internal/encryption"
	models "github.com/Dja-tiger/metrics-service/internal/model"
	"github.com/Dja-tiger/metrics-service/internal/retry"
	"github.com/Dja-tiger/metrics-service/internal/signature"
)

var errRetriableSend = errors.New("retriable send error")

// Agent collects runtime and system metrics and reports them to a metrics server.
type Agent struct {
	pendingMu      sync.Mutex
	pending        []delivery.Batch
	pollInterval   time.Duration
	reportInterval time.Duration
	serverURL      string
	client         *http.Client
	store          *Store
	key            string
	publicKey      *rsa.PublicKey
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

// WithPublicKey enables request encryption. The key must not be modified after construction.
func WithPublicKey(key *rsa.PublicKey) Option {
	return func(a *Agent) { a.publicKey = key }
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
func (a *Agent) ReportOnce() error {
	batches := a.takePending()
	var result error
	for _, batch := range batches {
		if err := a.sendBatch(batch); err != nil {
			a.requeue(batch)
			result = errors.Join(result, err)
		}
	}
	if result != nil {
		return result
	}
	metrics := a.store.SnapshotMetrics()
	if len(metrics) == 0 {
		return nil
	}
	batch := delivery.New(metrics)
	if err := a.sendBatch(batch); err != nil {
		a.requeue(batch)
		return err
	}
	return nil
}

func (a *Agent) takePending() []delivery.Batch {
	a.pendingMu.Lock()
	defer a.pendingMu.Unlock()
	batches := a.pending
	a.pending = nil
	return batches
}

func (a *Agent) requeue(batch delivery.Batch) {
	a.pendingMu.Lock()
	defer a.pendingMu.Unlock()
	a.pending = append(a.pending, batch)
}

// Run collects and reports metrics until cancellation, then drains all work and
// sends a final snapshot. The server must remain available until Run returns.
func (a *Agent) Run(ctx context.Context) error {
	group, runCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		a.PollLoop(runCtx)
		return nil
	})
	group.Go(func() error { return a.SystemPollLoop(runCtx) })
	group.Go(func() error {
		a.ReportLoop(runCtx)
		return nil
	})
	err := group.Wait()
	// Retry the original failed batches before sending the final collected snapshot.
	return errors.Join(err, a.ReportOnce())
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
// Delivery failures are logged and requeued; Run checks the final delivery result.
func (a *Agent) ReportLoop(ctx context.Context) {
	jobs := make(chan delivery.Batch, a.rateLimit)
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

func (a *Agent) enqueueReport(ctx context.Context, jobs chan<- delivery.Batch) {
	batches := a.takePending()
	if len(batches) == 0 {
		metrics := a.store.SnapshotMetrics()
		if len(metrics) == 0 {
			return
		}
		batches = []delivery.Batch{delivery.New(metrics)}
	}
	for i, batch := range batches {
		select {
		case jobs <- batch:
		case <-ctx.Done():
			for _, pending := range batches[i:] {
				a.requeue(pending)
			}
			return
		}
	}
}

func (a *Agent) startReportWorkers(jobs <-chan delivery.Batch) *sync.WaitGroup {
	var workers sync.WaitGroup
	workers.Add(a.rateLimit)

	for range a.rateLimit {
		go func() {
			defer workers.Done()
			for batch := range jobs {
				if err := a.sendBatch(batch); err != nil {
					a.requeue(batch)
					log.Printf("report metrics: %v", err)
				}
			}
		}()
	}

	return &workers
}

func (a *Agent) sendMetrics(metrics []models.Metrics) error {
	if len(metrics) == 0 {
		return nil
	}
	return a.sendBatch(delivery.New(metrics))
}

func (a *Agent) sendBatch(batch delivery.Batch) error {
	metrics := batch.Metrics
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

	payload := compressedBody.Bytes()
	if a.publicKey != nil {
		payload, err = encryption.Encrypt(a.publicKey, payload)
		if err != nil {
			return fmt.Errorf("encrypt metrics: %w", err)
		}
	}

	return retry.DoWithSleeper(func() error {
		return a.sendCompressedMetrics(payload, hash, batch.ID)
	}, isRetriableSendError, a.retrySleep)
}

func (a *Agent) sendCompressedMetrics(body []byte, hash string, batchID string) error {
	url := fmt.Sprintf("%s/updates/", a.serverURL)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set(delivery.Header, batchID)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("Accept-Encoding", "gzip")
	if a.publicKey != nil {
		req.Header.Set(encryption.Header, encryption.Algorithm)
	}
	if hash != "" {
		req.Header.Set(signature.Header, hash)
	}

	resp, err := a.client.Do(withRealIP(req))
	if err != nil {
		return fmt.Errorf("%w: send request: %w", errRetriableSend, err)
	}
	defer resp.Body.Close()
	// Drain the acknowledgement so the transport can reuse the connection.
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

func isRetriableSendError(err error) bool {
	return errors.Is(err, errRetriableSend)
}
