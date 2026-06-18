package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"
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
		agent.WithStore(store),
		agent.WithKey(cfg.Key),
		agent.WithRateLimit(cfg.RateLimit),
	)
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go metricsAgent.PollLoop(ctx)
	go metricsAgent.SystemPollLoop(ctx)
	metricsAgent.ReportLoop(ctx)
}
