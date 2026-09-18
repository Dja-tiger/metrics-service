package agent

import (
	"compress/gzip"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Dja-tiger/metrics-service/internal/delivery"
	"github.com/Dja-tiger/metrics-service/internal/handler"
	"github.com/Dja-tiger/metrics-service/internal/repository"
	"github.com/Dja-tiger/metrics-service/internal/service"
)

func TestLostHTTPAcknowledgementsDoNotDuplicateCounters(t *testing.T) {
	stored := repository.NewMemStorage()
	h := handler.NewMetricsHandler(service.NewMetricsService(stored))
	var mu sync.Mutex
	var ids []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		ids = append(ids, r.Header.Get(delivery.Header))
		mu.Unlock()
		reader, err := gzip.NewReader(r.Body)
		if err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		defer reader.Close()
		r.Body = reader
		h.UpdateMetricsJSON(w, r)
	}))
	defer server.Close()
	calls := 0
	transport := server.Client().Transport
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		response, err := transport.RoundTrip(r)
		if err != nil {
			return nil, err
		}
		calls++
		if calls <= 4 {
			response.Body.Close()
			return nil, errors.New("response lost after server commit")
		}
		return response, nil
	})}
	store := NewStore()
	store.IncCounter("PollCount", 5)
	a, err := NewAgent(server.URL, time.Hour, time.Hour, WithStore(store), WithHTTPClient(client), WithRetrySleep(func(time.Duration) {}))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.ReportOnce(); err == nil {
		t.Fatal("expected exhausted retries")
	}
	if got, _ := stored.GetCounter("PollCount"); got != 5 {
		t.Fatalf("four attempts applied %d instead of 5", got)
	}
	store.IncCounter("PollCount", 2)
	if err := a.ReportOnce(); err != nil {
		t.Fatal(err)
	}
	if got, _ := stored.GetCounter("PollCount"); got != 7 {
		t.Fatalf("resubmitted pending batch applied twice: %d", got)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(ids) != 6 {
		t.Fatalf("requests=%d", len(ids))
	}
	for _, id := range ids[:5] {
		if id == "" || id != ids[0] {
			t.Fatal("retry changed batch identity")
		}
	}
	if ids[5] == ids[0] {
		t.Fatal("new metrics reused old identity")
	}
}
