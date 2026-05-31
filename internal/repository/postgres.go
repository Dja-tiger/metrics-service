package repository

import (
	"database/sql"
	"embed"
	"fmt"

	models "github.com/Dja-tiger/metrics-service/internal/model"
	_ "github.com/lib/pq"
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
	if err := goose.Up(db, "migrations"); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}

type PostgresStorage struct {
	db *sql.DB
}

func NewPostgresStorage(db *sql.DB) *PostgresStorage {
	return &PostgresStorage{db: db}
}

func (s *PostgresStorage) UpdateGauge(name string, value float64) {
	_, _ = s.db.Exec(`
		INSERT INTO metrics (id, type, gauge_value, counter_value)
		VALUES ($1, $2, $3, NULL)
		ON CONFLICT (id) DO UPDATE
		SET type = EXCLUDED.type,
		    gauge_value = EXCLUDED.gauge_value,
		    counter_value = NULL
	`, name, models.Gauge, value)
}

func (s *PostgresStorage) UpdateCounter(name string, value int64) {
	_, _ = s.db.Exec(`
		INSERT INTO metrics (id, type, gauge_value, counter_value)
		VALUES ($1, $2, NULL, $3)
		ON CONFLICT (id) DO UPDATE
		SET type = EXCLUDED.type,
		    gauge_value = NULL,
		    counter_value = COALESCE(metrics.counter_value, 0) + EXCLUDED.counter_value
	`, name, models.Counter, value)
}

func (s *PostgresStorage) GetGauge(name string) (float64, bool) {
	var value float64
	err := s.db.QueryRow(`
		SELECT gauge_value
		FROM metrics
		WHERE id = $1 AND type = $2 AND gauge_value IS NOT NULL
	`, name, models.Gauge).Scan(&value)
	if err != nil {
		return 0, false
	}
	return value, true
}

func (s *PostgresStorage) GetCounter(name string) (int64, bool) {
	var value int64
	err := s.db.QueryRow(`
		SELECT counter_value
		FROM metrics
		WHERE id = $1 AND type = $2 AND counter_value IS NOT NULL
	`, name, models.Counter).Scan(&value)
	if err != nil {
		return 0, false
	}
	return value, true
}

func (s *PostgresStorage) GetAllGauges() map[string]float64 {
	rows, err := s.db.Query(`
		SELECT id, gauge_value
		FROM metrics
		WHERE type = $1 AND gauge_value IS NOT NULL
	`, models.Gauge)
	if err != nil {
		return map[string]float64{}
	}
	defer rows.Close()

	gauges := make(map[string]float64)
	for rows.Next() {
		var name string
		var value float64
		if err = rows.Scan(&name, &value); err != nil {
			return gauges
		}
		gauges[name] = value
	}
	return gauges
}

func (s *PostgresStorage) GetAllCounters() map[string]int64 {
	rows, err := s.db.Query(`
		SELECT id, counter_value
		FROM metrics
		WHERE type = $1 AND counter_value IS NOT NULL
	`, models.Counter)
	if err != nil {
		return map[string]int64{}
	}
	defer rows.Close()

	counters := make(map[string]int64)
	for rows.Next() {
		var name string
		var value int64
		if err = rows.Scan(&name, &value); err != nil {
			return counters
		}
		counters[name] = value
	}
	return counters
}
