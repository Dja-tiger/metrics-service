package repository

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"strings"

	models "github.com/Dja-tiger/metrics-service/internal/model"
	"github.com/Dja-tiger/metrics-service/internal/retry"
	"github.com/jackc/pgerrcode"
	"github.com/lib/pq"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func NewPostgresDB(dsn string) (*sql.DB, error) {
	if dsn == "" {
		return nil, nil
	}

	return sql.Open("postgres", dsn)
}

func MigratePostgres(db *sql.DB) error {
	if db == nil {
		return nil
	}

	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set migration dialect: %w", err)
	}
	if err := retryPostgres(func() error {
		return goose.Up(db, "migrations")
	}); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}

type PostgresStorage struct {
	db *sql.DB
}

const upsertGaugeQuery = `
	INSERT INTO metrics (id, type, gauge_value, counter_value)
	VALUES ($1, $2, $3, NULL)
	ON CONFLICT (id) DO UPDATE
	SET type = EXCLUDED.type,
	    gauge_value = EXCLUDED.gauge_value,
	    counter_value = NULL
`

const upsertCounterQuery = `
	INSERT INTO metrics (id, type, gauge_value, counter_value)
	VALUES ($1, $2, NULL, $3)
	ON CONFLICT (id) DO UPDATE
	SET type = EXCLUDED.type,
	    gauge_value = NULL,
	    counter_value = COALESCE(metrics.counter_value, 0) + EXCLUDED.counter_value
`

func NewPostgresStorage(db *sql.DB) *PostgresStorage {
	return &PostgresStorage{db: db}
}

func (s *PostgresStorage) UpdateGauge(name string, value float64) {
	_ = retryPostgres(func() error {
		_, err := s.db.Exec(upsertGaugeQuery, name, models.Gauge, value)
		return err
	})
}

func (s *PostgresStorage) UpdateCounter(name string, value int64) {
	_ = retryPostgres(func() error {
		_, err := s.db.Exec(upsertCounterQuery, name, models.Counter, value)
		return err
	})
}

func (s *PostgresStorage) UpdateMetrics(metrics []models.Metrics) {
	_ = retryPostgres(func() error {
		return s.updateMetrics(metrics)
	})
}

func (s *PostgresStorage) updateMetrics(metrics []models.Metrics) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	for _, metric := range metrics {
		switch metric.MType {
		case models.Gauge:
			if metric.Value == nil {
				continue
			}
			if _, err = tx.Exec(upsertGaugeQuery, metric.ID, models.Gauge, *metric.Value); err != nil {
				return err
			}
		case models.Counter:
			if metric.Delta == nil {
				continue
			}
			if _, err = tx.Exec(upsertCounterQuery, metric.ID, models.Counter, *metric.Delta); err != nil {
				return err
			}
		}
	}

	if err = tx.Commit(); err == nil {
		committed = true
	}
	return err
}

func (s *PostgresStorage) GetGauge(name string) (float64, bool) {
	var value float64
	err := retryPostgres(func() error {
		return s.db.QueryRow(`
			SELECT gauge_value
			FROM metrics
			WHERE id = $1 AND type = $2 AND gauge_value IS NOT NULL
		`, name, models.Gauge).Scan(&value)
	})
	if err != nil {
		return 0, false
	}
	return value, true
}

func (s *PostgresStorage) GetCounter(name string) (int64, bool) {
	var value int64
	err := retryPostgres(func() error {
		return s.db.QueryRow(`
			SELECT counter_value
			FROM metrics
			WHERE id = $1 AND type = $2 AND counter_value IS NOT NULL
		`, name, models.Counter).Scan(&value)
	})
	if err != nil {
		return 0, false
	}
	return value, true
}

func (s *PostgresStorage) GetAllGauges() map[string]float64 {
	var gauges map[string]float64
	err := retryPostgres(func() error {
		var err error
		gauges, err = s.getAllGauges()
		return err
	})
	if err != nil {
		return map[string]float64{}
	}
	return gauges
}

func (s *PostgresStorage) getAllGauges() (map[string]float64, error) {
	rows, err := s.db.Query(`
		SELECT id, gauge_value
		FROM metrics
		WHERE type = $1 AND gauge_value IS NOT NULL
	`, models.Gauge)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	gauges := make(map[string]float64)
	for rows.Next() {
		var name string
		var value float64
		if err = rows.Scan(&name, &value); err != nil {
			return nil, err
		}
		gauges[name] = value
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return gauges, nil
}

func (s *PostgresStorage) GetAllCounters() map[string]int64 {
	var counters map[string]int64
	err := retryPostgres(func() error {
		var err error
		counters, err = s.getAllCounters()
		return err
	})
	if err != nil {
		return map[string]int64{}
	}
	return counters
}

func (s *PostgresStorage) getAllCounters() (map[string]int64, error) {
	rows, err := s.db.Query(`
		SELECT id, counter_value
		FROM metrics
		WHERE type = $1 AND counter_value IS NOT NULL
	`, models.Counter)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counters := make(map[string]int64)
	for rows.Next() {
		var name string
		var value int64
		if err = rows.Scan(&name, &value); err != nil {
			return nil, err
		}
		counters[name] = value
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return counters, nil
}

func retryPostgres(operation func() error) error {
	return retry.Do(operation, isPostgresConnectionException)
}

func isPostgresConnectionException(err error) bool {
	var pqErr *pq.Error
	if !errors.As(err, &pqErr) {
		return false
	}
	return strings.HasPrefix(string(pqErr.Code), pgerrcode.ConnectionException[:2])
}
