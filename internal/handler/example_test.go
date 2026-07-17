package handler_test

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/Dja-tiger/metrics-service/internal/handler"
	"github.com/Dja-tiger/metrics-service/internal/repository"
	"github.com/Dja-tiger/metrics-service/internal/service"
)

func ExampleMetricsHandler_UpdateMetric() {
	metricsHandler := newExampleHandler()

	router := chi.NewRouter()
	router.Post("/update/{type}/{name}/{value}", metricsHandler.UpdateMetric)
	router.Get("/value/{type}/{name}", metricsHandler.GetValue)

	updateRequest := httptest.NewRequest(http.MethodPost, "/update/gauge/Alloc/42.5", nil)
	updateResponse := httptest.NewRecorder()
	router.ServeHTTP(updateResponse, updateRequest)

	valueRequest := httptest.NewRequest(http.MethodGet, "/value/gauge/Alloc", nil)
	valueResponse := httptest.NewRecorder()
	router.ServeHTTP(valueResponse, valueRequest)

	fmt.Println(updateResponse.Code)
	fmt.Println(valueResponse.Code)
	fmt.Println(valueResponse.Body.String())

	// Output:
	// 200
	// 200
	// 42.5
}

func ExampleMetricsHandler_UpdateMetricJSON() {
	metricsHandler := newExampleHandler()

	router := chi.NewRouter()
	router.Post("/update/", metricsHandler.UpdateMetricJSON)

	body := bytes.NewBufferString(`{"id":"Alloc","type":"gauge","value":42.5}`)
	request := httptest.NewRequest(http.MethodPost, "/update/", body)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	fmt.Println(response.Code)
	fmt.Println(strings.TrimSpace(response.Body.String()))

	// Output:
	// 200
	// {"id":"Alloc","type":"gauge","value":42.5}
}

func ExampleMetricsHandler_UpdateMetricsJSON() {
	metricsHandler := newExampleHandler()

	router := chi.NewRouter()
	router.Post("/updates/", metricsHandler.UpdateMetricsJSON)
	router.Post("/value/", metricsHandler.GetValueJSON)

	updateBody := bytes.NewBufferString(`[{"id":"Alloc","type":"gauge","value":42.5},{"id":"PollCount","type":"counter","delta":3}]`)
	updateRequest := httptest.NewRequest(http.MethodPost, "/updates/", updateBody)
	updateRequest.Header.Set("Content-Type", "application/json")
	updateResponse := httptest.NewRecorder()
	router.ServeHTTP(updateResponse, updateRequest)

	valueBody := bytes.NewBufferString(`{"id":"PollCount","type":"counter"}`)
	valueRequest := httptest.NewRequest(http.MethodPost, "/value/", valueBody)
	valueRequest.Header.Set("Content-Type", "application/json")
	valueResponse := httptest.NewRecorder()
	router.ServeHTTP(valueResponse, valueRequest)

	fmt.Println(updateResponse.Code)
	fmt.Println(valueResponse.Code)
	fmt.Println(strings.TrimSpace(valueResponse.Body.String()))

	// Output:
	// 200
	// 200
	// {"id":"PollCount","type":"counter","delta":3}
}

func ExampleMetricsHandler_ListMetrics() {
	metricsHandler := newExampleHandler()

	router := chi.NewRouter()
	router.Post("/update/{type}/{name}/{value}", metricsHandler.UpdateMetric)
	router.Get("/", metricsHandler.ListMetrics)

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/update/gauge/Alloc/42.5", nil))
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/update/counter/PollCount/3", nil))

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	body := response.Body.String()
	fmt.Println(response.Code)
	fmt.Println(strings.Contains(body, "Alloc: 42.5"))
	fmt.Println(strings.Contains(body, "PollCount: 3"))

	// Output:
	// 200
	// true
	// true
}

func newExampleHandler() *handler.MetricsHandler {
	storage := repository.NewMemStorage()
	metricsService := service.NewMetricsService(storage)
	return handler.NewMetricsHandler(metricsService)
}
