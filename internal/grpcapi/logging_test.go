package grpcapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Dja-tiger/metrics-service/internal/audit"
	pb "github.com/Dja-tiger/metrics-service/internal/proto"
	"github.com/Dja-tiger/metrics-service/internal/repository"
	"github.com/Dja-tiger/metrics-service/internal/service"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestAuditFailureLoggedAsWarning(t *testing.T) {
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer receiver.Close()
	core, logs := observer.New(zap.DebugLevel)
	store := repository.NewMemStorage()
	server := &metricsServer{
		service: service.NewMetricsService(store), logger: zap.New(core),
		auditor: audit.NewNotifier(audit.NewHTTPObserver(receiver.URL, receiver.Client())),
	}
	request := pb.UpdateMetricsRequest_builder{Metrics: []*pb.Metric{
		pb.Metric_builder{Id: "count", Type: pb.Metric_COUNTER, Delta: 1}.Build(),
	}}.Build()
	if _, err := server.UpdateMetrics(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if logs.FilterMessage("grpc audit failed").FilterLevelExact(zap.WarnLevel).Len() != 1 {
		t.Fatal("audit failure was not logged at Warn level")
	}
	if got, _ := store.GetCounter("count"); got != 1 {
		t.Fatalf("audit failure lost stored metric: %d", got)
	}
}
