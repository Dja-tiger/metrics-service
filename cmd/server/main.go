package main

import (
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/Dja-tiger/metrics-service/internal/config"
	"github.com/Dja-tiger/metrics-service/internal/handler"
	appmiddleware "github.com/Dja-tiger/metrics-service/internal/middleware"
	"github.com/Dja-tiger/metrics-service/internal/repository"
	"github.com/Dja-tiger/metrics-service/internal/service"
)

func main() {
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

	storage, err := repository.NewMemStorageWithRestore(cfg.FileStoragePath, cfg.Restore)
	if err != nil {
		log.Fatal(err)
	}
	metricsService := service.NewMetricsService(storage)
	service.ConfigurePersistence(metricsService, storage, cfg.FileStoragePath, time.Duration(cfg.StoreInterval)*time.Second, func(err error) {
		logger.Info("save metrics failed", zap.Error(err))
	})
	metricsHandler := handler.NewMetricsHandler(metricsService)

	router := chi.NewRouter()
	router.Use(appmiddleware.RequestLogger(logger))
	router.Use(appmiddleware.Gzip)
	router.Post("/update/{type}/{name}/{value}", metricsHandler.UpdateMetric)
	router.Post("/update", metricsHandler.UpdateMetricJSON)
	router.Post("/update/", metricsHandler.UpdateMetricJSON)
	router.Get("/value/{type}/{name}", metricsHandler.GetValue)
	router.Post("/value", metricsHandler.GetValueJSON)
	router.Post("/value/", metricsHandler.GetValueJSON)
	router.Get("/", metricsHandler.ListMetrics)

	if err = http.ListenAndServe(cfg.Address, router); err != nil {
		log.Fatal(err)
	}
}
