// Package server coordinates HTTP serving and graceful shutdown.
package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
)

// Run accepts requests until cancellation, waits for active handlers, and then
// calls flush. It does not cancel handler contexts or impose a shutdown deadline.
func Run(ctx context.Context, srv *http.Server, flush func() error) error {
	listener, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return errors.Join(fmt.Errorf("listen: %w", err), flush())
	}
	return serve(ctx, srv, listener, flush)
}

func serve(ctx context.Context, srv *http.Server, listener net.Listener, flush func() error) error {
	result := make(chan error, 1)
	go func() { result <- srv.Serve(listener) }()
	var serveErr error
	select {
	case <-ctx.Done():
	case serveErr = <-result:
	}
	// The signal context is canceled, so shutdown needs its own live context.
	shutdownErr := srv.Shutdown(context.Background())
	if serveErr == nil {
		serveErr = <-result
	}
	if errors.Is(serveErr, http.ErrServerClosed) {
		serveErr = nil
	}
	return errors.Join(serveErr, shutdownErr, flush())
}
