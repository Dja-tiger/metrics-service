package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/Dja-tiger/metrics-service/internal/audit"
	"github.com/Dja-tiger/metrics-service/internal/buildinfo"
	"github.com/Dja-tiger/metrics-service/internal/config"
	"github.com/Dja-tiger/metrics-service/internal/encryption"
	"github.com/Dja-tiger/metrics-service/internal/grpcapi"
	"github.com/Dja-tiger/metrics-service/internal/handler"
	appmiddleware "github.com/Dja-tiger/metrics-service/internal/middleware"
	"github.com/Dja-tiger/metrics-service/internal/repository"
	"github.com/Dja-tiger/metrics-service/internal/server"
	"github.com/Dja-tiger/metrics-service/internal/service"
)

var (
	buildVersion = "N/A"
	buildDate    = "N/A"
	buildCommit  = "N/A"
)

func main() {
	buildinfo.Print(buildVersion, buildDate, buildCommit)

	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		_ = logger.Sync()
	}()

	cfg, err := config.LoadServerConfig()
	if err != nil {
		log.Fatal(err)
	}

	trustedSubnet, err := appmiddleware.TrustedSubnet(cfg.TrustedSubnet)
	if err != nil {
		log.Fatal(err)
	}

	privateKey, err := encryption.LoadPrivateKey(cfg.CryptoKey)
	if err != nil {
		log.Fatal(err)
	}

	db, err := repository.NewPostgresDB(cfg.DatabaseDSN)
	if err != nil {
		log.Fatal(err)
	}
	if db != nil {
		defer db.Close()
	}

	var metricsService *service.MetricsService
	if db != nil {
		if err = repository.MigratePostgres(db); err != nil {
			log.Fatal(err)
		}
		metricsService = service.NewMetricsService(repository.NewPostgresStorage(db))
	} else if cfg.FileStorageEnabled {
		storage, err := repository.NewMemStorageWithRestore(cfg.FileStoragePath, cfg.Restore)
		if err != nil {
			log.Fatal(err)
		}
		metricsService = service.NewMetricsServiceWithPersistence(storage, storage, cfg.FileStoragePath, time.Duration(cfg.StoreInterval)*time.Second, func(err error) {
			logger.Info("save metrics failed", zap.Error(err))
		})
	} else {
		metricsService = service.NewMetricsService(repository.NewMemStorage())
	}

	auditNotifier := audit.NewNotifier()
	if cfg.AuditFile != "" {
		auditNotifier.Subscribe(audit.NewFileObserver(cfg.AuditFile))
	}
	if cfg.AuditURL != "" {
		auditNotifier.Subscribe(audit.NewHTTPObserver(cfg.AuditURL, nil))
	}

	var auditor handler.AuditPublisher
	if auditNotifier.Enabled() {
		auditor = auditNotifier
	}

	metricsHandler := handler.NewMetricsHandlerWithDBAndAudit(metricsService, db, auditor)

	router := chi.NewRouter()
	router.Use(appmiddleware.RequestLogger(logger))
	router.Use(trustedSubnet)
	if privateKey != nil {
		router.Use(appmiddleware.Decrypt(privateKey))
	}
	router.Use(appmiddleware.Gzip)
	router.Use(appmiddleware.HashSHA256(cfg.Key))
	router.Post("/update/{type}/{name}/{value}", metricsHandler.UpdateMetric)
	router.Post("/update", metricsHandler.UpdateMetricJSON)
	router.Post("/update/", metricsHandler.UpdateMetricJSON)
	router.Post("/updates", metricsHandler.UpdateMetricsJSON)
	router.Post("/updates/", metricsHandler.UpdateMetricsJSON)
	router.Get("/value/{type}/{name}", metricsHandler.GetValue)
	router.Post("/value", metricsHandler.GetValueJSON)
	router.Post("/value/", metricsHandler.GetValueJSON)
	router.Get("/ping", metricsHandler.Ping)
	router.Get("/", metricsHandler.ListMetrics)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()
	var grpcServer *grpc.Server
	if cfg.GRPCAddress != "" {
		grpcServer, err = grpcapi.NewServer(metricsService, cfg.TrustedSubnet, auditor, logger)
		if err != nil {
			log.Fatal(err)
		}
	}
	srv := &http.Server{Addr: cfg.Address, Handler: router}
	if err = server.RunWithGRPC(ctx, srv, grpcServer, cfg.GRPCAddress, metricsService.Close); err != nil {
		if db != nil {
			_ = db.Close()
		}
		_ = logger.Sync()
		log.Fatal(err)
	}
}
