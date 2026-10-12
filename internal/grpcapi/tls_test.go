package grpcapi

import (
	"context"
	"crypto/tls"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	models "github.com/Dja-tiger/metrics-service/internal/model"
	"github.com/Dja-tiger/metrics-service/internal/repository"
	"github.com/Dja-tiger/metrics-service/internal/service"
	"github.com/Dja-tiger/metrics-service/internal/testutil"
)

func TestTLSLoaders(t *testing.T) {
	f := testutil.NewTLS(t)
	if cfg, err := LoadServerTLS(f.CertFile, f.KeyFile); err != nil || cfg.MinVersion < tls.VersionTLS12 {
		t.Fatalf("server TLS: %v", err)
	}
	if cfg, err := LoadClientTLS(f.CertFile); err != nil || cfg.RootCAs == nil || cfg.InsecureSkipVerify {
		t.Fatalf("client TLS: %v", err)
	}
	if cfg, err := LoadClientTLS(""); err != nil || cfg.RootCAs != nil || cfg.InsecureSkipVerify {
		t.Fatalf("system trust: %v", err)
	}
	invalid := filepath.Join(t.TempDir(), "invalid.pem")
	if err := os.WriteFile(invalid, []byte("not a certificate"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{invalid, invalid + ".missing"} {
		if _, err := LoadClientTLS(path); err == nil {
			t.Errorf("invalid trust bundle accepted: %s", path)
		}
		if _, err := LoadServerTLS(f.CertFile, path); err == nil {
			t.Errorf("invalid key accepted: %s", path)
		}
	}
	other := testutil.NewTLS(t)
	if _, err := LoadServerTLS(f.CertFile, other.KeyFile); err == nil {
		t.Error("mismatched certificate and key accepted")
	}
	for _, cfg := range []*tls.Config{nil, {InsecureSkipVerify: true}} {
		if _, err := NewClient("localhost:9090", cfg); err == nil {
			t.Error("unverified TLS accepted")
		}
	}
	if _, err := NewServer(service.NewMetricsService(repository.NewMemStorage()), "", nil, nil, nil); err == nil {
		t.Error("server without TLS accepted")
	}
	if !(ipMetadata{}).RequireTransportSecurity() {
		t.Error("IP metadata does not require TLS")
	}
}

func TestTLSRejectsUntrustedPeers(t *testing.T) {
	f := testutil.NewTLS(t)
	store := repository.NewMemStorage()
	server, err := NewServer(service.NewMetricsService(store), "", nil, nil, f.Server)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go server.Serve(listener)
	t.Cleanup(server.Stop)
	other := testutil.NewTLS(t)
	wrongName := f.Client.Clone()
	wrongName.ServerName = "wrong.example"
	for _, tc := range []struct {
		name string
		cfg  *tls.Config
	}{{"untrusted CA", other.Client}, {"wrong server name", wrongName}} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			conn, err := (&tls.Dialer{Config: tc.cfg}).DialContext(ctx, "tcp", listener.Addr().String())
			if err == nil {
				conn.Close()
				t.Fatal("invalid server certificate accepted")
			}
		})
	}
	// A plaintext HTTP/2 preface must not reach the gRPC handler.
	plain, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer plain.Close()
	if err := plain.SetDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := plain.Write([]byte("PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n")); err != nil {
		t.Fatal(err)
	}
	var response [64]byte
	if n, err := plain.Read(response[:]); n != 0 || err == nil {
		t.Fatalf("plaintext connection accepted: %q, %v", response[:n], err)
	} else if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
		t.Fatal("server did not reject plaintext before deadline")
	}
	clientConfig, err := LoadClientTLS(f.CertFile)
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewClient(listener.Addr().String(), clientConfig)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	delta := int64(3)
	if err := client.Send([]models.Metrics{{ID: "count", MType: models.Counter, Delta: &delta}}); err != nil {
		t.Fatal(err)
	}
	if got, _ := store.GetCounter("count"); got != 3 {
		t.Fatalf("TLS batch was not applied: %d", got)
	}
}
