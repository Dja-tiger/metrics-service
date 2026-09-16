package grpcapi

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"time"

	models "github.com/Dja-tiger/metrics-service/internal/model"
	pb "github.com/Dja-tiger/metrics-service/internal/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/encoding/gzip"
)

// Client sends metric batches. Close it only after all agent workers finish.
type Client struct {
	conn *grpc.ClientConn
	rpc  pb.MetricsClient
}

// NewClient creates a reusable plaintext gRPC connection with source-IP metadata.
func NewClient(address string) (*Client, error) {
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(&ipCredentials{TransportCredentials: insecure.NewCredentials()}), grpc.WithPerRPCCredentials(ipMetadata{}), grpc.WithDefaultCallOptions(grpc.UseCompressor(gzip.Name)))
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
	request := &pb.UpdateMetricsRequest{Metrics: make([]*pb.Metric, 0, len(metrics))}
	for _, m := range metrics {
		item := &pb.Metric{Id: m.ID}
		switch m.MType {
		case models.Gauge:
			if m.Value == nil {
				return fmt.Errorf("gauge %q has no value", m.ID)
			}
			item.Type = pb.Metric_GAUGE
			item.Value = *m.Value
		case models.Counter:
			if m.Delta == nil {
				return fmt.Errorf("counter %q has no delta", m.ID)
			}
			item.Type = pb.Metric_COUNTER
			item.Delta = *m.Delta
		default:
			return fmt.Errorf("unknown metric type %q", m.MType)
		}
		request.Metrics = append(request.Metrics, item)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := c.rpc.UpdateMetrics(ctx, request)
	return err
}

type ipAuthInfo struct {
	credentials.AuthInfo
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
	host, _, err := net.SplitHostPort(conn.LocalAddr().String())
	if err != nil {
		return nil, nil, fmt.Errorf("gRPC local address: %w", err)
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return nil, nil, fmt.Errorf("gRPC local IP: %w", err)
	}
	return conn, ipAuthInfo{AuthInfo: info, ip: ip.WithZone("").Unmap().String()}, nil
}

type ipMetadata struct{}

func (ipMetadata) RequireTransportSecurity() bool { return false }
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
