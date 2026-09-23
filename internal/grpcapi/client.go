package grpcapi

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/netip"
	"time"

	"github.com/Dja-tiger/metrics-service/internal/delivery"
	models "github.com/Dja-tiger/metrics-service/internal/model"
	pb "github.com/Dja-tiger/metrics-service/internal/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/metadata"
)

// Client sends metric batches. Close it only after all agent workers finish.
type Client struct {
	conn *grpc.ClientConn
	rpc  pb.MetricsClient
}

// NewClient creates a verified TLS connection with source-IP metadata.
func NewClient(address string, tlsConfig *tls.Config) (*Client, error) {
	if tlsConfig == nil || tlsConfig.InsecureSkipVerify {
		return nil, fmt.Errorf("verified gRPC TLS configuration is required")
	}
	tlsConfig = tlsConfig.Clone()
	if tlsConfig.MinVersion < tls.VersionTLS12 {
		tlsConfig.MinVersion = tls.VersionTLS12
	}
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(&ipCredentials{TransportCredentials: credentials.NewTLS(tlsConfig)}), grpc.WithPerRPCCredentials(ipMetadata{}), grpc.WithDefaultCallOptions(grpc.UseCompressor(gzip.Name)))
	if err != nil {
		return nil, fmt.Errorf("create gRPC client: %w", err)
	}
	return &Client{conn: conn, rpc: pb.NewMetricsClient(conn)}, nil
}

// Close releases the connection after pending sends have completed.
func (c *Client) Close() error { return c.conn.Close() }

// Send converts and sends a nonempty batch with a per-attempt timeout.
func (c *Client) Send(metrics []models.Metrics) error {
	if len(metrics) == 0 {
		return nil
	}
	return c.SendBatch(delivery.New(metrics))
}

// SendBatch preserves a logical request ID across retry attempts.
func (c *Client) SendBatch(batch delivery.Batch) error {
	metrics := batch.Metrics
	if len(metrics) == 0 {
		return nil
	}
	if !delivery.ValidKey(batch.ID) {
		return fmt.Errorf("invalid batch ID")
	}
	items := make([]*pb.Metric, 0, len(metrics))
	for _, m := range metrics {
		item := pb.Metric_builder{Id: m.ID}.Build()
		switch m.MType {
		case models.Gauge:
			if m.Value == nil {
				return fmt.Errorf("gauge %q has no value", m.ID)
			}
			item.SetType(pb.Metric_GAUGE)
			item.SetValue(*m.Value)
		case models.Counter:
			if m.Delta == nil {
				return fmt.Errorf("counter %q has no delta", m.ID)
			}
			item.SetType(pb.Metric_COUNTER)
			item.SetDelta(*m.Delta)
		default:
			return fmt.Errorf("unknown metric type %q", m.MType)
		}
		items = append(items, item)
	}
	request := pb.UpdateMetricsRequest_builder{Metrics: items}.Build()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ctx = metadata.AppendToOutgoingContext(ctx, "idempotency-key", batch.ID)
	_, err := c.rpc.UpdateMetrics(ctx, request)
	if err != nil {
		return fmt.Errorf("send gRPC metrics: %w", err)
	}
	return nil
}

type ipAuthInfo struct {
	credentials.TLSInfo
	ip string
}
type ipCredentials struct {
	credentials.TransportCredentials
}

func (c *ipCredentials) Clone() credentials.TransportCredentials {
	return &ipCredentials{TransportCredentials: c.TransportCredentials.Clone()}
}
func (c *ipCredentials) ClientHandshake(ctx context.Context, authority string, raw net.Conn) (net.Conn, credentials.AuthInfo, error) {
	conn, info, err := c.TransportCredentials.ClientHandshake(ctx, authority, raw)
	if err != nil {
		return nil, nil, err
	}
	tlsInfo, ok := info.(credentials.TLSInfo)
	if !ok {
		return nil, nil, fmt.Errorf("gRPC TLS handshake information unavailable")
	}
	host, _, err := net.SplitHostPort(conn.LocalAddr().String())
	if err != nil {
		return nil, nil, fmt.Errorf("gRPC local address: %w", err)
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return nil, nil, fmt.Errorf("gRPC local IP: %w", err)
	}
	return conn, ipAuthInfo{TLSInfo: tlsInfo, ip: ip.WithZone("").Unmap().String()}, nil
}

type ipMetadata struct{}

func (ipMetadata) RequireTransportSecurity() bool { return true }
func (ipMetadata) GetRequestMetadata(ctx context.Context, _ ...string) (map[string]string, error) {
	info, ok := credentials.RequestInfoFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("gRPC connection information unavailable")
	}
	auth, ok := info.AuthInfo.(ipAuthInfo)
	if !ok {
		return nil, fmt.Errorf("gRPC source IP unavailable")
	}
	return map[string]string{"x-real-ip": auth.ip}, nil
}
