package main

import (
	"flag"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/Dja-tiger/metrics-service/internal/handler"
	"github.com/Dja-tiger/metrics-service/internal/repository"
	"github.com/Dja-tiger/metrics-service/internal/service"
)

func main() {
	addr := flag.String("a", "localhost:8080", "HTTP server address")
	flag.Parse()

	storage := repository.NewMemStorage()
	metricsService := service.NewMetricsService(storage)
	metricsHandler := handler.NewMetricsHandler(metricsService)

	router := chi.NewRouter()
	router.Post("/update/{type}/{name}/{value}", metricsHandler.UpdateMetric)
	router.Get("/value/{type}/{name}", metricsHandler.GetValue)
	router.Get("/", metricsHandler.ListMetrics)

	listenAddr := normalizeListenAddr(*addr)
	if err := http.ListenAndServe(listenAddr, router); err != nil {
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
