package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Dja-tiger/metrics-service/internal/agent"
)

func main() {
	addrFlag := flag.String("a", "localhost:8080", "HTTP server address")
	reportIntervalFlag := flag.Int("r", 10, "report interval in seconds")
	pollIntervalFlag := flag.Int("p", 2, "poll interval in seconds")
	flag.Parse()

	rand.Seed(time.Now().UnixNano())

	address := *addrFlag
	reportInterval := *reportIntervalFlag
	pollInterval := *pollIntervalFlag

	if envAddress := os.Getenv("ADDRESS"); envAddress != "" {
		address = envAddress
	}
	if envReportInterval := os.Getenv("REPORT_INTERVAL"); envReportInterval != "" {
		parsed, err := parsePositiveSeconds("REPORT_INTERVAL", envReportInterval)
		if err != nil {
			log.Fatal(err)
		}
		reportInterval = parsed
	}
	if envPollInterval := os.Getenv("POLL_INTERVAL"); envPollInterval != "" {
		parsed, err := parsePositiveSeconds("POLL_INTERVAL", envPollInterval)
		if err != nil {
			log.Fatal(err)
		}
		pollInterval = parsed
	}

	if pollInterval <= 0 {
		log.Fatal("poll interval must be positive")
	}
	if reportInterval <= 0 {
		log.Fatal("report interval must be positive")
	}

	serverURL := normalizeServerURL(address)
	store := agent.NewStore()
	metricsAgent, err := agent.NewAgent(
		serverURL,
		time.Duration(pollInterval)*time.Second,
		time.Duration(reportInterval)*time.Second,
		nil,
		store,
	)
	if err != nil {
		log.Fatal(err)
	}

	go metricsAgent.PollLoop()
	metricsAgent.ReportLoop()
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
