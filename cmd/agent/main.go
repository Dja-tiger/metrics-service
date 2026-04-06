package main

import (
	"math/rand"
	"time"

	"github.com/Dja-tiger/metrics-service/internal/agent"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	store := agent.NewStore()
	metricsAgent := agent.NewAgent("http://localhost:8080", 2*time.Second, 10*time.Second, nil, store)

	go metricsAgent.PollLoop()
	go metricsAgent.ReportLoop()

	select {}
}
