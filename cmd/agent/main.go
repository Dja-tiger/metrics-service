package main

import (
	"context"
	"errors"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/Dja-tiger/metrics-service/internal/agent"
	"github.com/Dja-tiger/metrics-service/internal/buildinfo"
	"github.com/Dja-tiger/metrics-service/internal/config"
	"github.com/Dja-tiger/metrics-service/internal/encryption"
	"github.com/Dja-tiger/metrics-service/internal/grpcapi"
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

	var client *grpcapi.Client
	var sender agent.BatchSender
	if cfg.GRPCAddress != "" {
		tlsConfig, tlsErr := grpcapi.LoadClientTLS(cfg.GRPCTLSCA)
		if tlsErr != nil {
			log.Fatal(tlsErr)
		}
		client, err = grpcapi.NewClient(cfg.GRPCAddress, tlsConfig)
		if err != nil {
			log.Fatal(err)
		}
		sender = client
	}
	store := agent.NewStore()
	metricsAgent, err := agent.NewAgent(
		cfg.Address,
		time.Duration(cfg.PollInterval)*time.Second,
		time.Duration(cfg.ReportInterval)*time.Second,
		agent.WithStore(store),
		agent.WithBatchSender(sender),
		agent.WithKey(cfg.Key),
		agent.WithPublicKey(publicKey),
		agent.WithRateLimit(cfg.RateLimit),
	)
	if err != nil {
		if client != nil {
			err = errors.Join(err, client.Close())
		}
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()

	err = metricsAgent.Run(ctx)
	if client != nil {
		err = errors.Join(err, client.Close())
	}
	if err != nil {
		log.Fatal(err)
	}
}
