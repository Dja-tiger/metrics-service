package agent

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Dja-tiger/metrics-service/internal/encryption"
	"github.com/Dja-tiger/metrics-service/internal/handler"
	"github.com/Dja-tiger/metrics-service/internal/middleware"
	models "github.com/Dja-tiger/metrics-service/internal/model"
	"github.com/Dja-tiger/metrics-service/internal/repository"
	"github.com/Dja-tiger/metrics-service/internal/service"
	"github.com/go-chi/chi/v5"
)

func TestEncryptedBatchWithGzipSignatureAndRetry(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	storage := repository.NewMemStorage()
	h := handler.NewMetricsHandler(service.NewMetricsService(storage))
	router := chi.NewRouter()
	router.Use(middleware.Decrypt(key))
	router.Use(middleware.Gzip)
	router.Use(middleware.HashSHA256("secret"))
	router.Post("/updates/", h.UpdateMetricsJSON)
	srv := httptest.NewServer(router)
	defer srv.Close()
	attempts := 0
	var firstBody []byte
	client := srv.Client()
	transport := client.Transport
	client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		attempts++
		if r.Header.Get(encryption.Header) != encryption.Algorithm {
			t.Error("missing encryption header")
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, err
		}
		_ = r.Body.Close()
		if bytes.Contains(body, []byte("PollCount")) || bytes.HasPrefix(body, []byte{0x1f, 0x8b}) {
			t.Error("plaintext or gzip leaked on wire")
		}
		if attempts == 1 {
			firstBody = bytes.Clone(body)
			return nil, errors.New("temporary connection failure")
		}
		if !bytes.Equal(firstBody, body) {
			t.Error("retry body changed")
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		return transport.RoundTrip(r)
	})
	a, err := NewAgent(srv.URL, time.Second, time.Second, WithPublicKey(&key.PublicKey), WithKey("secret"), WithHTTPClient(client), WithRetrySleep(func(time.Duration) {}))
	if err != nil {
		t.Fatal(err)
	}
	value, delta := 12.5, int64(7)
	if err := a.sendMetrics([]models.Metrics{{ID: "Alloc", MType: models.Gauge, Value: &value}, {ID: "PollCount", MType: models.Counter, Delta: &delta}}); err != nil {
		t.Fatal(err)
	}
	if attempts != 2 {
		t.Fatalf("attempts: %d", attempts)
	}
	if got, ok := storage.GetGauge("Alloc"); !ok || got != value {
		t.Fatalf("gauge: %v %v", got, ok)
	}
	if got, ok := storage.GetCounter("PollCount"); !ok || got != delta {
		t.Fatalf("counter: %v %v", got, ok)
	}
}
