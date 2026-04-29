package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

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
	storeIntervalFlag := flag.Int("i", 300, "metrics store interval in seconds")
	fileStoragePathFlag := flag.String("f", "metrics-storage.json", "metrics file storage path")
	restoreFlag := flag.Bool("r", true, "restore metrics from file storage")
	flag.Parse()

	address := *addrFlag
	storeInterval := *storeIntervalFlag
	fileStoragePath := *fileStoragePathFlag
	restore := *restoreFlag

	if envAddress := os.Getenv("ADDRESS"); envAddress != "" {
		address = envAddress
	}
	if envStoreInterval := os.Getenv("STORE_INTERVAL"); envStoreInterval != "" {
		parsed, err := parseNonNegativeSeconds("STORE_INTERVAL", envStoreInterval)
		if err != nil {
			log.Fatal(err)
		}
		storeInterval = parsed
	}
	if envFileStoragePath := os.Getenv("FILE_STORAGE_PATH"); envFileStoragePath != "" {
		fileStoragePath = envFileStoragePath
	}
	if envRestore := os.Getenv("RESTORE"); envRestore != "" {
		parsed, err := strconv.ParseBool(envRestore)
		if err != nil {
			log.Fatalf("RESTORE must be boolean: %v", err)
		}
		restore = parsed
	}

	if storeInterval < 0 {
		log.Fatal("store interval must be non-negative")
	}

	storage := repository.NewMemStorage()
	if restore {
		if err = storage.LoadFromFile(fileStoragePath); err != nil {
			log.Fatal(err)
		}
	}

	metricsService := service.NewMetricsService(storage)
	if storeInterval == 0 {
		metricsService.SetSaveOnUpdate(func() {
			if err := storage.SaveToFile(fileStoragePath); err != nil {
				logger.Info("save metrics failed", zap.Error(err))
			}
		})
	} else {
		go saveMetricsPeriodically(storage, fileStoragePath, time.Duration(storeInterval)*time.Second, logger)
	}
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

	listenAddr := normalizeListenAddr(address)
	if err = http.ListenAndServe(listenAddr, router); err != nil {
		log.Fatal(err)
	}
}

func saveMetricsPeriodically(storage *repository.MemStorage, fileStoragePath string, interval time.Duration, logger *zap.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		if err := storage.SaveToFile(fileStoragePath); err != nil {
			logger.Info("save metrics failed", zap.Error(err))
		}
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

func parseNonNegativeSeconds(name, value string) (int, error) {
	seconds, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer value in seconds: %w", name, err)
	}
	if seconds < 0 {
		return 0, fmt.Errorf("%s must be non-negative", name)
	}
	return seconds, nil
}
