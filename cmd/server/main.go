package main

import (
	"net/http"
	"strconv"
	"strings"
)

type MemStorage struct {
	gauges   map[string]float64
	counters map[string]int64
}

func NewMemStorage() *MemStorage {
	return &MemStorage{
		gauges:   make(map[string]float64),
		counters: make(map[string]int64),
	}
}

func (s *MemStorage) UpdateGauge(name string, value float64) {
	s.gauges[name] = value
}

func (s *MemStorage) UpdateCounter(name string, value int64) {
	s.counters[name] += value
}

func main() {
	storage := NewMemStorage()

	mux := http.NewServeMux()
	mux.HandleFunc("/update/", func(w http.ResponseWriter, r *http.Request) {
		updateMetricHandler(w, r, storage)
	})

	err := http.ListenAndServe(":8080", mux)
	if err != nil {
		panic(err)
	}
}

func updateMetricHandler(w http.ResponseWriter, r *http.Request, storage *MemStorage) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")

	// ожидаем путь вида:
	// /update/<type>/<name>/<value>
	if len(parts) != 4 {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if parts[0] != "update" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	metricType := parts[1]
	metricName := parts[2]
	metricValue := parts[3]

	if metricName == "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	switch metricType {
	case "gauge":
		value, err := strconv.ParseFloat(metricValue, 64)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		storage.UpdateGauge(metricName, value)
		w.WriteHeader(http.StatusOK)

	case "counter":
		value, err := strconv.ParseInt(metricValue, 10, 64)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		storage.UpdateCounter(metricName, value)
		w.WriteHeader(http.StatusOK)

	default:
		w.WriteHeader(http.StatusBadRequest)
	}
}
