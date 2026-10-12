package server

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync"

	"google.golang.org/grpc"
)

// RunWithGRPC drains both transports before flushing their shared storage.
// A nil gRPC server preserves the HTTP-only lifecycle.
func RunWithGRPC(ctx context.Context, srv *http.Server, rpc *grpc.Server, address string, flush func() error) error {
	if rpc == nil {
		return Run(ctx, srv, flush)
	}
	httpListener, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return errors.Join(err, flush())
	}
	rpcListener, err := net.Listen("tcp", address)
	if err != nil {
		_ = httpListener.Close()
		return errors.Join(err, flush())
	}
	return serveBoth(ctx, srv, rpc, httpListener, rpcListener, flush)
}

func serveBoth(ctx context.Context, srv *http.Server, rpc *grpc.Server, httpListener, rpcListener net.Listener, flush func() error) error {
	results := make(chan error, 2)
	go func() { results <- srv.Serve(httpListener) }()
	go func() { results <- rpc.Serve(rpcListener) }()
	var first error
	remaining := 2
	select {
	case <-ctx.Done():
	case first = <-results:
		remaining--
	}
	var shutdownErr error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); shutdownErr = srv.Shutdown(context.Background()) }()
	go func() { defer wg.Done(); rpc.GracefulStop() }()
	wg.Wait()
	errs := []error{first, shutdownErr}
	for i := 0; i < remaining; i++ {
		errs = append(errs, <-results)
	}
	for i, err := range errs {
		if errors.Is(err, http.ErrServerClosed) || errors.Is(err, grpc.ErrServerStopped) {
			errs[i] = nil
		}
	}
	return errors.Join(append(errs, flush())...)
}
