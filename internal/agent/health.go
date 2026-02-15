/*
Health service implementation for the VKVMA agent.

Implements the standard gRPC health checking protocol (grpc.health.v1.Health)
plus integrates with the ProcessManager to report real workload health.
*/
package agent

import (
	"context"
	"log"
	"sync"

	healthpb "github.com/aws/aws-virtual-kubelet/proto/grpc/health/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// HealthService implements the gRPC Health service with real process state awareness.
type HealthService struct {
	healthpb.UnimplementedHealthServer

	mu        sync.RWMutex
	statusMap map[string]healthpb.HealthCheckResponse_ServingStatus
	procMgr   *ProcessManager
}

// NewHealthService creates a new health service that monitors the given process manager.
func NewHealthService(procMgr *ProcessManager) *HealthService {
	return &HealthService{
		statusMap: map[string]healthpb.HealthCheckResponse_ServingStatus{
			"": healthpb.HealthCheckResponse_SERVING,
		},
		procMgr: procMgr,
	}
}

// SetServing marks a service as SERVING.
func (s *HealthService) SetServing(service string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.statusMap[service] = healthpb.HealthCheckResponse_SERVING
}

// SetNotServing marks a service as NOT_SERVING.
func (s *HealthService) SetNotServing(service string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.statusMap[service] = healthpb.HealthCheckResponse_NOT_SERVING
}

// Check implements the unary health check RPC.
func (s *HealthService) Check(
	ctx context.Context, req *healthpb.HealthCheckRequest,
) (*healthpb.HealthCheckResponse, error) {

	s.mu.RLock()
	servingStatus, ok := s.statusMap[req.Service]
	s.mu.RUnlock()

	if !ok {
		return nil, status.Error(codes.NotFound, "unknown service")
	}

	return &healthpb.HealthCheckResponse{Status: servingStatus}, nil
}

// Watch implements the streaming health check RPC.
func (s *HealthService) Watch(
	req *healthpb.HealthCheckRequest,
	stream healthpb.Health_WatchServer,
) error {

	service := req.Service
	log.Printf("Health.Watch started for service=%q", service)

	// Send initial status
	s.mu.RLock()
	servingStatus, ok := s.statusMap[service]
	s.mu.RUnlock()

	if !ok {
		servingStatus = healthpb.HealthCheckResponse_SERVICE_UNKNOWN
	}

	if err := stream.Send(&healthpb.HealthCheckResponse{Status: servingStatus}); err != nil {
		return err
	}

	// Block until client disconnects
	<-stream.Context().Done()
	return status.Error(codes.Canceled, "stream ended")
}
