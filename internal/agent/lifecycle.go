/*
ApplicationLifecycle gRPC service implementation for the VKVMA agent.

Implements the vkvmagent.v0.ApplicationLifecycleServer interface:
  - LaunchApplication:       Start a workload process from pod spec
  - TerminateApplication:    Gracefully stop the workload
  - CheckApplicationHealth:  Return current pod status (unary)
  - WatchApplicationHealth:  Stream pod status updates (server streaming)
*/
package agent

import (
	"context"
	"fmt"
	"log"
	"time"

	pb "github.com/aws/aws-virtual-kubelet/proto/vkvmagent/v0"
)

// LifecycleService implements the ApplicationLifecycle gRPC service.
type LifecycleService struct {
	pb.UnimplementedApplicationLifecycleServer
	procMgr *ProcessManager
}

// NewLifecycleService creates a new lifecycle service backed by the given process manager.
func NewLifecycleService(procMgr *ProcessManager) *LifecycleService {
	return &LifecycleService{procMgr: procMgr}
}

// LaunchApplication starts a workload process based on the pod spec.
func (s *LifecycleService) LaunchApplication(
	ctx context.Context, req *pb.LaunchApplicationRequest,
) (*pb.LaunchApplicationResponse, error) {

	pod := req.GetPod()
	if pod == nil {
		return nil, fmt.Errorf("LaunchApplication: pod is nil")
	}

	log.Printf("LaunchApplication: pod=%s/%s containers=%d",
		pod.Namespace, pod.Name, len(pod.Spec.Containers))

	if err := s.procMgr.Launch(pod); err != nil {
		return nil, fmt.Errorf("LaunchApplication failed: %w", err)
	}

	return &pb.LaunchApplicationResponse{}, nil
}

// TerminateApplication gracefully stops the running workload.
func (s *LifecycleService) TerminateApplication(
	ctx context.Context, req *pb.TerminateApplicationRequest,
) (*pb.TerminateApplicationResponse, error) {

	log.Printf("TerminateApplication: stopping workload")

	if err := s.procMgr.Terminate(ctx); err != nil {
		return nil, fmt.Errorf("TerminateApplication failed: %w", err)
	}

	return &pb.TerminateApplicationResponse{}, nil
}

// CheckApplicationHealth returns the current pod status.
func (s *LifecycleService) CheckApplicationHealth(
	ctx context.Context, req *pb.ApplicationHealthRequest,
) (*pb.ApplicationHealthResponse, error) {

	log.Printf("CheckApplicationHealth: state=%d", s.procMgr.State())

	podStatus := BuildPodStatus(s.procMgr)

	return &pb.ApplicationHealthResponse{
		PodStatus: podStatus,
	}, nil
}

// WatchApplicationHealth streams pod status updates at regular intervals.
func (s *LifecycleService) WatchApplicationHealth(
	req *pb.ApplicationHealthRequest,
	stream pb.ApplicationLifecycle_WatchApplicationHealthServer,
) error {

	log.Printf("WatchApplicationHealth: starting stream")

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Send initial status immediately
	podStatus := BuildPodStatus(s.procMgr)
	if err := stream.Send(&pb.ApplicationHealthResponse{PodStatus: podStatus}); err != nil {
		return fmt.Errorf("failed to send initial status: %w", err)
	}

	for {
		select {
		case <-stream.Context().Done():
			log.Printf("WatchApplicationHealth: client disconnected")
			return nil
		case <-ticker.C:
			podStatus := BuildPodStatus(s.procMgr)
			if err := stream.Send(&pb.ApplicationHealthResponse{PodStatus: podStatus}); err != nil {
				log.Printf("WatchApplicationHealth: send error: %v", err)
				return err
			}
		}
	}
}
