package ec2provider

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-virtual-kubelet/internal/awsutils"
	k8stypes "k8s.io/apimachinery/pkg/types"
)

// recoveryMockEC2 implements the EC2API interface for recovery tests.
type recoveryMockEC2 struct {
	awsutils.EC2API
	describeResp *ec2.DescribeInstancesOutput
	describeErr  error
}

func (m *recoveryMockEC2) DescribeInstances(ctx context.Context, input *ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error) {
	if m.describeErr != nil {
		return nil, m.describeErr
	}
	return m.describeResp, nil
}

func makeInstance(instanceID, privateIP string, tags map[string]string) types.Instance {
	var ec2Tags []types.Tag
	for k, v := range tags {
		ec2Tags = append(ec2Tags, types.Tag{Key: aws.String(k), Value: aws.String(v)})
	}
	return types.Instance{
		InstanceId:       aws.String(instanceID),
		PrivateIpAddress: aws.String(privateIP),
		Tags:             ec2Tags,
		State:            &types.InstanceState{Name: types.InstanceStateNameRunning},
	}
}

func TestRecoverPodsNoInstances(t *testing.T) {
	initTestConfig()
	mock := &recoveryMockEC2{
		describeResp: &ec2.DescribeInstancesOutput{},
	}

	recovery := NewPodRecovery(mock, "test-node")
	recovered, err := recovery.RecoverPods(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(recovered) != 0 {
		t.Errorf("expected 0 recovered pods, got %d", len(recovered))
	}
}

func TestRecoverPodsWithTaggedInstance(t *testing.T) {
	initTestConfig()
	inst := makeInstance("i-abc123", "10.0.0.5", map[string]string{
		awsutils.TagKeyPodUID:       "uid-123",
		awsutils.TagKeyPodName:      "my-pod",
		awsutils.TagKeyPodNamespace: "default",
		awsutils.TagKeyNodeName:     "test-node",
		awsutils.TagKeyClusterName:  "test-cluster",
	})

	mock := &recoveryMockEC2{
		describeResp: &ec2.DescribeInstancesOutput{
			Reservations: []types.Reservation{{Instances: []types.Instance{inst}}},
		},
	}

	recovery := NewPodRecovery(mock, "test-node")
	recovered, err := recovery.RecoverPods(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(recovered) != 1 {
		t.Fatalf("expected 1 recovered pod, got %d", len(recovered))
	}

	rp := recovered[0]
	if rp.InstanceID != "i-abc123" {
		t.Errorf("expected instance ID i-abc123, got %s", rp.InstanceID)
	}
	if rp.PrivateIP != "10.0.0.5" {
		t.Errorf("expected private IP 10.0.0.5, got %s", rp.PrivateIP)
	}
	if rp.Pod.Name != "my-pod" {
		t.Errorf("expected pod name my-pod, got %s", rp.Pod.Name)
	}
	if rp.Pod.Namespace != "default" {
		t.Errorf("expected namespace default, got %s", rp.Pod.Namespace)
	}
	if string(rp.Pod.UID) != "uid-123" {
		t.Errorf("expected UID uid-123, got %s", rp.Pod.UID)
	}
	if rp.Pod.Status.PodIP != "10.0.0.5" {
		t.Errorf("expected pod IP 10.0.0.5, got %s", rp.Pod.Status.PodIP)
	}
	if rp.Pod.Annotations["compute.amazonaws.com/instance-id"] != "i-abc123" {
		t.Errorf("expected instance-id annotation, got %s", rp.Pod.Annotations["compute.amazonaws.com/instance-id"])
	}
	if rp.Pod.Annotations["compute.amazonaws.com/recovered"] != "true" {
		t.Error("expected recovered annotation")
	}
}

func TestRecoverPodsSkipsKnownPods(t *testing.T) {
	initTestConfig()
	inst := makeInstance("i-abc123", "10.0.0.5", map[string]string{
		awsutils.TagKeyPodUID:       "uid-known",
		awsutils.TagKeyPodName:      "known-pod",
		awsutils.TagKeyPodNamespace: "default",
		awsutils.TagKeyNodeName:     "test-node",
		awsutils.TagKeyClusterName:  "test-cluster",
	})

	mock := &recoveryMockEC2{
		describeResp: &ec2.DescribeInstancesOutput{
			Reservations: []types.Reservation{{Instances: []types.Instance{inst}}},
		},
	}

	known := map[k8stypes.UID]bool{
		k8stypes.UID("uid-known"): true,
	}

	recovery := NewPodRecovery(mock, "test-node")
	recovered, err := recovery.RecoverPods(context.Background(), known)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(recovered) != 0 {
		t.Errorf("expected 0 recovered pods (known UID should be skipped), got %d", len(recovered))
	}
}

func TestRecoverPodsSkipsIncompleteTags(t *testing.T) {
	initTestConfig()
	// Missing PodNamespace tag
	inst := makeInstance("i-abc123", "10.0.0.5", map[string]string{
		awsutils.TagKeyPodUID:  "uid-123",
		awsutils.TagKeyPodName: "my-pod",
		// No namespace
	})

	mock := &recoveryMockEC2{
		describeResp: &ec2.DescribeInstancesOutput{
			Reservations: []types.Reservation{{Instances: []types.Instance{inst}}},
		},
	}

	recovery := NewPodRecovery(mock, "test-node")
	recovered, err := recovery.RecoverPods(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(recovered) != 0 {
		t.Errorf("expected 0 recovered pods (incomplete tags), got %d", len(recovered))
	}
}

func TestRecoverPodsMultipleInstances(t *testing.T) {
	initTestConfig()
	inst1 := makeInstance("i-111", "10.0.0.1", map[string]string{
		awsutils.TagKeyPodUID:       "uid-1",
		awsutils.TagKeyPodName:      "pod-1",
		awsutils.TagKeyPodNamespace: "ns-a",
		awsutils.TagKeyNodeName:     "test-node",
		awsutils.TagKeyClusterName:  "test-cluster",
	})
	inst2 := makeInstance("i-222", "10.0.0.2", map[string]string{
		awsutils.TagKeyPodUID:       "uid-2",
		awsutils.TagKeyPodName:      "pod-2",
		awsutils.TagKeyPodNamespace: "ns-b",
		awsutils.TagKeyNodeName:     "test-node",
		awsutils.TagKeyClusterName:  "test-cluster",
	})

	mock := &recoveryMockEC2{
		describeResp: &ec2.DescribeInstancesOutput{
			Reservations: []types.Reservation{
				{Instances: []types.Instance{inst1, inst2}},
			},
		},
	}

	recovery := NewPodRecovery(mock, "test-node")
	recovered, err := recovery.RecoverPods(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(recovered) != 2 {
		t.Fatalf("expected 2 recovered pods, got %d", len(recovered))
	}
}

func TestRecoverPodsEC2Error(t *testing.T) {
	initTestConfig()
	mock := &recoveryMockEC2{
		describeErr: context.DeadlineExceeded,
	}

	recovery := NewPodRecovery(mock, "test-node")
	_, err := recovery.RecoverPods(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error from EC2 API failure")
	}
}

func TestFindOrphanedInstancesNone(t *testing.T) {
	initTestConfig()
	inst := makeInstance("i-abc123", "10.0.0.5", map[string]string{
		awsutils.TagKeyPodUID:       "uid-active",
		awsutils.TagKeyPodName:      "active-pod",
		awsutils.TagKeyPodNamespace: "default",
		awsutils.TagKeyNodeName:     "test-node",
		awsutils.TagKeyClusterName:  "test-cluster",
	})

	mock := &recoveryMockEC2{
		describeResp: &ec2.DescribeInstancesOutput{
			Reservations: []types.Reservation{{Instances: []types.Instance{inst}}},
		},
	}

	active := map[k8stypes.UID]bool{
		k8stypes.UID("uid-active"): true,
	}

	recovery := NewPodRecovery(mock, "test-node")
	orphaned, err := recovery.FindOrphanedInstances(context.Background(), active)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(orphaned) != 0 {
		t.Errorf("expected 0 orphaned, got %d", len(orphaned))
	}
}

func TestFindOrphanedInstancesDetectsOrphan(t *testing.T) {
	initTestConfig()
	inst := makeInstance("i-orphan", "10.0.0.5", map[string]string{
		awsutils.TagKeyPodUID:       "uid-gone",
		awsutils.TagKeyPodName:      "gone-pod",
		awsutils.TagKeyPodNamespace: "default",
		awsutils.TagKeyNodeName:     "test-node",
		awsutils.TagKeyClusterName:  "test-cluster",
	})

	mock := &recoveryMockEC2{
		describeResp: &ec2.DescribeInstancesOutput{
			Reservations: []types.Reservation{{Instances: []types.Instance{inst}}},
		},
	}

	// Empty active set — pod is orphaned
	active := map[k8stypes.UID]bool{}

	recovery := NewPodRecovery(mock, "test-node")
	orphaned, err := recovery.FindOrphanedInstances(context.Background(), active)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(orphaned) != 1 {
		t.Fatalf("expected 1 orphaned, got %d", len(orphaned))
	}
	if orphaned[0] != "i-orphan" {
		t.Errorf("expected orphan i-orphan, got %s", orphaned[0])
	}
}

func TestFindOrphanedSkipsWarmPoolInstances(t *testing.T) {
	initTestConfig()
	// Instance with no PodUID tag (warm pool instance)
	inst := makeInstance("i-warmpool", "10.0.0.5", map[string]string{
		"aws-virtual-kubelet/WarmpoolStatus": "Operation.READY",
		awsutils.TagKeyClusterName:           "test-cluster",
	})

	mock := &recoveryMockEC2{
		describeResp: &ec2.DescribeInstancesOutput{
			Reservations: []types.Reservation{{Instances: []types.Instance{inst}}},
		},
	}

	recovery := NewPodRecovery(mock, "test-node")
	orphaned, err := recovery.FindOrphanedInstances(context.Background(), map[k8stypes.UID]bool{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(orphaned) != 0 {
		t.Errorf("expected 0 orphaned (warm pool should be skipped), got %d", len(orphaned))
	}
}

func TestExtractTagMap(t *testing.T) {
	tags := []types.Tag{
		{Key: aws.String("key1"), Value: aws.String("val1")},
		{Key: aws.String("key2"), Value: aws.String("val2")},
	}

	m := extractTagMap(tags)
	if m["key1"] != "val1" {
		t.Errorf("expected val1, got %s", m["key1"])
	}
	if m["key2"] != "val2" {
		t.Errorf("expected val2, got %s", m["key2"])
	}
}

func TestExtractTagMapNilValues(t *testing.T) {
	tags := []types.Tag{
		{Key: aws.String("key1"), Value: nil},
		{Key: nil, Value: aws.String("val2")},
	}

	m := extractTagMap(tags)
	if len(m) != 0 {
		t.Errorf("expected empty map for nil key/value tags, got %d entries", len(m))
	}
}
