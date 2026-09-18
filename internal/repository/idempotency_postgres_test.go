package repository

import (
	"crypto/rand"
	"database/sql"
	"net/url"
	"os"
	"strings"
	"testing"

	models "github.com/Dja-tiger/metrics-service/internal/model"
)

// Set METRICS_TEST_DSN to a test PostgreSQL database; each run uses its own schema.
func TestPostgresDeduplication(t *testing.T) {
	dsn := os.Getenv("METRICS_TEST_DSN")
	if dsn == "" {
		t.Skip("METRICS_TEST_DSN is not configured")
	}

	admin, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := "idempotency_" + strings.ToLower(rand.Text())
	if _, err := admin.Exec("CREATE SCHEMA " + schema); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := admin.Exec("DROP SCHEMA " + schema + " CASCADE"); err != nil {
			t.Error(err)
		}
	}()
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		parsed, err := url.Parse(dsn)
		if err != nil {
			t.Fatal(err)
		}
		query := parsed.Query()
		query.Set("search_path", schema)
		parsed.RawQuery = query.Encode()
		dsn = parsed.String()
	} else {
		dsn += " search_path=" + schema
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := MigratePostgres(db); err != nil {
		t.Fatal(err)
	}
	store := NewPostgresStorage(db)
	checkDeduplication(t, store, store.GetCounter)
	// A failed transaction must not leave a receipt that would suppress the retry.
	delta := int64(1)
	max := int64(1<<63 - 1)
	bad := []models.Metrics{{ID: "rolled-back", MType: models.Counter, Delta: &delta}, {ID: "DedupCounter", MType: models.Counter, Delta: &max}}
	if _, err := store.UpdateMetricsOnce("failed-batch", "bad", bad); err == nil {
		t.Fatal("expected bigint overflow")
	}
	var exists bool
	if err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM metric_receipts WHERE request_id=$1)", "failed-batch").Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("receipt survived failed batch")
	}
	if _, ok := store.GetCounter("rolled-back"); ok {
		t.Fatal("partial batch survived rollback")
	}
	// Reopen the client to verify receipts are stored in PostgreSQL, not in the process.
	other, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	delta = 5
	applied, err := NewPostgresStorage(other).UpdateMetricsOnce("batch-one", "payload-one", []models.Metrics{{ID: "DedupCounter", MType: models.Counter, Delta: &delta}})
	if err != nil || applied {
		t.Fatalf("persistent receipt: %v %v", applied, err)
	}
}
