package ec2provider

import (
	"sync"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func testPodWithName(name, namespace string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func TestNewPodCache(t *testing.T) {
	pc := NewPodCache()
	if pc == nil {
		t.Fatal("expected non-nil PodCache")
	}
	if len(pc.pods) != 0 {
		t.Errorf("expected empty cache, got %d entries", len(pc.pods))
	}
}

func TestPodCacheSetAndGet(t *testing.T) {
	pc := NewPodCache()
	pod := testPodWithName("test-pod", "default")
	mp := NewMetaPod(pod, nil, nil)

	pc.Set("default-test-pod", mp)

	got := pc.Get("default-test-pod")
	if got == nil {
		t.Fatal("expected to get MetaPod back")
	}
	if got.pod.Name != "test-pod" {
		t.Errorf("expected pod name 'test-pod', got %q", got.pod.Name)
	}
}

func TestPodCacheGetNonExistent(t *testing.T) {
	pc := NewPodCache()
	got := pc.Get("nonexistent")
	if got != nil {
		t.Errorf("expected nil for nonexistent key, got %v", got)
	}
}

func TestPodCacheDelete(t *testing.T) {
	pc := NewPodCache()
	pod := testPodWithName("pod1", "ns1")
	pc.Set("ns1-pod1", NewMetaPod(pod, nil, nil))

	pc.Delete("ns1-pod1")

	if pc.Get("ns1-pod1") != nil {
		t.Error("expected nil after delete")
	}
}

func TestPodCacheDeleteNonExistent(t *testing.T) {
	pc := NewPodCache()
	// Should not panic
	pc.Delete("nonexistent")
}

func TestPodCacheGetList(t *testing.T) {
	pc := NewPodCache()
	pc.Set("key1", NewMetaPod(testPodWithName("pod1", "ns1"), nil, nil))
	pc.Set("key2", NewMetaPod(testPodWithName("pod2", "ns2"), nil, nil))

	list := pc.GetList()
	if len(list) != 2 {
		t.Errorf("expected 2 MetaPods, got %d", len(list))
	}
}

func TestPodCacheGetPodList(t *testing.T) {
	pc := NewPodCache()
	pc.Set("key1", NewMetaPod(testPodWithName("pod1", "ns1"), nil, nil))
	pc.Set("key2", NewMetaPod(testPodWithName("pod2", "ns2"), nil, nil))

	podList := pc.GetPodList()
	if len(podList) != 2 {
		t.Errorf("expected 2 pods, got %d", len(podList))
	}
}

func TestPodCacheUpdatePod(t *testing.T) {
	pc := NewPodCache()
	pod := testPodWithName("pod1", "ns1")
	pc.Set("ns1-pod1", NewMetaPod(pod, nil, nil))

	updatedPod := testPodWithName("pod1", "ns1")
	updatedPod.Labels = map[string]string{"updated": "true"}

	err := pc.UpdatePod("ns1-pod1", updatedPod)
	if err != nil {
		t.Fatalf("UpdatePod failed: %v", err)
	}

	got := pc.Get("ns1-pod1")
	if got.pod.Labels["updated"] != "true" {
		t.Error("expected pod to be updated with new labels")
	}
}

func TestPodCacheUpdatePodNonExistent(t *testing.T) {
	pc := NewPodCache()
	err := pc.UpdatePod("nonexistent", testPodWithName("pod1", "ns1"))
	if err == nil {
		t.Error("expected error when updating nonexistent key")
	}
}

func TestPodCachePopulate(t *testing.T) {
	pc := NewPodCache()
	podList := &corev1.PodList{
		Items: []corev1.Pod{
			{ObjectMeta: metav1.ObjectMeta{Name: "pod1", Namespace: "ns1"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "pod2", Namespace: "ns2"}},
		},
	}

	pc.Populate(podList)

	if pc.Get("ns1-pod1") == nil {
		t.Error("expected pod1 to be populated")
	}
	if pc.Get("ns2-pod2") == nil {
		t.Error("expected pod2 to be populated")
	}
}

func TestPodCacheConcurrentAccess(t *testing.T) {
	pc := NewPodCache()
	var wg sync.WaitGroup

	// Concurrent writes
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			pod := testPodWithName("pod", "ns")
			pc.Set("key", NewMetaPod(pod, nil, nil))
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = pc.Get("key")
		}()
	}

	wg.Wait()
}
