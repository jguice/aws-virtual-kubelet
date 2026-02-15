/*
Pod status construction for the VKVMA agent.

Translates the native process state into a Kubernetes PodStatus that the
Virtual Kubelet provider can report back to the API server.
*/
package agent

import (
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BuildPodStatus constructs a Kubernetes PodStatus from the current process state.
func BuildPodStatus(pm *ProcessManager) *corev1.PodStatus {
	state := pm.State()
	now := metav1.Now()

	switch state {
	case StateIdle:
		return buildPendingStatus(now)
	case StateRunning:
		return buildRunningStatus(pm, now)
	case StateTerminated:
		return buildTerminatedStatus(pm, now)
	case StateFailed:
		return buildFailedStatus(pm, now)
	default:
		return buildPendingStatus(now)
	}
}

func buildPendingStatus(now metav1.Time) *corev1.PodStatus {
	return &corev1.PodStatus{
		Phase:   corev1.PodPending,
		Message: "waiting for workload launch",
		Conditions: []corev1.PodCondition{
			{
				Type:               corev1.PodScheduled,
				Status:             corev1.ConditionTrue,
				LastTransitionTime: now,
			},
			{
				Type:               corev1.PodInitialized,
				Status:             corev1.ConditionTrue,
				LastTransitionTime: now,
			},
			{
				Type:               corev1.ContainersReady,
				Status:             corev1.ConditionFalse,
				LastTransitionTime: now,
				Reason:             "ContainersNotReady",
			},
			{
				Type:               corev1.PodReady,
				Status:             corev1.ConditionFalse,
				LastTransitionTime: now,
				Reason:             "ContainersNotReady",
			},
		},
	}
}

func buildRunningStatus(pm *ProcessManager, now metav1.Time) *corev1.PodStatus {
	pod := pm.Pod()
	startTime := metav1.NewTime(pm.StartTime())

	status := &corev1.PodStatus{
		Phase:     corev1.PodRunning,
		Message:   "workload is running",
		StartTime: &startTime,
		Conditions: []corev1.PodCondition{
			{
				Type:               corev1.PodScheduled,
				Status:             corev1.ConditionTrue,
				LastTransitionTime: startTime,
			},
			{
				Type:               corev1.PodInitialized,
				Status:             corev1.ConditionTrue,
				LastTransitionTime: startTime,
			},
			{
				Type:               corev1.ContainersReady,
				Status:             corev1.ConditionTrue,
				LastTransitionTime: startTime,
			},
			{
				Type:               corev1.PodReady,
				Status:             corev1.ConditionTrue,
				LastTransitionTime: startTime,
			},
		},
	}

	// Build container statuses from pod spec
	if pod != nil {
		status.ContainerStatuses = buildRunningContainerStatuses(pod, pm.StartTime())
	}

	return status
}

func buildTerminatedStatus(pm *ProcessManager, now metav1.Time) *corev1.PodStatus {
	pod := pm.Pod()
	exitCode := pm.ExitCode()
	startTime := metav1.NewTime(pm.StartTime())

	phase := corev1.PodSucceeded
	reason := "Completed"
	if exitCode != 0 {
		phase = corev1.PodFailed
		reason = fmt.Sprintf("ExitCode:%d", exitCode)
	}

	status := &corev1.PodStatus{
		Phase:     phase,
		Message:   fmt.Sprintf("workload exited with code %d", exitCode),
		Reason:    reason,
		StartTime: &startTime,
		Conditions: []corev1.PodCondition{
			{
				Type:               corev1.PodScheduled,
				Status:             corev1.ConditionTrue,
				LastTransitionTime: startTime,
			},
			{
				Type:               corev1.PodInitialized,
				Status:             corev1.ConditionTrue,
				LastTransitionTime: startTime,
			},
			{
				Type:               corev1.ContainersReady,
				Status:             corev1.ConditionFalse,
				LastTransitionTime: now,
				Reason:             reason,
			},
			{
				Type:               corev1.PodReady,
				Status:             corev1.ConditionFalse,
				LastTransitionTime: now,
				Reason:             reason,
			},
		},
	}

	if pod != nil {
		status.ContainerStatuses = buildTerminatedContainerStatuses(pod, pm.StartTime(), exitCode)
	}

	return status
}

func buildFailedStatus(pm *ProcessManager, now metav1.Time) *corev1.PodStatus {
	pod := pm.Pod()
	exitErr := pm.ExitError()

	message := "workload failed to start"
	if exitErr != nil {
		message = fmt.Sprintf("workload failed: %v", exitErr)
	}

	status := &corev1.PodStatus{
		Phase:   corev1.PodFailed,
		Message: message,
		Reason:  "StartError",
		Conditions: []corev1.PodCondition{
			{
				Type:               corev1.PodScheduled,
				Status:             corev1.ConditionTrue,
				LastTransitionTime: now,
			},
			{
				Type:               corev1.ContainersReady,
				Status:             corev1.ConditionFalse,
				LastTransitionTime: now,
				Reason:             "StartError",
				Message:            message,
			},
			{
				Type:               corev1.PodReady,
				Status:             corev1.ConditionFalse,
				LastTransitionTime: now,
				Reason:             "StartError",
				Message:            message,
			},
		},
	}

	if pod != nil {
		status.ContainerStatuses = buildFailedContainerStatuses(pod, message)
	}

	return status
}

// Container status builders

func buildRunningContainerStatuses(pod *corev1.Pod, startTime time.Time) []corev1.ContainerStatus {
	statuses := make([]corev1.ContainerStatus, 0, len(pod.Spec.Containers))
	for _, c := range pod.Spec.Containers {
		statuses = append(statuses, corev1.ContainerStatus{
			Name:  c.Name,
			Ready: true,
			State: corev1.ContainerState{
				Running: &corev1.ContainerStateRunning{
					StartedAt: metav1.NewTime(startTime),
				},
			},
			Image:   c.Image,
			ImageID: c.Image,
		})
	}
	return statuses
}

func buildTerminatedContainerStatuses(pod *corev1.Pod, startTime time.Time, exitCode int) []corev1.ContainerStatus {
	statuses := make([]corev1.ContainerStatus, 0, len(pod.Spec.Containers))
	now := metav1.Now()
	for _, c := range pod.Spec.Containers {
		statuses = append(statuses, corev1.ContainerStatus{
			Name:  c.Name,
			Ready: false,
			State: corev1.ContainerState{
				Terminated: &corev1.ContainerStateTerminated{
					ExitCode:   int32(exitCode),
					StartedAt:  metav1.NewTime(startTime),
					FinishedAt: now,
				},
			},
			Image:   c.Image,
			ImageID: c.Image,
		})
	}
	return statuses
}

func buildFailedContainerStatuses(pod *corev1.Pod, message string) []corev1.ContainerStatus {
	statuses := make([]corev1.ContainerStatus, 0, len(pod.Spec.Containers))
	for _, c := range pod.Spec.Containers {
		statuses = append(statuses, corev1.ContainerStatus{
			Name:  c.Name,
			Ready: false,
			State: corev1.ContainerState{
				Waiting: &corev1.ContainerStateWaiting{
					Reason:  "StartError",
					Message: message,
				},
			},
			Image:   c.Image,
			ImageID: c.Image,
		})
	}
	return statuses
}
