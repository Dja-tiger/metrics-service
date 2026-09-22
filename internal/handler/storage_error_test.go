package handler

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Dja-tiger/metrics-service/internal/repository"
	"github.com/Dja-tiger/metrics-service/internal/service"
	"github.com/go-chi/chi/v5"
)

func TestWriteErrorsAreNotAcknowledged(t *testing.T) {
	for _, storageKind := range []string{"postgres", "file"} {
		t.Run(storageKind, func(t *testing.T) {
			var svc *service.MetricsService
			if storageKind == "postgres" {
				db, err := repository.NewPostgresDB("host=localhost dbname=unused")
				if err != nil {
					t.Fatal(err)
				}
				// A closed DB deterministically fails without requiring a PostgreSQL process.
				if err := db.Close(); err != nil {
					t.Fatal(err)
				}
				svc = service.NewMetricsService(repository.NewPostgresStorage(db))
			} else {
				parent := filepath.Join(t.TempDir(), "not-a-directory")
				if err := os.WriteFile(parent, []byte("file"), 0600); err != nil {
					t.Fatal(err)
				}
				store := repository.NewMemStorage()
				svc = service.NewMetricsServiceWithPersistence(store, store, filepath.Join(parent, "metrics.json"), 0, nil)
				t.Cleanup(func() { _ = svc.Close() })
			}
			auditor := &fakeAuditor{}
			h := NewMetricsHandlerWithDBAndAudit(svc, nil, auditor)
			router := chi.NewRouter()
			router.Post("/update/{type}/{name}/{value}", h.UpdateMetric)
			router.Post("/update", h.UpdateMetricJSON)
			router.Post("/updates", h.UpdateMetricsJSON)
			for _, tc := range []struct{ path, body string }{
				{"/update/gauge/Alloc/1", ""},
				{"/update/counter/PollCount/1", ""},
				{"/update", `{"id":"Alloc","type":"gauge","value":1}`},
				{"/updates", `[{"id":"PollCount","type":"counter","delta":1}]`},
			} {
				response := httptest.NewRecorder()
				router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body)))
				if response.Code != http.StatusInternalServerError {
					t.Errorf("%s returned %d", tc.path, response.Code)
				}
			}
			if len(auditor.events) != 0 {
				t.Error("failed writes produced successful audit events")
			}
		})
	}
}
