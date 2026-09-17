package agent

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestReportsReuseHTTPConnection(t *testing.T) {
	var connections atomic.Int32
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		_, _ = io.WriteString(w, strings.Repeat("ack", 4096))
	}))
	server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			connections.Add(1)
		}
	}
	server.Start()
	defer server.Close()
	a, err := NewAgent(server.URL, time.Second, time.Second, WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	a.store.SetGauge("Alloc", 1)
	for range 2 {
		if err := a.ReportOnce(); err != nil {
			t.Fatal(err)
		}
	}
	if got := connections.Load(); got != 1 {
		t.Errorf("opened %d connections for two reports", got)
	}
}
