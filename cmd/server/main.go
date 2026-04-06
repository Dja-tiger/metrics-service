package main

import (
	"net/http"

	"github.com/Dja-tiger/metrics-service/internal/handler"
	"github.com/Dja-tiger/metrics-service/internal/repository"
	"github.com/Dja-tiger/metrics-service/internal/service"
)

func main() {
	storage := repository.NewMemStorage()
	metricsService := service.NewMetricsService(storage)
	metricsHandler := handler.NewMetricsHandler(metricsService)

	mux := http.NewServeMux()
	mux.HandleFunc("/update/", metricsHandler.UpdateMetric)

	err := http.ListenAndServe(":8080", mux)
	if err != nil {
		panic(err)
	}
}
