package server

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"
)

func TestShutdownDrainsHandlersBeforeFlush(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	started, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(release) })
	finished := false
	flushed := false
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		if r.Context().Err() != nil {
			t.Error("handler context canceled during shutdown")
		}
		finished = true
		_, _ = w.Write([]byte("saved"))
	})}
	done := make(chan error, 1)
	go func() {
		done <- serve(ctx, srv, listener, func() error {
			if !finished {
				t.Error("flush before handler completion")
			}
			flushed = true
			return nil
		})
	}()
	response := make(chan error, 1)
	go func() {
		client := &http.Client{Timeout: 3 * time.Second}
		r, err := client.Get("http://" + listener.Addr().String())
		if err == nil {
			_, err = io.Copy(io.Discard, r.Body)
			_ = r.Body.Close()
		}
		response <- err
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("request not started")
	}
	cancel()
	select {
	case <-done:
		t.Fatal("shutdown returned before handler completed")
	case <-time.After(20 * time.Millisecond):
	}
	once.Do(func() { close(release) })
	if err := <-response; err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown stuck")
	}
	if !flushed {
		t.Fatal("not flushed")
	}
}

func TestRunReportsListenAndFlushErrors(t *testing.T) {
	want := errors.New("disk failure")
	err := Run(context.Background(), &http.Server{Addr: "invalid address"}, func() error { return want })
	if !errors.Is(err, want) {
		t.Fatalf("got %v", err)
	}
}
