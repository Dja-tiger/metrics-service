package config

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// AgentConfig contains the effective agent settings.
type AgentConfig struct {
	Address        string
	ReportInterval int
	PollInterval   int
	RateLimit      int
	Key            string
	CryptoKey      string
}

// LoadAgentConfig loads defaults, JSON, explicit flags and environment, in that order.
func LoadAgentConfig() (AgentConfig, error) {
	addrFlag := flag.String("a", "localhost:8080", "HTTP server address")
	reportIntervalFlag := flag.Int("r", 10, "report interval in seconds")
	pollIntervalFlag := flag.Int("p", 2, "poll interval in seconds")
	rateLimitFlag := flag.Int("l", 1, "maximum number of concurrent requests")
	keyFlag := flag.String("k", "", "SHA256 signing key")
	cryptoKeyFlag := flag.String("crypto-key", "", "RSA public key PEM file")
	configPath := configFileFlags()
	flag.Parse()
	if err := applyFile(*configPath, []fileOption{
		{field: "address", flag: "a", env: []string{"ADDRESS"}},
		{field: "report_interval", flag: "r", env: []string{"REPORT_INTERVAL"}, duration: true},
		{field: "poll_interval", flag: "p", env: []string{"POLL_INTERVAL"}, duration: true},
		{field: "rate_limit", flag: "l", env: []string{"RATE_LIMIT"}},
		{field: "key", flag: "k", env: []string{"KEY"}},
		{field: "crypto_key", flag: "crypto-key", env: []string{"CRYPTO_KEY"}},
	}); err != nil {
		return AgentConfig{}, err
	}

	cfg := AgentConfig{
		Address:        *addrFlag,
		ReportInterval: *reportIntervalFlag,
		PollInterval:   *pollIntervalFlag,
		RateLimit:      *rateLimitFlag,
		Key:            *keyFlag,
		CryptoKey:      *cryptoKeyFlag,
	}

	if envAddress, ok := os.LookupEnv("ADDRESS"); ok {
		cfg.Address = envAddress
	}
	if envReportInterval, ok := os.LookupEnv("REPORT_INTERVAL"); ok {
		parsed, err := parsePositiveSeconds("REPORT_INTERVAL", envReportInterval)
		if err != nil {
			return AgentConfig{}, err
		}
		cfg.ReportInterval = parsed
	}
	if envPollInterval, ok := os.LookupEnv("POLL_INTERVAL"); ok {
		parsed, err := parsePositiveSeconds("POLL_INTERVAL", envPollInterval)
		if err != nil {
			return AgentConfig{}, err
		}
		cfg.PollInterval = parsed
	}
	if envKey, ok := os.LookupEnv("KEY"); ok {
		cfg.Key = envKey
	}
	if value, ok := os.LookupEnv("CRYPTO_KEY"); ok {
		cfg.CryptoKey = value
	}
	if envRateLimit, ok := os.LookupEnv("RATE_LIMIT"); ok {
		parsed, err := strconv.Atoi(envRateLimit)
		if err != nil {
			return AgentConfig{}, fmt.Errorf("RATE_LIMIT must be an integer: %w", err)
		}
		cfg.RateLimit = parsed
	}

	if cfg.PollInterval <= 0 {
		return AgentConfig{}, fmt.Errorf("poll interval must be positive")
	}
	if cfg.ReportInterval <= 0 {
		return AgentConfig{}, fmt.Errorf("report interval must be positive")
	}
	if cfg.RateLimit <= 0 {
		return AgentConfig{}, fmt.Errorf("rate limit must be positive")
	}

	if err := validateSeconds("PollInterval", cfg.PollInterval); err != nil {
		return AgentConfig{}, err
	}
	if err := validateSeconds("ReportInterval", cfg.ReportInterval); err != nil {
		return AgentConfig{}, err
	}

	cfg.Address = normalizeServerURL(cfg.Address)
	return cfg, nil
}

func normalizeServerURL(addr string) string {
	if strings.HasPrefix(addr, "http://") || strings.HasPrefix(addr, "https://") {
		return addr
	}
	return "http://" + addr
}

func parsePositiveSeconds(name, value string) (int, error) {
	seconds, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer value in seconds: %w", name, err)
	}
	if seconds <= 0 {
		return 0, fmt.Errorf("%s must be positive", name)
	}
	return seconds, nil
}
