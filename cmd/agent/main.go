package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/Dja-tiger/metrics-service/internal/agent"
	"github.com/Dja-tiger/metrics-service/internal/buildinfo"
	"github.com/Dja-tiger/metrics-service/internal/config"
	"github.com/Dja-tiger/metrics-service/internal/encryption"
)

var (
	buildVersion = "N/A"
	buildDate    = "N/A"
	buildCommit  = "N/A"
)

func main() {
	buildinfo.Print(buildVersion, buildDate, buildCommit)

	cfg, err := config.LoadAgentConfig()
	if err != nil {
		log.Fatal(err)
	}

	publicKey, err := encryption.LoadPublicKey(cfg.CryptoKey)
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
		agent.WithPublicKey(publicKey),
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
