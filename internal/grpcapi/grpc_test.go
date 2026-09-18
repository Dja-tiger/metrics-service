package grpcapi

import (
	"context"
	"math"
	"net"
	"sync"
	"testing"

	models "github.com/Dja-tiger/metrics-service/internal/model"
	pb "github.com/Dja-tiger/metrics-service/internal/proto"
	"github.com/Dja-tiger/metrics-service/internal/repository"
	"github.com/Dja-tiger/metrics-service/internal/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestNetworkBatch(t *testing.T) {
	for _, subnet := range []string{"", "127.0.0.0/8", "192.0.2.0/24"} {
		t.Run(subnet, func(t *testing.T) {
			store := repository.NewMemStorage()
			server, err := NewServer(service.NewMetricsService(store), subnet, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			go server.Serve(listener)
			t.Cleanup(server.Stop)
			client, err := NewClient(listener.Addr().String())
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = client.Close() })
			value, delta := 0.0, int64(2)
			batch := []models.Metrics{{ID: "zero", MType: models.Gauge, Value: &value}, {ID: "count", MType: models.Counter, Delta: &delta}}
			var wg sync.WaitGroup
			for i := 0; i < 4; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					err := client.Send(batch)
					if subnet == "192.0.2.0/24" {
						if status.Code(err) != codes.PermissionDenied {
							t.Errorf("denied: %v", err)
						}
					} else if err != nil {
						t.Errorf("send: %v", err)
					}
				}()
			}
			wg.Wait()
			if subnet == "192.0.2.0/24" {
				if _, ok := store.GetCounter("count"); ok {
					t.Fatal("denied batch stored")
				}
				return
			}
			if got, ok := store.GetCounter("count"); !ok || got != 8 {
				t.Fatalf("counter %d %v", got, ok)
			}
			if got, ok := store.GetGauge("zero"); !ok || got != 0 {
				t.Fatalf("gauge %f %v", got, ok)
			}
			// Raw clients without metadata must be rejected when the subnet is configured.
			conn, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			_, err = pb.NewMetricsClient(conn).UpdateMetrics(context.Background(), &pb.UpdateMetricsRequest{})
			if subnet != "" && status.Code(err) != codes.PermissionDenied {
				t.Fatalf("missing IP: %v", err)
			}
		})
	}
}

func TestSubnet(t *testing.T) {
	if _, err := TrustedSubnet("bad"); err == nil {
		t.Fatal("invalid CIDR accepted")
	}
	tests := []struct {
		subnet  string
		ips     []string
		allowed bool
	}{
		{"", nil, true}, {"127.0.0.0/8", nil, false}, {"127.0.0.0/8", []string{"bad"}, false},
		{"127.0.0.0/8", []string{"127.0.0.1", "127.0.0.2"}, false},
		{"127.0.0.0/8", []string{"127.0.0.1"}, true},
		{"2001:db8::/32", []string{"2001:db8::1"}, true},
		{"2001:db8::/32", []string{"2001:db9::1"}, false},
	}
	for _, tc := range tests {
		interceptor, err := TrustedSubnet(tc.subnet)
		if err != nil {
			t.Fatal(err)
		}
		ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{"x-real-ip": tc.ips})
		called := false
		_, err = interceptor(ctx, nil, &grpc.UnaryServerInfo{}, func(context.Context, any) (any, error) { called = true; return nil, nil })
		if called != tc.allowed {
			t.Fatalf("%+v called=%v", tc, called)
		}
		if !tc.allowed && status.Code(err) != codes.PermissionDenied {
			t.Fatal(err)
		}
	}
}

func TestBatchValidation(t *testing.T) {
	store := repository.NewMemStorage()
	server := &metricsServer{service: service.NewMetricsService(store)}
	for _, item := range []*pb.Metric{nil, {Id: ""}, {Id: "bad", Type: 99}, {Id: "nan", Value: math.NaN()}, {Id: "inf", Value: math.Inf(1)}} {
		_, err := server.UpdateMetrics(context.Background(), &pb.UpdateMetricsRequest{Metrics: []*pb.Metric{{Id: "valid", Value: 10}, item}})
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("expected invalid: %v", err)
		}
		if _, ok := store.GetGauge("valid"); ok {
			t.Fatal("partial batch written")
		}
	}
	if _, err := server.UpdateMetrics(context.Background(), nil); status.Code(err) != codes.InvalidArgument {
		t.Fatal(err)
	}
	if _, err := server.UpdateMetrics(context.Background(), &pb.UpdateMetricsRequest{}); err != nil {
		t.Fatal(err)
	}
	// Empty sends must not touch the connection.
	if err := (&Client{}).Send(nil); err != nil {
		t.Fatal(err)
	}
}

func TestStorageFailureReturnsInternal(t *testing.T) {
	db, err := repository.NewPostgresDB("host=localhost dbname=unused")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	server, err := NewServer(service.NewMetricsService(repository.NewPostgresStorage(db)), "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go server.Serve(listener)
	t.Cleanup(server.Stop)
	client, err := NewClient(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	delta := int64(5)
	err = client.Send([]models.Metrics{{ID: "PollCount", MType: models.Counter, Delta: &delta}})
	if status.Code(err) != codes.Internal {
		t.Fatalf("failed storage acknowledged: %v", err)
	}
}

func TestServerRequiresService(t *testing.T) {
	if _, err := NewServer(nil, "", nil, nil); err == nil {
		t.Error("nil service accepted")
	}
}
