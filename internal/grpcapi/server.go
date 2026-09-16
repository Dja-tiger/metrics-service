// Package grpcapi implements batch metric delivery over gRPC.
package grpcapi

import (
	"context"
	"fmt"
	"math"
	"net"
	"time"

	"github.com/Dja-tiger/metrics-service/internal/audit"
	models "github.com/Dja-tiger/metrics-service/internal/model"
	pb "github.com/Dja-tiger/metrics-service/internal/proto"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	_ "google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// BatchService stores a validated batch through the existing service layer.
type BatchService interface{ UpdateMetrics([]models.Metrics) }

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
func NewServer(service BatchService, subnet string, auditor Auditor, logger *zap.Logger) (*grpc.Server, error) {
	check, err := TrustedSubnet(subnet)
	if err != nil {
		return nil, err
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
	server := grpc.NewServer(grpc.ChainUnaryInterceptor(logging, check))
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
	metrics := make([]models.Metrics, 0, len(request.Metrics))
	names := make([]string, 0, len(request.Metrics))
	for _, item := range request.Metrics {
		if item == nil || item.Id == "" {
			return nil, status.Error(codes.InvalidArgument, "metric id is required")
		}
		m := models.Metrics{ID: item.Id}
		switch item.Type {
		case pb.Metric_GAUGE:
			if math.IsNaN(item.Value) || math.IsInf(item.Value, 0) {
				return nil, status.Error(codes.InvalidArgument, "gauge must be finite")
			}
			v := item.Value
			m.MType = models.Gauge
			m.Value = &v
		case pb.Metric_COUNTER:
			d := item.Delta
			m.MType = models.Counter
			m.Delta = &d
		default:
			return nil, status.Error(codes.InvalidArgument, "unknown metric type")
		}
		metrics = append(metrics, m)
		names = append(names, item.Id)
	}
	if len(metrics) == 0 {
		return &pb.UpdateMetricsResponse{}, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, status.FromContextError(err).Err()
	}
	s.service.UpdateMetrics(metrics)
	if s.auditor != nil {
		ip := ""
		if values := metadata.ValueFromIncomingContext(ctx, "x-real-ip"); len(values) == 1 {
			ip = values[0]
		}
		if err := s.auditor.Notify(ctx, audit.Event{Timestamp: time.Now().Unix(), Metrics: names, IPAddress: ip}); err != nil {
			s.logger.Info("grpc audit failed", zap.Error(err))
		}
	}
	return &pb.UpdateMetricsResponse{}, nil
}
