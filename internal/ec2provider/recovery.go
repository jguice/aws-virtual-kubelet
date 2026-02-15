package ec2provider

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-virtual-kubelet/internal/awsutils"
	"github.com/aws/aws-virtual-kubelet/internal/metrics"
	"github.com/aws/aws-virtual-kubelet/internal/config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
)

// RecoveredPod holds a pod reconstructed from EC2 tags along with its instance metadata.
type RecoveredPod struct {
	Pod        *corev1.Pod
	InstanceID string
	PrivateIP  string
}

// PodRecovery handles recovering pod state from EC2 instance tags on VK restart.
type PodRecovery struct {
	ec2Client  awsutils.EC2API
	nodeName   string
	clusterName string
	mu         sync.Mutex
}

// NewPodRecovery creates a new PodRecovery instance.
func NewPodRecovery(ec2Client awsutils.EC2API, nodeName string) *PodRecovery {
	cfg := config.Config()
	return &PodRecovery{
		ec2Client:   ec2Client,
		nodeName:    nodeName,
		clusterName: cfg.ClusterName,
	}
}

// RecoverPods queries EC2 for running instances with VK pod tags and rebuilds pod objects.
// It returns pods that exist in EC2 but are NOT in the provided knownPodUIDs set.
func (r *PodRecovery) RecoverPods(ctx context.Context, knownPodUIDs map[k8stypes.UID]bool) ([]RecoveredPod, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	instances, err := r.queryTaggedInstances(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query EC2 for tagged instances: %w", err)
	}

	var recovered []RecoveredPod
	for _, inst := range instances {
		tags := extractTagMap(inst.Tags)

		podUID := k8stypes.UID(tags[awsutils.TagKeyPodUID])
		if podUID == "" {
			continue
		}

		// Skip pods that K8s already knows about
		if knownPodUIDs[podUID] {
			klog.V(1).Infof("Instance %s: pod UID %s already known to K8s, skipping",
				aws.ToString(inst.InstanceId), podUID)
			continue
		}

		pod := r.buildPodFromTags(tags, inst)
		if pod == nil {
			continue
		}

		recovered = append(recovered, RecoveredPod{
			Pod:        pod,
			InstanceID: aws.ToString(inst.InstanceId),
			PrivateIP:  aws.ToString(inst.PrivateIpAddress),
		})

		klog.Infof("Recovered pod %s/%s (UID=%s) from EC2 instance %s",
			pod.Namespace, pod.Name, pod.UID, aws.ToString(inst.InstanceId))
	}

	klog.Infof("Pod recovery complete: %d pods recovered from EC2 tags", len(recovered))
	return recovered, nil
}

// FindOrphanedInstances returns instance IDs that have VK pod tags but whose pod UIDs
// are not in the active pod set. These are candidates for cleanup.
func (r *PodRecovery) FindOrphanedInstances(ctx context.Context, activePodUIDs map[k8stypes.UID]bool) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	instances, err := r.queryTaggedInstances(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query EC2 for tagged instances: %w", err)
	}

	var orphaned []string
	for _, inst := range instances {
		tags := extractTagMap(inst.Tags)
		podUID := k8stypes.UID(tags[awsutils.TagKeyPodUID])

		if podUID == "" {
			// No pod UID tag — could be a warm pool instance, skip
			continue
		}

		if !activePodUIDs[podUID] {
			instanceID := aws.ToString(inst.InstanceId)
			klog.Warningf("Orphaned instance detected: %s (pod UID %s not in active set)",
				instanceID, podUID)
			orphaned = append(orphaned, instanceID)
		}
	}

	return orphaned, nil
}

// queryTaggedInstances fetches all running/pending EC2 instances tagged for this VK node.
func (r *PodRecovery) queryTaggedInstances(ctx context.Context) ([]types.Instance, error) {
	input := &ec2.DescribeInstancesInput{
		Filters: []types.Filter{
			{Name: aws.String("tag:" + awsutils.TagKeyClusterName), Values: []string{r.clusterName}},
			{Name: aws.String("instance-state-name"), Values: []string{"running", "pending"}},
		},
	}

	// Also filter by node name if set
	if r.nodeName != "" {
		input.Filters = append(input.Filters, types.Filter{
			Name:   aws.String("tag:" + awsutils.TagKeyNodeName),
			Values: []string{r.nodeName},
		})
	}

	var allInstances []types.Instance
	first := true
	for first || input.NextToken != nil {
		first = false
		resp, err := r.ec2Client.DescribeInstances(ctx, input)
		if err != nil {
			return nil, err
		}
		for _, reservation := range resp.Reservations {
			allInstances = append(allInstances, reservation.Instances...)
		}
		input.NextToken = resp.NextToken
	}

	return allInstances, nil
}

// buildPodFromTags creates a minimal corev1.Pod from EC2 instance tags.
func (r *PodRecovery) buildPodFromTags(tags map[string]string, inst types.Instance) *corev1.Pod {
	podName := tags[awsutils.TagKeyPodName]
	podNamespace := tags[awsutils.TagKeyPodNamespace]
	podUID := tags[awsutils.TagKeyPodUID]

	if podName == "" || podNamespace == "" || podUID == "" {
		klog.Warningf("Instance %s has incomplete pod tags (name=%q ns=%q uid=%q), skipping",
			aws.ToString(inst.InstanceId), podName, podNamespace, podUID)
		return nil
	}

	instanceID := aws.ToString(inst.InstanceId)
	privateIP := aws.ToString(inst.PrivateIpAddress)

	now := metav1.Now()
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: podNamespace,
			UID:       k8stypes.UID(podUID),
			Annotations: map[string]string{
				"compute.amazonaws.com/instance-id": instanceID,
				"compute.amazonaws.com/recovered":   "true",
			},
			CreationTimestamp: now,
		},
		Spec: corev1.PodSpec{
			// Minimal spec — the pod was recovered from EC2 tags, not K8s
			NodeName: r.nodeName,
			Containers: []corev1.Container{
				{Name: "recovered-workload"},
			},
		},
		Status: corev1.PodStatus{
			Phase:  corev1.PodRunning,
			PodIP:  privateIP,
			HostIP: privateIP,
			StartTime: &now,
			Conditions: []corev1.PodCondition{
				{
					Type:               corev1.PodReady,
					Status:             corev1.ConditionTrue,
					LastTransitionTime: now,
				},
			},
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:  "recovered-workload",
					Ready: true,
					State: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{StartedAt: now},
					},
				},
			},
		},
	}

	return pod
}

// extractTagMap converts a slice of EC2 tags into a map for easy lookup.
func extractTagMap(tags []types.Tag) map[string]string {
	m := make(map[string]string, len(tags))
	for _, t := range tags {
		if t.Key != nil && t.Value != nil {
			m[*t.Key] = *t.Value
		}
	}
	return m
}

// ReconciliationLoop runs periodic reconciliation to detect orphaned EC2 instances.
// It logs warnings for orphaned instances but does NOT auto-terminate them (safety first).
func (p *Ec2Provider) ReconciliationLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	klog.Infof("Starting reconciliation loop (interval=%v)", interval)

	for {
		select {
		case <-ctx.Done():
			klog.Info("Reconciliation loop stopped")
			return
		case <-ticker.C:
			p.reconcile(ctx)
		}
	}
}

// reconcile compares EC2 state with in-memory pod cache and reports discrepancies.
func (p *Ec2Provider) reconcile(ctx context.Context) {
	metrics.ReconciliationRuns.Inc()

	ec2Client, err := awsutils.NewEc2Client()
	if err != nil {
		klog.Errorf("Reconciliation: failed to create EC2 client: %v", err)
		metrics.ReconciliationErrors.Inc()
		return
	}

	recovery := NewPodRecovery(ec2Client, p.NodeName)

	// Build set of active pod UIDs and update active pods gauge
	activePodUIDs := make(map[k8stypes.UID]bool)
	podList := p.pods.GetList()
	for _, metaPod := range podList {
		activePodUIDs[metaPod.pod.UID] = true
	}
	metrics.ActivePods.Set(float64(len(podList)))

	orphaned, err := recovery.FindOrphanedInstances(ctx, activePodUIDs)
	if err != nil {
		klog.Errorf("Reconciliation: failed to find orphaned instances: %v", err)
		metrics.ReconciliationErrors.Inc()
		return
	}

	if len(orphaned) > 0 {
		klog.Warningf("Reconciliation: found %d orphaned EC2 instances: %v", len(orphaned), orphaned)
		klog.Warning("Reconciliation: orphaned instances are NOT auto-terminated. " +
			"Manual cleanup required or enable auto-cleanup in config.")
		metrics.OrphanedInstancesDetected.Add(float64(len(orphaned)))
	} else {
		klog.V(1).Info("Reconciliation: no orphaned instances detected")
	}
}

// RecoverAndMerge runs EC2-based recovery and merges recovered pods into the provider's cache.
// Call this during startup after K8s cache rehydration.
func (p *Ec2Provider) RecoverAndMerge(ctx context.Context) error {
	ec2Client, err := awsutils.NewEc2Client()
	if err != nil {
		return fmt.Errorf("failed to create EC2 client for recovery: %w", err)
	}

	recovery := NewPodRecovery(ec2Client, p.NodeName)

	// Build set of known pod UIDs from the K8s-populated cache
	knownUIDs := make(map[k8stypes.UID]bool)
	if p.pods != nil {
		for _, metaPod := range p.pods.GetList() {
			knownUIDs[metaPod.pod.UID] = true
		}
	}

	recovered, err := recovery.RecoverPods(ctx, knownUIDs)
	if err != nil {
		return fmt.Errorf("EC2-based pod recovery failed: %w", err)
	}

	if len(recovered) == 0 {
		klog.Info("No additional pods recovered from EC2 tags")
		return nil
	}

	// Add recovered pods to cache
	if p.pods == nil {
		p.pods = NewPodCache()
	}
	for _, rp := range recovered {
		podKey := fmt.Sprintf("%s-%s", rp.Pod.Namespace, rp.Pod.Name)
		if existing := p.pods.Get(podKey); existing != nil {
			klog.V(1).Infof("Pod %s already in cache, skipping recovered version", podKey)
			continue
		}

		metaPod := NewMetaPod(rp.Pod, nil, p.podNotifier)
		p.pods.Set(podKey, metaPod)
		metrics.PodsRecovered.Inc()
		metrics.ActivePods.Inc()
		klog.Infof("Added recovered pod %s/%s to cache (instance=%s)",
			rp.Pod.Namespace, rp.Pod.Name, rp.InstanceID)
	}

	return nil
}
