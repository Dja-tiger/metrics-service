package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Dja-tiger/metrics-service/internal/delivery"
	"github.com/Dja-tiger/metrics-service/internal/repository"
	"github.com/Dja-tiger/metrics-service/internal/service"
)

func TestBatchIdempotencyAndAudit(t *testing.T) {
	store := repository.NewMemStorage()
	auditor := &fakeAuditor{}
	h := NewMetricsHandlerWithDBAndAudit(service.NewMetricsService(store), nil, auditor)
	send := func(key string, delta string) int {
		r := httptest.NewRequest(http.MethodPost, "/updates/", strings.NewReader(`[{"id":"PollCount","type":"counter","delta":`+delta+`}]`))
		if key != "" {
			r.Header.Set(delivery.Header, key)
		}
		w := httptest.NewRecorder()
		h.UpdateMetricsJSON(w, r)
		return w.Code
	}
	for range 3 {
		if got := send("batch-one", "5"); got != 200 {
			t.Fatalf("duplicate status=%d", got)
		}
	}
	if got := send("batch-one", "7"); got != 409 {
		t.Fatalf("key conflict status=%d", got)
	}
	if got := send("bad key", "5"); got != 400 {
		t.Fatalf("invalid key status=%d", got)
	}
	if got := send(strings.Repeat("a", 129), "5"); got != 400 {
		t.Fatalf("long key status=%d", got)
	}
	if value, _ := store.GetCounter("PollCount"); value != 5 {
		t.Fatalf("duplicates changed counter: %d", value)
	}
	if len(auditor.events) != 1 {
		t.Fatalf("duplicate audits=%d", len(auditor.events))
	}
	// Legacy clients without an idempotency key still use additive updates.
	for range 2 {
		if got := send("", "5"); got != 200 {
			t.Fatal(got)
		}
	}
	if value, _ := store.GetCounter("PollCount"); value != 15 {
		t.Fatalf("legacy updates changed behavior: %d", value)
	}
}
