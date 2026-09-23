package grpcapi

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

// LoadServerTLS loads the certificate and private key required by the gRPC listener.
func LoadServerTLS(certFile, keyFile string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("load gRPC server certificate: %w", err)
	}
	return &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}}, nil
}

// LoadClientTLS uses system roots, or an explicit PEM trust bundle for a private CA.
// The certificate's DNS name or IP must match the gRPC target address.
func LoadClientTLS(caFile string) (*tls.Config, error) {
	cfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if caFile == "" {
		return cfg, nil
	}
	pem, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("read gRPC trust bundle: %w", err)
	}
	cfg.RootCAs = x509.NewCertPool()
	if !cfg.RootCAs.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("gRPC trust bundle contains no certificates")
	}
	return cfg, nil
}
