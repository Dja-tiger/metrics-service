package agent

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Dja-tiger/metrics-service/internal/middleware"
)

func TestRealIPFromConnection(t *testing.T) {
	for _, tc := range []struct{ network, address, cidr string }{
		{"tcp4", "127.0.0.1:0", "127.0.0.0/8"},
		{"tcp6", "[::1]:0", "::1/128"},
	} {
		t.Run(tc.network, func(t *testing.T) {
			listener, err := net.Listen(tc.network, tc.address)
			if err != nil && tc.network == "tcp6" {
				t.Skipf("IPv6 unavailable: %v", err)
			}
			if err != nil {
				t.Fatal(err)
			}
			mw, err := middleware.TrustedSubnet(tc.cidr)
			if err != nil {
				t.Fatal(err)
			}
			received := make(chan string, 2)
			srv := httptest.NewUnstartedServer(mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				host, _, err := net.SplitHostPort(r.RemoteAddr)
				if err != nil {
					t.Error(err)
				}
				if r.Header.Get("X-Real-IP") != host {
					t.Errorf("header=%q remote=%q", r.Header.Get("X-Real-IP"), host)
				}
				received <- r.Header.Get("X-Real-IP")
				w.WriteHeader(http.StatusOK)
			})))
			_ = srv.Listener.Close()
			srv.Listener = listener
			srv.Start()
			defer srv.Close()
			a, err := NewAgent(srv.URL, time.Hour, time.Hour)
			if err != nil {
				t.Fatal(err)
			}
			a.store.SetGauge("Alloc", 1)
			for range 2 {
				if err := a.ReportOnce(); err != nil {
					t.Fatal(err)
				}
				if ip := <-received; net.ParseIP(ip) == nil {
					t.Fatalf("invalid IP %q", ip)
				}
			}
		})
	}
}
