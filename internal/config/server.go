package config

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// ServerConfig contains the effective server settings.
type ServerConfig struct {
	Address            string
	GRPCAddress        string
	StoreInterval      int
	FileStoragePath    string
	FileStorageEnabled bool
	Restore            bool
	DatabaseDSN        string
	Key                string
	CryptoKey          string
	AuditFile          string
	AuditURL           string
	TrustedSubnet      string
}

// LoadServerConfig loads defaults, JSON, explicit flags and environment, in that order.
func LoadServerConfig() (ServerConfig, error) {
	grpcFlag := flag.String("grpc-address", "", "optional gRPC server address")
	addrFlag := flag.String("a", "localhost:8080", "HTTP server address")
	storeIntervalFlag := flag.Int("i", 300, "metrics store interval in seconds")
	fileStoragePathFlag := flag.String("f", "metrics-storage.json", "metrics file storage path")
	restoreFlag := flag.Bool("r", true, "restore metrics from file storage")
	databaseDSNFlag := flag.String("d", "", "PostgreSQL connection string")
	keyFlag := flag.String("k", "", "SHA256 signing key")
	cryptoKeyFlag := flag.String("crypto-key", "", "RSA private key PEM file")
	auditFileFlag := flag.String("audit-file", "", "audit log file path")
	auditURLFlag := flag.String("audit-url", "", "audit log receiver URL")
	trustedSubnetFlag := flag.String("t", "", "trusted agent subnet in CIDR notation")
	configPath := configFileFlags()
	flag.Parse()
	if err := applyFile(*configPath, []fileOption{
		{field: "grpc_address", flag: "grpc-address", env: []string{"GRPC_ADDRESS"}},
		{field: "address", flag: "a", env: []string{"ADDRESS"}},
		{field: "store_interval", flag: "i", env: []string{"STORE_INTERVAL"}, duration: true},
		{field: "store_file", flag: "f", env: []string{"FILE_STORAGE_PATH", "STORE_FILE"}},
		{field: "restore", flag: "r", env: []string{"RESTORE"}},
		{field: "database_dsn", flag: "d", env: []string{"DATABASE_DSN"}},
		{field: "key", flag: "k", env: []string{"KEY"}},
		{field: "crypto_key", flag: "crypto-key", env: []string{"CRYPTO_KEY"}},
		{field: "audit_file", flag: "audit-file", env: []string{"AUDIT_FILE"}},
		{field: "audit_url", flag: "audit-url", env: []string{"AUDIT_URL"}},
		{field: "trusted_subnet", flag: "t", env: []string{"TRUSTED_SUBNET"}},
	}); err != nil {
		return ServerConfig{}, err
	}

	fileStorageFlagSet := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "f" {
			fileStorageFlagSet = true
		}
	})

	cfg := ServerConfig{
		GRPCAddress:        *grpcFlag,
		Address:            *addrFlag,
		StoreInterval:      *storeIntervalFlag,
		FileStoragePath:    *fileStoragePathFlag,
		FileStorageEnabled: fileStorageFlagSet && *fileStoragePathFlag != "",
		Restore:            *restoreFlag,
		DatabaseDSN:        *databaseDSNFlag,
		Key:                *keyFlag,
		CryptoKey:          *cryptoKeyFlag,
		AuditFile:          *auditFileFlag,
		AuditURL:           *auditURLFlag,
		TrustedSubnet:      *trustedSubnetFlag,
	}

	if value, ok := os.LookupEnv("GRPC_ADDRESS"); ok {
		cfg.GRPCAddress = value
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
	if value, ok := os.LookupEnv("STORE_FILE"); ok {
		cfg.FileStoragePath = value
		cfg.FileStorageEnabled = value != ""
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
	if value, ok := os.LookupEnv("CRYPTO_KEY"); ok {
		cfg.CryptoKey = value
	}
	if envAuditFile, ok := os.LookupEnv("AUDIT_FILE"); ok {
		cfg.AuditFile = envAuditFile
	}
	if envAuditURL, ok := os.LookupEnv("AUDIT_URL"); ok {
		cfg.AuditURL = envAuditURL
	}
	if value, ok := os.LookupEnv("TRUSTED_SUBNET"); ok {
		cfg.TrustedSubnet = value
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
