package config

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// ServerConfig contains command-line and environment settings for the server.
type ServerConfig struct {
	Address            string
	StoreInterval      int
	FileStoragePath    string
	FileStorageEnabled bool
	Restore            bool
	DatabaseDSN        string
	Key                string
	AuditFile          string
	AuditURL           string
}

// LoadServerConfig parses server flags and environment variables.
func LoadServerConfig() (ServerConfig, error) {
	addrFlag := flag.String("a", "localhost:8080", "HTTP server address")
	storeIntervalFlag := flag.Int("i", 300, "metrics store interval in seconds")
	fileStoragePathFlag := flag.String("f", "metrics-storage.json", "metrics file storage path")
	restoreFlag := flag.Bool("r", true, "restore metrics from file storage")
	databaseDSNFlag := flag.String("d", "", "PostgreSQL connection string")
	keyFlag := flag.String("k", "", "SHA256 signing key")
	auditFileFlag := flag.String("audit-file", "", "audit log file path")
	auditURLFlag := flag.String("audit-url", "", "audit log receiver URL")
	flag.Parse()

	fileStorageFlagSet := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "f" {
			fileStorageFlagSet = true
		}
	})

	cfg := ServerConfig{
		Address:            *addrFlag,
		StoreInterval:      *storeIntervalFlag,
		FileStoragePath:    *fileStoragePathFlag,
		FileStorageEnabled: fileStorageFlagSet && *fileStoragePathFlag != "",
		Restore:            *restoreFlag,
		DatabaseDSN:        *databaseDSNFlag,
		Key:                *keyFlag,
		AuditFile:          *auditFileFlag,
		AuditURL:           *auditURLFlag,
	}

	if envAddress, ok := os.LookupEnv("ADDRESS"); ok {
		cfg.Address = envAddress
	}
	if envStoreInterval, ok := os.LookupEnv("STORE_INTERVAL"); ok {
		parsed, err := parseNonNegativeSeconds("STORE_INTERVAL", envStoreInterval)
		if err != nil {
			return ServerConfig{}, err
		}
		cfg.StoreInterval = parsed
	}
	if envFileStoragePath, ok := os.LookupEnv("FILE_STORAGE_PATH"); ok {
		cfg.FileStoragePath = envFileStoragePath
		cfg.FileStorageEnabled = envFileStoragePath != ""
	}
	if envRestore, ok := os.LookupEnv("RESTORE"); ok {
		parsed, err := strconv.ParseBool(envRestore)
		if err != nil {
			return ServerConfig{}, fmt.Errorf("RESTORE must be boolean: %w", err)
		}
		cfg.Restore = parsed
	}
	if envDatabaseDSN, ok := os.LookupEnv("DATABASE_DSN"); ok {
		cfg.DatabaseDSN = envDatabaseDSN
	}
	if envKey, ok := os.LookupEnv("KEY"); ok {
		cfg.Key = envKey
	}
	if envAuditFile, ok := os.LookupEnv("AUDIT_FILE"); ok {
		cfg.AuditFile = envAuditFile
	}
	if envAuditURL, ok := os.LookupEnv("AUDIT_URL"); ok {
		cfg.AuditURL = envAuditURL
	}

	if cfg.StoreInterval < 0 {
		return ServerConfig{}, fmt.Errorf("store interval must be non-negative")
	}

	cfg.Address = normalizeListenAddr(cfg.Address)
	return cfg, nil
}

func normalizeListenAddr(addr string) string {
	if strings.HasPrefix(addr, "http://") {
		return strings.TrimPrefix(addr, "http://")
	}
	if strings.HasPrefix(addr, "https://") {
		return strings.TrimPrefix(addr, "https://")
	}
	return addr
}

func parseNonNegativeSeconds(name, value string) (int, error) {
	seconds, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer value in seconds: %w", name, err)
	}
	if seconds < 0 {
		return 0, fmt.Errorf("%s must be non-negative", name)
	}
	return seconds, nil
}
