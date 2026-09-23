// Package grpcapi implements batch metric delivery over gRPC.
package grpcapi

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"math"
	"net"
	"time"

	"github.com/Dja-tiger/metrics-service/internal/audit"
	"github.com/Dja-tiger/metrics-service/internal/delivery"
	models "github.com/Dja-tiger/metrics-service/internal/model"
	pb "github.com/Dja-tiger/metrics-service/internal/proto"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	_ "google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// BatchService stores a validated batch through the existing service layer.
type BatchService interface {
	UpdateMetricsOnce(string, []models.Metrics) (bool, error)
}

// Auditor receives successful batch audit events.
type Auditor interface {
	Notify(context.Context, audit.Event) error
}

type metricsServer struct {
	pb.UnimplementedMetricsServer
	service BatchService
	auditor Auditor
	logger  *zap.Logger
}

// NewServer registers the Metrics service and subnet and logging interceptors.
func NewServer(service BatchService, subnet string, auditor Auditor, logger *zap.Logger, tlsConfig *tls.Config) (*grpc.Server, error) {
	if service == nil {
		return nil, fmt.Errorf("metrics service is required")
	}
	if tlsConfig == nil || len(tlsConfig.Certificates) == 0 {
		return nil, fmt.Errorf("gRPC TLS server certificate is required")
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	logging := func(ctx context.Context, req any, info *grpc.UnaryServerInfo, next grpc.UnaryHandler) (any, error) {
		start := time.Now()
		response, err := next(ctx, req)
		logger.Info("grpc request handled", zap.String("method", info.FullMethod), zap.Duration("duration", time.Since(start)), zap.String("status", status.Code(err).String()))
		return response, err
	}
	interceptors := []grpc.UnaryServerInterceptor{logging}
	if subnet != "" {
		check, err := TrustedSubnet(subnet)
		if err != nil {
			return nil, err
		}
		interceptors = append(interceptors, check)
	}
	tlsConfig = tlsConfig.Clone()
	if tlsConfig.MinVersion < tls.VersionTLS12 {
		tlsConfig.MinVersion = tls.VersionTLS12
	}
	server := grpc.NewServer(grpc.Creds(credentials.NewTLS(tlsConfig)), grpc.ChainUnaryInterceptor(interceptors...))
	pb.RegisterMetricsServer(server, &metricsServer{service: service, auditor: auditor, logger: logger})
	return server, nil
}

// TrustedSubnet validates x-real-ip metadata, or allows requests for an empty CIDR.
func TrustedSubnet(cidr string) (grpc.UnaryServerInterceptor, error) {
	var subnet *net.IPNet
	if cidr != "" {
		_, parsed, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, fmt.Errorf("parse gRPC trusted subnet: %w", err)
		}
		subnet = parsed
	}
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, next grpc.UnaryHandler) (any, error) {
		if subnet != nil {
			values := metadata.ValueFromIncomingContext(ctx, "x-real-ip")
			if len(values) != 1 || !subnet.Contains(net.ParseIP(values[0])) {
				return nil, status.Error(codes.PermissionDenied, "IP is outside trusted subnet")
			}
		}
		return next(ctx, req)
	}, nil
}

func (s *metricsServer) UpdateMetrics(ctx context.Context, request *pb.UpdateMetricsRequest) (*pb.UpdateMetricsResponse, error) {
	if request == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	key := ""
	if values := metadata.ValueFromIncomingContext(ctx, "idempotency-key"); len(values) > 0 {
		if len(values) != 1 || !delivery.ValidKey(values[0]) {
			return nil, status.Error(codes.InvalidArgument, "invalid idempotency key")
		}
		key = values[0]
	}
	metrics := make([]models.Metrics, 0, len(request.GetMetrics()))
	names := make([]string, 0, len(request.GetMetrics()))
	for _, item := range request.GetMetrics() {
		if item == nil || item.GetId() == "" {
			return nil, status.Error(codes.InvalidArgument, "metric id is required")
		}
		m := models.Metrics{ID: item.GetId()}
		switch item.GetType() {
		case pb.Metric_GAUGE:
			if math.IsNaN(item.GetValue()) || math.IsInf(item.GetValue(), 0) {
				return nil, status.Error(codes.InvalidArgument, "gauge must be finite")
			}
			v := item.GetValue()
			m.MType = models.Gauge
			m.Value = &v
		case pb.Metric_COUNTER:
			d := item.GetDelta()
			m.MType = models.Counter
			m.Delta = &d
		default:
			return nil, status.Error(codes.InvalidArgument, "unknown metric type")
		}
		metrics = append(metrics, m)
		names = append(names, item.GetId())
	}
	if len(metrics) == 0 {
		return pb.UpdateMetricsResponse_builder{}.Build(), nil
	}
	if err := ctx.Err(); err != nil {
		return nil, status.FromContextError(err).Err()
	}
	applied, err := s.service.UpdateMetricsOnce(key, metrics)
	if err != nil {
		if errors.Is(err, delivery.ErrConflict) {
			return nil, status.Error(codes.AlreadyExists, err.Error())
		}
		s.logger.Error("store grpc metrics failed", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to store metrics")
	}
	if applied && s.auditor != nil {
		ip := ""
		if values := metadata.ValueFromIncomingContext(ctx, "x-real-ip"); len(values) == 1 {
			ip = values[0]
		}
		if err := s.auditor.Notify(ctx, audit.Event{Timestamp: time.Now().Unix(), Metrics: names, IPAddress: ip}); err != nil {
			s.logger.Warn("grpc audit failed", zap.Error(err))
		}
	}
	return pb.UpdateMetricsResponse_builder{}.Build(), nil
}
