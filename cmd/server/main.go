package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/Dja-tiger/metrics-service/internal/handler"
	appmiddleware "github.com/Dja-tiger/metrics-service/internal/middleware"
	"github.com/Dja-tiger/metrics-service/internal/repository"
	"github.com/Dja-tiger/metrics-service/internal/service"
)

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}
	defer func() {
		_ = logger.Sync()
	}()

	addrFlag := flag.String("a", "localhost:8080", "HTTP server address")
	flag.Parse()

	address := *addrFlag
	if envAddress := os.Getenv("ADDRESS"); envAddress != "" {
		address = envAddress
	}

	storage := repository.NewMemStorage()
	metricsService := service.NewMetricsService(storage)
	metricsHandler := handler.NewMetricsHandler(metricsService)

	router := chi.NewRouter()
	router.Use(appmiddleware.RequestLogger(logger))
	router.Post("/update/{type}/{name}/{value}", metricsHandler.UpdateMetric)
	router.Post("/update", metricsHandler.UpdateMetricJSON)
	router.Get("/value/{type}/{name}", metricsHandler.GetValue)
	router.Post("/value", metricsHandler.GetValueJSON)
	router.Get("/", metricsHandler.ListMetrics)

	listenAddr := normalizeListenAddr(address)
	if err = http.ListenAndServe(listenAddr, router); err != nil {
		log.Fatal(err)
	}
}

func normalizeListenAddr(addr string) string {
	if strings.HasPrefix(addr, "http://") {
		return strings.TrimPrefix(addr, "http://")
	}
	if strings.HasPrefix(addr, "https://") {
		return strings.TrimPrefix(addr, "https://")
	}
	return addr
}
