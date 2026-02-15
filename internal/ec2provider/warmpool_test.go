package ec2provider

import (
	"testing"

	"github.com/aws/aws-virtual-kubelet/internal/config"

	corev1 "k8s.io/api/core/v1"
)

func initTestConfig() {
	config.InitConfig(&config.DirectLoader{DirectConfig: config.ProviderConfig{
		ClusterName:      "test-cluster",
		ManagementSubnet: "subnet-test",
	}})
}

func resetVKState() {
	VKState = State{}
	VKState.ReadyEC2 = make(map[string]Ec2Info)
	VKState.ProvisioningEC2 = make(map[string]Ec2Info)
	VKState.UnhealthyEC2 = make(map[string]Ec2Info)
	VKState.AllocatedEC2 = make(map[string]Ec2Info)
}

func TestPopSingleElement(t *testing.T) {
	m := map[string]Ec2Info{
		"i-123": {InstanceID: "i-123", PrivateIP: "10.0.0.1"},
	}

	elem, remainder := pop(m)

	if elem.InstanceID != "i-123" {
		t.Errorf("expected InstanceID i-123, got %s", elem.InstanceID)
	}
	if len(remainder) != 0 {
		t.Errorf("expected empty remainder, got %d elements", len(remainder))
	}
}

func TestPopMultipleElements(t *testing.T) {
	m := map[string]Ec2Info{
		"i-1": {InstanceID: "i-1"},
		"i-2": {InstanceID: "i-2"},
		"i-3": {InstanceID: "i-3"},
	}

	elem, remainder := pop(m)

	// Should have popped one element
	if elem.InstanceID == "" {
		t.Error("expected non-empty InstanceID")
	}
	if len(remainder) != 2 {
		t.Errorf("expected 2 remaining, got %d", len(remainder))
	}
	// The popped element should not be in the remainder
	if _, exists := remainder[elem.InstanceID]; exists {
		t.Error("popped element should not be in remainder")
	}
}

func TestPopKeySingleElement(t *testing.T) {
	m := map[string]Ec2Info{
		"i-only": {InstanceID: "i-only"},
	}

	key := popKey(m)
	if key != "i-only" {
		t.Errorf("expected key 'i-only', got %q", key)
	}
}

func TestPopKeyMultipleElements(t *testing.T) {
	m := map[string]Ec2Info{
		"i-1": {InstanceID: "i-1"},
		"i-2": {InstanceID: "i-2"},
	}

	key := popKey(m)
	if key != "i-1" && key != "i-2" {
		t.Errorf("expected one of 'i-1' or 'i-2', got %q", key)
	}
}

func TestRemoveFromWarmPool(t *testing.T) {
	resetVKState()
	VKState.ReadyEC2["i-1"] = Ec2Info{InstanceID: "i-1"}
	VKState.ProvisioningEC2["i-1"] = Ec2Info{InstanceID: "i-1"}
	VKState.UnhealthyEC2["i-1"] = Ec2Info{InstanceID: "i-1"}

	err := RemoveFromWarmPool(Ec2Info{InstanceID: "i-1"})
	if err != nil {
		t.Fatalf("RemoveFromWarmPool failed: %v", err)
	}

	if _, exists := VKState.ReadyEC2["i-1"]; exists {
		t.Error("i-1 should not be in ReadyEC2")
	}
	if _, exists := VKState.ProvisioningEC2["i-1"]; exists {
		t.Error("i-1 should not be in ProvisioningEC2")
	}
	if _, exists := VKState.UnhealthyEC2["i-1"]; exists {
		t.Error("i-1 should not be in UnhealthyEC2")
	}
}

func TestRemoveFromReadyState(t *testing.T) {
	resetVKState()
	VKState.ReadyEC2["i-1"] = Ec2Info{InstanceID: "i-1"}

	err := RemoveFromReadyState(Ec2Info{InstanceID: "i-1"})
	if err != nil {
		t.Fatalf("RemoveFromReadyState failed: %v", err)
	}

	if _, exists := VKState.ReadyEC2["i-1"]; exists {
		t.Error("i-1 should not be in ReadyEC2")
	}
}

func TestRemoveFromUnhealthyState(t *testing.T) {
	resetVKState()
	VKState.UnhealthyEC2["i-1"] = Ec2Info{InstanceID: "i-1"}

	err := RemoveFromUnhealthyState(Ec2Info{InstanceID: "i-1"})
	if err != nil {
		t.Fatalf("RemoveFromUnhealthyState failed: %v", err)
	}

	if _, exists := VKState.UnhealthyEC2["i-1"]; exists {
		t.Error("i-1 should not be in UnhealthyEC2")
	}
}

func TestRemoveFromWarmPoolNonExistent(t *testing.T) {
	resetVKState()

	// Should not error when removing non-existent instance
	err := RemoveFromWarmPool(Ec2Info{InstanceID: "i-nonexistent"})
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
}

func TestEc2InfoStruct(t *testing.T) {
	info := Ec2Info{
		InstanceID:     "i-abc123",
		PrivateIP:      "10.0.0.5",
		IAMProfile:     "my-profile",
		SecurityGroups: []string{"sg-1", "sg-2"},
		RetryCount:     3,
	}

	if info.InstanceID != "i-abc123" {
		t.Errorf("expected InstanceID 'i-abc123', got %q", info.InstanceID)
	}
	if info.PrivateIP != "10.0.0.5" {
		t.Errorf("expected PrivateIP '10.0.0.5', got %q", info.PrivateIP)
	}
	if len(info.SecurityGroups) != 2 {
		t.Errorf("expected 2 SecurityGroups, got %d", len(info.SecurityGroups))
	}
}

func TestStateInitialization(t *testing.T) {
	resetVKState()

	if VKState.ReadyEC2 == nil {
		t.Error("ReadyEC2 should be initialized")
	}
	if VKState.ProvisioningEC2 == nil {
		t.Error("ProvisioningEC2 should be initialized")
	}
	if VKState.UnhealthyEC2 == nil {
		t.Error("UnhealthyEC2 should be initialized")
	}
	if VKState.AllocatedEC2 == nil {
		t.Error("AllocatedEC2 should be initialized")
	}
}

func TestPopulateEC2Tags(t *testing.T) {
	initTestConfig()
	// Set global nodeName for tag generation
	nodeName = "test-node"

	wpm := &WarmPoolManager{}

	tests := []struct {
		name           string
		reason         string
		expectedStatus string
	}{
		{"initial setup", initialSetup, operationPendingWarmpool},
		{"set ready", setReady, operationReady},
		{"set unhealthy", setUnhealthy, operationUnhealthy},
		{"set in use", setInUse, operationPodInUse},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tags := wpm.populateEC2Tags(tt.reason, corev1.Pod{})

			if len(tags) != 1 {
				t.Fatalf("expected 1 TagSpecification, got %d", len(tags))
			}

			// Find the WarmpoolStatus tag
			statusFound := false
			nodeNameFound := false
			for _, tag := range tags[0].Tags {
				if *tag.Key == "aws-virtual-kubelet/WarmpoolStatus" {
					statusFound = true
					if *tag.Value != tt.expectedStatus {
						t.Errorf("expected status %q, got %q", tt.expectedStatus, *tag.Value)
					}
				}
				if *tag.Key == "aws-virtual-kubelet/WarmpoolNodeName" {
					nodeNameFound = true
					if *tag.Value != "test-node" {
						t.Errorf("expected node name 'test-node', got %q", *tag.Value)
					}
				}
			}

			if !statusFound {
				t.Error("WarmpoolStatus tag not found")
			}
			if !nodeNameFound {
				t.Error("WarmpoolNodeName tag not found")
			}
		})
	}
}

func TestPopulateEC2TagsSetPod(t *testing.T) {
	initTestConfig()
	nodeName = "test-node"
	wpm := &WarmPoolManager{}

	pod := corev1.Pod{}
	pod.Name = "my-pod"
	pod.Namespace = "my-ns"
	pod.UID = "uid-123"

	tags := wpm.populateEC2Tags(setPod, pod)

	podNameFound := false
	podNsFound := false
	podUIDFound := false
	for _, tag := range tags[0].Tags {
		switch *tag.Key {
		case "aws-virtual-kubelet/WarmpoolPodName":
			podNameFound = true
			if *tag.Value != "my-pod" {
				t.Errorf("expected pod name 'my-pod', got %q", *tag.Value)
			}
		case "aws-virtual-kubelet/WarmpoolPodNamespace":
			podNsFound = true
			if *tag.Value != "my-ns" {
				t.Errorf("expected namespace 'my-ns', got %q", *tag.Value)
			}
		case "aws-virtual-kubelet/WarmpoolPodUID":
			podUIDFound = true
			if *tag.Value != "uid-123" {
				t.Errorf("expected UID 'uid-123', got %q", *tag.Value)
			}
		}
	}

	if !podNameFound {
		t.Error("WarmpoolPodName tag not found")
	}
	if !podNsFound {
		t.Error("WarmpoolPodNamespace tag not found")
	}
	if !podUIDFound {
		t.Error("WarmpoolPodUID tag not found")
	}
}

func TestSetNodeName(t *testing.T) {
	wpm := &WarmPoolManager{}
	wpm.SetNodeName("my-test-node")

	if nodeName != "my-test-node" {
		t.Errorf("expected nodeName 'my-test-node', got %q", nodeName)
	}
}
