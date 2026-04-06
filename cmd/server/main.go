package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Dja-tiger/metrics-service/internal/handler"
	"github.com/Dja-tiger/metrics-service/internal/repository"
	"github.com/Dja-tiger/metrics-service/internal/service"
)

func main() {
	storage := repository.NewMemStorage()
	metricsService := service.NewMetricsService(storage)
	metricsHandler := handler.NewMetricsHandler(metricsService)

	router := chi.NewRouter()
	router.Post("/update/{type}/{name}/{value}", metricsHandler.UpdateMetric)
	router.Get("/value/{type}/{name}", metricsHandler.GetValue)
	router.Get("/", metricsHandler.ListMetrics)

	err := http.ListenAndServe(":8080", router)
	if err != nil {
		panic(err)
	}
}
