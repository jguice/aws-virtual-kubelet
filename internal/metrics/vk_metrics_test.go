/*
This sample, non-production-ready code contains a Virtual Kubelet EC2-based provider and example VM Agent implementation.
© 2021 Amazon Web Services, Inc. or its affiliates. All Rights Reserved.

This AWS Content is provided subject to the terms of the AWS Customer Agreement
available at http://aws.amazon.com/agreement or other written agreement between
Customer and either Amazon Web Services, Inc. or Amazon Web Services EMEA SARL or both.
*/

package metrics

import (
	"strconv"
	"strings"
	"testing"

	io_prometheus_client "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
)

// TestMetricsCounter tests the pod created metric
func TestMetricsCounter(t *testing.T) {
	cases := []struct {
		podCounter int
	}{
		{
			podCounter: 1,
		},
	}
	for i, tt := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			//increment pod created metric
			PodsLaunched.Inc()
			//get all metrics data
			metricFamilyList := GetMetricsData()
			//find PodCreated metrics value from the MetricsFamily list
			var podMetric io_prometheus_client.MetricFamily
			for i := 0; i < len(metricFamilyList); i++ {
				if strings.Compare(*metricFamilyList[i].Name, "vkec2_pods_created_total") == 0 {
					podMetric = *metricFamilyList[i]
					break
				}
			}
			assert.Equal(t, int(*podMetric.Metric[0].Counter.Value), tt.podCounter)
		})
	}
}

func findMetricFamily(families []*io_prometheus_client.MetricFamily, name string) *io_prometheus_client.MetricFamily {
	for _, f := range families {
		if f.GetName() == name {
			return f
		}
	}
	return nil
}

func TestHistogramMetrics(t *testing.T) {
	// Record some observations
	CreatePodDuration.Observe(1.5)
	CreatePodDuration.Observe(0.5)
	DeletePodDuration.Observe(2.0)

	families := GetMetricsData()

	createPod := findMetricFamily(families, "vkec2_create_pod_duration_seconds")
	if createPod == nil {
		t.Fatal("vkec2_create_pod_duration_seconds metric not found")
	}
	hist := createPod.Metric[0].Histogram
	if hist == nil {
		t.Fatal("expected histogram data")
	}
	if hist.GetSampleCount() != 2 {
		t.Errorf("expected 2 samples, got %d", hist.GetSampleCount())
	}

	deletePod := findMetricFamily(families, "vkec2_delete_pod_duration_seconds")
	if deletePod == nil {
		t.Fatal("vkec2_delete_pod_duration_seconds metric not found")
	}
}

func TestGaugeMetrics(t *testing.T) {
	// Set gauge values
	ActivePods.Set(5)
	WarmPoolReady.Set(3)
	WarmPoolProvisioning.Set(1)

	families := GetMetricsData()

	active := findMetricFamily(families, "vkec2_active_pods")
	if active == nil {
		t.Fatal("vkec2_active_pods metric not found")
	}
	if active.Metric[0].Gauge.GetValue() != 5 {
		t.Errorf("expected ActivePods=5, got %f", active.Metric[0].Gauge.GetValue())
	}

	ready := findMetricFamily(families, "vkec2_warmpool_ready_instances")
	if ready == nil {
		t.Fatal("vkec2_warmpool_ready_instances metric not found")
	}
	if ready.Metric[0].Gauge.GetValue() != 3 {
		t.Errorf("expected WarmPoolReady=3, got %f", ready.Metric[0].Gauge.GetValue())
	}
}

func TestGRPCCallDurationHistogramVec(t *testing.T) {
	GRPCCallDuration.WithLabelValues("LaunchApplication").Observe(0.25)
	GRPCCallDuration.WithLabelValues("TerminateApplication").Observe(0.5)

	families := GetMetricsData()
	grpcDuration := findMetricFamily(families, "vkec2_grpc_call_duration_seconds")
	if grpcDuration == nil {
		t.Fatal("vkec2_grpc_call_duration_seconds metric not found")
	}
	if len(grpcDuration.Metric) < 2 {
		t.Errorf("expected at least 2 label sets, got %d", len(grpcDuration.Metric))
	}
}

func TestRecoveryMetrics(t *testing.T) {
	PodsRecovered.Inc()
	PodsRecovered.Inc()
	OrphanedInstancesDetected.Add(3)
	ReconciliationRuns.Inc()

	families := GetMetricsData()

	recovered := findMetricFamily(families, "vkec2_pods_recovered_total")
	if recovered == nil {
		t.Fatal("vkec2_pods_recovered_total metric not found")
	}

	orphaned := findMetricFamily(families, "vkec2_orphaned_instances_detected_total")
	if orphaned == nil {
		t.Fatal("vkec2_orphaned_instances_detected_total metric not found")
	}

	reconciliation := findMetricFamily(families, "vkec2_reconciliation_runs_total")
	if reconciliation == nil {
		t.Fatal("vkec2_reconciliation_runs_total metric not found")
	}
}

func TestCircuitBreakerGaugeVec(t *testing.T) {
	CircuitBreakerState.WithLabelValues("launch-application").Set(0) // closed
	CircuitBreakerState.WithLabelValues("launch-application").Set(1) // open

	families := GetMetricsData()
	cb := findMetricFamily(families, "vkec2_circuit_breaker_state")
	if cb == nil {
		t.Fatal("vkec2_circuit_breaker_state metric not found")
	}
	// Should have value 1 (last set)
	if cb.Metric[0].Gauge.GetValue() != 1 {
		t.Errorf("expected circuit breaker state=1, got %f", cb.Metric[0].Gauge.GetValue())
	}
}
