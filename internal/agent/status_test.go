package agent

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
)

func TestBuildPodStatusIdle(t *testing.T) {
	pm := NewProcessManager(t.TempDir())

	status := BuildPodStatus(pm)
	if status.Phase != corev1.PodPending {
		t.Errorf("expected PodPending, got %s", status.Phase)
	}
}

func TestBuildPodStatusRunning(t *testing.T) {
	pm := NewProcessManager(t.TempDir())

	pod := testPod([]string{"sleep"}, []string{"60"})
	if err := pm.Launch(pod); err != nil {
		t.Fatalf("Launch failed: %v", err)
	}
	defer pm.Terminate(context.Background())

	status := BuildPodStatus(pm)
	if status.Phase != corev1.PodRunning {
		t.Errorf("expected PodRunning, got %s", status.Phase)
	}

	if status.StartTime == nil {
		t.Error("expected StartTime to be set")
	}

	// Check container statuses
	if len(status.ContainerStatuses) != 1 {
		t.Fatalf("expected 1 container status, got %d", len(status.ContainerStatuses))
	}
	cs := status.ContainerStatuses[0]
	if cs.Name != "test-container" {
		t.Errorf("expected container name 'test-container', got %q", cs.Name)
	}
	if !cs.Ready {
		t.Error("expected container to be ready")
	}
	if cs.State.Running == nil {
		t.Error("expected container state to be Running")
	}

	// Check conditions
	readyFound := false
	for _, c := range status.Conditions {
		if c.Type == corev1.PodReady && c.Status == corev1.ConditionTrue {
			readyFound = true
		}
	}
	if !readyFound {
		t.Error("expected PodReady condition to be True")
	}
}

func TestBuildPodStatusTerminated(t *testing.T) {
	pm := NewProcessManager(t.TempDir())

	pod := testPod([]string{"/bin/sh"}, []string{"-c", "exit 0"})
	if err := pm.Launch(pod); err != nil {
		t.Fatalf("Launch failed: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	status := BuildPodStatus(pm)
	if status.Phase != corev1.PodSucceeded {
		t.Errorf("expected PodSucceeded, got %s", status.Phase)
	}
}

func TestBuildPodStatusFailed(t *testing.T) {
	pm := NewProcessManager(t.TempDir())

	pod := testPod([]string{"/bin/sh"}, []string{"-c", "exit 1"})
	if err := pm.Launch(pod); err != nil {
		t.Fatalf("Launch failed: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	status := BuildPodStatus(pm)
	if status.Phase != corev1.PodFailed {
		t.Errorf("expected PodFailed, got %s", status.Phase)
	}

	if len(status.ContainerStatuses) == 0 {
		t.Fatal("expected container statuses")
	}

	cs := status.ContainerStatuses[0]
	if cs.State.Terminated == nil {
		t.Error("expected terminated container state")
	} else if cs.State.Terminated.ExitCode != 1 {
		t.Errorf("expected exit code 1, got %d", cs.State.Terminated.ExitCode)
	}
}
