package agent

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Dja-tiger/metrics-service/internal/delivery"
	models "github.com/Dja-tiger/metrics-service/internal/model"
	"github.com/shirou/gopsutil/v4/mem"
)

func TestRunDrainsInFlightAndFinalMetrics(t *testing.T) {
	oldMemory, oldCPU := readVirtualMemory, readCPUPercent
	t.Cleanup(func() { readVirtualMemory, readCPUPercent = oldMemory, oldCPU })
	collecting, releaseCollector := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(releaseCollector) }) })
	readVirtualMemory = func() (*mem.VirtualMemoryStat, error) {
		close(collecting)
		<-releaseCollector
		return &mem.VirtualMemoryStat{Total: 100, Free: 50}, nil
	}
	readCPUPercent = func(time.Duration, bool) ([]float64, error) { return []float64{12}, nil }
	started, releaseRequest := make(chan struct{}), make(chan struct{})
	var requestOnce sync.Once
	t.Cleanup(func() { requestOnce.Do(func() { close(releaseRequest) }) })
	var batches [][]models.Metrics
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		reader, err := gzip.NewReader(r.Body)
		if err != nil {
			return nil, err
		}
		var batch []models.Metrics
		err = json.NewDecoder(reader).Decode(&batch)
		_ = reader.Close()
		_ = r.Body.Close()
		if err != nil {
			return nil, err
		}
		batches = append(batches, batch)
		if len(batches) == 1 {
			close(started)
			<-releaseRequest
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})}
	store := NewStore()
	store.IncCounter("TestCounter", 3)
	a, err := NewAgent("http://server", time.Hour, time.Hour, WithHTTPClient(client), WithStore(store))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- a.Run(ctx) }()
	for _, ch := range []chan struct{}{started, collecting} {
		select {
		case <-ch:
		case <-time.After(3 * time.Second):
			t.Fatal("work did not start")
		}
	}
	store.IncCounter("TestCounter", 2)
	cancel()
	select {
	case <-done:
		t.Fatal("returned before in-flight work completed")
	case <-time.After(20 * time.Millisecond):
	}
	requestOnce.Do(func() { close(releaseRequest) })
	select {
	case <-done:
		t.Fatal("returned before collector completed")
	case <-time.After(20 * time.Millisecond):
	}
	releaseOnce.Do(func() { close(releaseCollector) })
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown stuck")
	}
	var total int64
	var finalMemory float64
	for _, batch := range batches {
		for _, m := range batch {
			if m.ID == "TestCounter" {
				total += *m.Delta
			}
			if m.ID == "TotalMemory" {
				finalMemory = *m.Value
			}
		}
	}
	if total != 5 || finalMemory != 100 {
		t.Fatalf("counter=%d memory=%v", total, finalMemory)
	}
}

func TestCanceledEnqueueRestoresCounters(t *testing.T) {
	a, err := NewAgent("http://server", time.Hour, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	a.store.IncCounter("PollCount", 5)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	a.enqueueReport(ctx, make(chan delivery.Batch))
	pending := a.takePending()
	if len(pending) != 1 || pending[0].ID == "" || *pending[0].Metrics[0].Delta != 5 {
		t.Fatalf("original batch lost: %+v", pending)
	}
}

func TestFailedReportRestoresCounters(t *testing.T) {
	a, err := NewAgent("http://server", time.Hour, time.Hour, WithHTTPClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, errors.New("offline") })}), WithRetrySleep(func(time.Duration) {}))
	if err != nil {
		t.Fatal(err)
	}
	a.store.IncCounter("PollCount", 5)
	if err := a.ReportOnce(); err == nil {
		t.Fatal("delivery error hidden")
	}
	pending := a.takePending()
	if len(pending) != 1 || pending[0].ID == "" || *pending[0].Metrics[0].Delta != 5 {
		t.Fatalf("original batch lost: %+v", pending)
	}
}

func TestRunSucceedsAfterFailedBatchIsRecovered(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	var delivered int64
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls <= 4 {
			if calls == 4 {
				cancel()
			}
			_ = r.Body.Close()
			return nil, errors.New("temporarily offline")
		}
		reader, err := gzip.NewReader(r.Body)
		if err != nil {
			return nil, err
		}
		defer reader.Close()
		defer r.Body.Close()
		var batch []models.Metrics
		if err := json.NewDecoder(reader).Decode(&batch); err != nil {
			return nil, err
		}
		for _, m := range batch {
			if m.ID == "TestCounter" {
				delivered += *m.Delta
			}
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("ok")), Header: make(http.Header)}, nil
	})}
	store := NewStore()
	store.IncCounter("TestCounter", 7)
	a, err := NewAgent("http://server", time.Hour, time.Hour, WithHTTPClient(client), WithStore(store), WithRetrySleep(func(time.Duration) {}))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Run(ctx); err != nil {
		t.Fatalf("recovered delivery reported as failure: %v", err)
	}
	if calls != 6 || delivered != 7 {
		t.Fatalf("calls=%d delivered=%d", calls, delivered)
	}
}
