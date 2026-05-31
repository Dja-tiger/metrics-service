package main

import (
	"log"
	"time"

	"github.com/Dja-tiger/metrics-service/internal/agent"
	"github.com/Dja-tiger/metrics-service/internal/config"
)

func main() {
	cfg, err := config.LoadAgentConfig()
	if err != nil {
		log.Fatal(err)
	}

	store := agent.NewStore()
	metricsAgent, err := agent.NewAgent(
		cfg.Address,
		time.Duration(cfg.PollInterval)*time.Second,
		time.Duration(cfg.ReportInterval)*time.Second,
		nil,
		store,
	)
	if err != nil {
		log.Fatal(err)
	}

	go metricsAgent.PollLoop()
	metricsAgent.ReportLoop()
}
