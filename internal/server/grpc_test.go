package server

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	pb "github.com/Dja-tiger/metrics-service/internal/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type blockingMetrics struct {
	pb.UnimplementedMetricsServer
	entered chan struct{}
	release chan struct{}
}

func (s *blockingMetrics) UpdateMetrics(context.Context, *pb.UpdateMetricsRequest) (*pb.UpdateMetricsResponse, error) {
	close(s.entered)
	<-s.release
	return &pb.UpdateMetricsResponse{}, nil
}

func TestGRPCShutdownDrainsBeforeFlush(t *testing.T) {
	httpListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer httpListener.Close()
	rpcListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer rpcListener.Close()
	impl := &blockingMetrics{entered: make(chan struct{}), release: make(chan struct{})}
	rpc := grpc.NewServer()
	pb.RegisterMetricsServer(rpc, impl)
	defer rpc.Stop()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	flushed := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- serveBoth(ctx, &http.Server{}, rpc, httpListener, rpcListener, func() error { close(flushed); return nil })
	}()
	conn, err := grpc.NewClient(rpcListener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	requestDone := make(chan error, 1)
	requestCtx, requestCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer requestCancel()
	go func() {
		_, err := pb.NewMetricsClient(conn).UpdateMetrics(requestCtx, &pb.UpdateMetricsRequest{})
		requestDone <- err
	}()
	select {
	case <-impl.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("RPC did not start")
	}
	cancel()
	select {
	case <-flushed:
		t.Fatal("flushed before active RPC completed")
	case <-time.After(50 * time.Millisecond):
	}
	close(impl.release)
	if err := <-requestDone; err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown blocked")
	}
	select {
	case <-flushed:
	default:
		t.Fatal("not flushed")
	}
}
