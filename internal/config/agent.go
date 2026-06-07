package config

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type AgentConfig struct {
	Address        string
	ReportInterval int
	PollInterval   int
	Key            string
}

func LoadAgentConfig() (AgentConfig, error) {
	addrFlag := flag.String("a", "localhost:8080", "HTTP server address")
	reportIntervalFlag := flag.Int("r", 10, "report interval in seconds")
	pollIntervalFlag := flag.Int("p", 2, "poll interval in seconds")
	keyFlag := flag.String("k", "", "SHA256 signing key")
	flag.Parse()

	cfg := AgentConfig{
		Address:        *addrFlag,
		ReportInterval: *reportIntervalFlag,
		PollInterval:   *pollIntervalFlag,
		Key:            *keyFlag,
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

	if cfg.PollInterval <= 0 {
		return AgentConfig{}, fmt.Errorf("poll interval must be positive")
	}
	if cfg.ReportInterval <= 0 {
		return AgentConfig{}, fmt.Errorf("report interval must be positive")
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
