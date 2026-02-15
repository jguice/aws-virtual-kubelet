package ec2provider

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestEniNodeConfigure(t *testing.T) {
	en := &EniNode{
		name:               "fargate-10.0.0.1",
		hostname:           "fargate-10.0.0.1",
		lastTransitionTime: time.Now(),
	}

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "placeholder",
		},
	}

	result, err := en.Configure(context.Background(), node)
	if err != nil {
		t.Fatalf("Configure failed: %v", err)
	}

	// Check node name was set
	if result.Name != "fargate-10.0.0.1" {
		t.Errorf("expected node name 'fargate-10.0.0.1', got %q", result.Name)
	}

	// Check labels
	expectedLabels := map[string]string{
		"type":               "virtual-kubelet",
		"kubernetes.io/role": "agent",
	}
	for key, expectedVal := range expectedLabels {
		if result.Labels[key] != expectedVal {
			t.Errorf("expected label %s=%s, got %s", key, expectedVal, result.Labels[key])
		}
	}

	// Check capacity
	if result.Status.Capacity == nil {
		t.Fatal("expected capacity to be set")
	}
	cpuQuantity := result.Status.Capacity["cpu"]
	if cpuQuantity.IsZero() {
		t.Error("expected non-zero CPU capacity")
	}
	memQuantity := result.Status.Capacity["memory"]
	if memQuantity.IsZero() {
		t.Error("expected non-zero memory capacity")
	}

	// Check conditions
	if len(result.Status.Conditions) == 0 {
		t.Error("expected node conditions to be set")
	}

	readyFound := false
	for _, c := range result.Status.Conditions {
		if c.Type == corev1.NodeReady && c.Status == corev1.ConditionTrue {
			readyFound = true
		}
	}
	if !readyFound {
		t.Error("expected NodeReady condition to be True")
	}

	// Check allocatable equals capacity
	if len(result.Status.Allocatable) != len(result.Status.Capacity) {
		t.Error("expected allocatable to match capacity")
	}
}

func TestEniNodePing(t *testing.T) {
	en := &EniNode{name: "test-node"}
	err := en.Ping(context.Background())
	if err != nil {
		t.Errorf("expected Ping to return nil, got: %v", err)
	}
}

func TestEniNodeNotifyNodeStatus(t *testing.T) {
	en := &EniNode{name: "test-node"}
	// Should not panic
	en.NotifyNodeStatus(context.Background(), func(n *corev1.Node) {})
}
