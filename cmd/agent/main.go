package main

import (
	"flag"
	"math/rand"
	"strings"
	"time"

	"github.com/Dja-tiger/metrics-service/internal/agent"
)

func main() {
	addr := flag.String("a", "localhost:8080", "HTTP server address")
	reportInterval := flag.Int("r", 10, "report interval in seconds")
	pollInterval := flag.Int("p", 2, "poll interval in seconds")
	flag.Parse()

	rand.Seed(time.Now().UnixNano())

	serverURL := normalizeServerURL(*addr)
	store := agent.NewStore()
	metricsAgent := agent.NewAgent(
		serverURL,
		time.Duration(*pollInterval)*time.Second,
		time.Duration(*reportInterval)*time.Second,
		nil,
		store,
	)

	go metricsAgent.PollLoop()
	go metricsAgent.ReportLoop()

	select {}
}

func normalizeServerURL(addr string) string {
	if strings.HasPrefix(addr, "http://") || strings.HasPrefix(addr, "https://") {
		return addr
	}
	return "http://" + addr
}
