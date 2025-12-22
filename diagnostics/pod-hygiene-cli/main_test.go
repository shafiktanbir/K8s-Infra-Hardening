package main

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestEvaluatePodHealth(t *testing.T) {
	tests := []struct {
		name          string
		pod           *corev1.Pod
		wantUnhealthy bool
		wantReason    string
	}{
		{
			name: "Healthy running pod",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "healthy-pod", Namespace: "default"},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name:  "app",
							Ready: true,
							State: corev1.ContainerState{
								Running: &corev1.ContainerStateRunning{},
							},
						},
					},
				},
			},
			wantUnhealthy: false,
			wantReason:    "",
		},
		{
			name: "Evicted pod",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "evicted-pod", Namespace: "default"},
				Status: corev1.PodStatus{
					Phase:  corev1.PodFailed,
					Reason: "Evicted",
				},
			},
			wantUnhealthy: true,
			wantReason:    "Evicted",
		},
		{
			name: "CrashLoopBackOff container",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "crashing-pod", Namespace: "default"},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name: "app",
							State: corev1.ContainerState{
								Waiting: &corev1.ContainerStateWaiting{
									Reason: "CrashLoopBackOff",
								},
							},
						},
					},
				},
			},
			wantUnhealthy: true,
			wantReason:    "CrashLoopBackOff",
		},
		{
			name: "ImagePullBackOff container",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "pull-fail-pod", Namespace: "default"},
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name: "app",
							State: corev1.ContainerState{
								Waiting: &corev1.ContainerStateWaiting{
									Reason: "ImagePullBackOff",
								},
							},
						},
					},
				},
			},
			wantUnhealthy: true,
			wantReason:    "ImagePullBackOff",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unhealthy, reason := evaluatePodHealth(tt.pod)
			if unhealthy != tt.wantUnhealthy {
				t.Errorf("evaluatePodHealth() unhealthy = %v, want %v", unhealthy, tt.wantUnhealthy)
			}
			if reason != tt.wantReason {
				t.Errorf("evaluatePodHealth() reason = %q, want %q", reason, tt.wantReason)
			}
		})
	}
}
