package config

import (
	"path/filepath"
	"testing"
)

func TestExampleConfigurations(t *testing.T) {
	t.Run("server", func(t *testing.T) {
		prepareConfig(t, []string{"-config", filepath.Join("..", "..", "server.example.json")}, nil)
		cfg, err := LoadServerConfig()
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Address != "localhost:8080" || cfg.StoreInterval != 300 || !cfg.Restore ||
			!cfg.FileStorageEnabled || cfg.FileStoragePath != "metrics-storage.json" {
			t.Fatalf("unexpected example settings: %+v", cfg)
		}
	})
	t.Run("agent", func(t *testing.T) {
		prepareConfig(t, []string{"-c", filepath.Join("..", "..", "agent.example.json")}, nil)
		cfg, err := LoadAgentConfig()
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Address != "http://localhost:8080" || cfg.ReportInterval != 10 || cfg.PollInterval != 2 || cfg.RateLimit != 1 {
			t.Fatalf("unexpected example settings: %+v", cfg)
		}
	})
}
