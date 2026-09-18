package repository

import (
	"database/sql"
	"fmt"

	"github.com/Dja-tiger/metrics-service/internal/delivery"
	models "github.com/Dja-tiger/metrics-service/internal/model"
)

// IdempotentRepository commits the receipt and metric changes atomically.
// applied is false when a matching receipt already exists.
type IdempotentRepository interface {
	UpdateMetricsOnce(key, fingerprint string, metrics []models.Metrics) (applied bool, err error)
}

// UpdateMetricsOnce serializes duplicate checks with in-memory updates.
func (s *MemStorage) UpdateMetricsOnce(key, fingerprint string, metrics []models.Metrics) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if previous, ok := s.receipts[key]; ok {
		if previous != fingerprint {
			return false, delivery.ErrConflict
		}
		return false, nil
	}
	s.updateMetricsLocked(metrics)
	s.receipts[key] = fingerprint
	return true, nil
}

// UpdateMetricsOnce inserts the receipt and updates metrics in the same transaction.
func (s *PostgresStorage) UpdateMetricsOnce(key, fingerprint string, metrics []models.Metrics) (bool, error) {
	var applied bool
	err := retryPostgres(func() error {
		var err error
		applied, err = s.updateMetricsOnce(key, fingerprint, metrics)
		return err
	})
	return applied, err
}

func (s *PostgresStorage) updateMetricsOnce(key, fingerprint string, metrics []models.Metrics) (bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return false, fmt.Errorf("begin idempotent batch: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.Exec("INSERT INTO metric_receipts (request_id, fingerprint) VALUES ($1,$2) ON CONFLICT DO NOTHING", key, fingerprint)
	if err != nil {
		return false, fmt.Errorf("insert batch receipt: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if n == 0 {
		var previous string
		if err := tx.QueryRow("SELECT fingerprint FROM metric_receipts WHERE request_id=$1", key).Scan(&previous); err != nil {
			return false, err
		}
		if previous != fingerprint {
			return false, delivery.ErrConflict
		}
		return false, nil
	}
	if err := updateMetricsTx(tx, metrics); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit idempotent batch: %w", err)
	}
	return true, nil
}

func updateMetricsTx(tx *sql.Tx, metrics []models.Metrics) error {
	for _, metric := range metrics {
		switch metric.MType {
		case models.Gauge:
			if metric.Value != nil {
				if _, err := tx.Exec(upsertGaugeQuery, metric.ID, models.Gauge, *metric.Value); err != nil {
					return err
				}
			}
		case models.Counter:
			if metric.Delta != nil {
				if _, err := tx.Exec(upsertCounterQuery, metric.ID, models.Counter, *metric.Delta); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
