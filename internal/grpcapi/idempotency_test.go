package grpcapi

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/Dja-tiger/metrics-service/internal/agent"
	pb "github.com/Dja-tiger/metrics-service/internal/proto"
	"github.com/Dja-tiger/metrics-service/internal/repository"
	"github.com/Dja-tiger/metrics-service/internal/service"
	"github.com/Dja-tiger/metrics-service/internal/testutil"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestLostGRPCAcknowledgementsDoNotDuplicateCounters(t *testing.T) {
	fixture := testutil.NewTLS(t)
	stored := repository.NewMemStorage()
	var mu sync.Mutex
	var ids []string
	server := grpc.NewServer(grpc.Creds(credentials.NewTLS(fixture.Server)), grpc.UnaryInterceptor(func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, next grpc.UnaryHandler) (any, error) {
		keys := metadata.ValueFromIncomingContext(ctx, "idempotency-key")
		ips := metadata.ValueFromIncomingContext(ctx, "x-real-ip")
		if len(keys) != 1 || len(ips) != 1 || net.ParseIP(ips[0]) == nil {
			t.Error("missing batch ID or source IP")
			return nil, status.Error(codes.InvalidArgument, "missing metadata")
		}
		mu.Lock()
		ids = append(ids, keys[0])
		call := len(ids)
		mu.Unlock()
		response, err := next(ctx, req)
		// Fail after storage committed, including every retry in the first report.
		if err == nil && call <= 4 {
			return nil, status.Error(codes.Unavailable, "response lost after commit")
		}
		return response, err
	}))
	pb.RegisterMetricsServer(server, &metricsServer{service: service.NewMetricsService(stored), logger: zap.NewNop()})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go server.Serve(listener)
	t.Cleanup(server.Stop)
	client, err := NewClient(listener.Addr().String(), fixture.Client)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	store := agent.NewStore()
	store.IncCounter("PollCount", 5)
	a, err := agent.NewAgent("http://localhost:8080", time.Hour, time.Hour,
		agent.WithStore(store), agent.WithBatchSender(client), agent.WithRetrySleep(func(time.Duration) {}))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.ReportOnce(); status.Code(err) != codes.Unavailable {
		t.Fatalf("expected exhausted retries: %v", err)
	}
	if got, _ := stored.GetCounter("PollCount"); got != 5 {
		t.Fatalf("four attempts applied %d instead of 5", got)
	}
	store.IncCounter("PollCount", 2)
	if err := a.ReportOnce(); err != nil {
		t.Fatal(err)
	}
	if got, _ := stored.GetCounter("PollCount"); got != 7 {
		t.Fatalf("resubmitted pending batch applied twice: %d", got)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(ids) != 6 {
		t.Fatalf("requests=%d", len(ids))
	}
	for _, id := range ids[:5] {
		if id == "" || id != ids[0] {
			t.Fatal("retry changed batch identity")
		}
	}
	if ids[5] == ids[0] {
		t.Fatal("new metrics reused old identity")
	}
}

func TestGRPCIdempotencyValidation(t *testing.T) {
	stored := repository.NewMemStorage()
	server := &metricsServer{service: service.NewMetricsService(stored), logger: zap.NewNop()}
	request := pb.UpdateMetricsRequest_builder{Metrics: []*pb.Metric{pb.Metric_builder{Id: "count", Type: pb.Metric_COUNTER, Delta: 5}.Build()}}.Build()
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("idempotency-key", "batch-1"))
	for i := 0; i < 2; i++ {
		if _, err := server.UpdateMetrics(ctx, request); err != nil {
			t.Fatal(err)
		}
	}
	request.GetMetrics()[0].SetDelta(9)
	if _, err := server.UpdateMetrics(ctx, request); status.Code(err) != codes.AlreadyExists {
		t.Fatalf("expected conflict: %v", err)
	}
	for _, keys := range [][]string{{""}, {"bad key"}, {"a", "b"}} {
		invalid := metadata.NewIncomingContext(context.Background(), metadata.MD{"idempotency-key": keys})
		if _, err := server.UpdateMetrics(invalid, request); status.Code(err) != codes.InvalidArgument {
			t.Fatalf("keys %v: %v", keys, err)
		}
	}
	if got, _ := stored.GetCounter("count"); got != 5 {
		t.Fatalf("duplicate/conflicting requests changed counter: %d", got)
	}
	for i := 0; i < 2; i++ {
		if _, err := server.UpdateMetrics(context.Background(), request); err != nil {
			t.Fatal(err)
		}
	}
	if got, _ := stored.GetCounter("count"); got != 23 {
		t.Fatalf("legacy requests must remain additive: %d", got)
	}
}
