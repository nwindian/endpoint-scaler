package controller

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	esv1alpha1 "github.com/example/endpoint-scaler/controller/pkg/apis/endpointscaler/v1alpha1"
)

func TestBuildHPA(t *testing.T) {
	r := &EndpointPolicyReconciler{}
	cpuTarget := int32(70)
	memTarget := int32(80)
	policy := &esv1alpha1.EndpointPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
		},
		Spec: esv1alpha1.EndpointPolicySpec{
			AppRef: esv1alpha1.AppReference{
				Name: "my-app",
			},
			GatewayRef: esv1alpha1.GatewayReference{
				Name: "my-gateway",
			},
			Endpoints: []esv1alpha1.EndpointSpec{
				{
					ID: "lookup",
					HPA: &esv1alpha1.HPASpec{
						Min:          2,
						Max:          10,
						CPUTarget:    &cpuTarget,
						MemoryTarget: &memTarget,
					},
				},
			},
		},
	}
	endpoint := &policy.Spec.Endpoints[0]

	hpa := r.buildHPA(policy, endpoint)

	// Check name
	expectedName := "my-app-lookup"
	if hpa.Name != expectedName {
		t.Errorf("expected name %q, got %q", expectedName, hpa.Name)
	}

	// Check namespace
	if hpa.Namespace != "default" {
		t.Errorf("expected namespace 'default', got %q", hpa.Namespace)
	}

	// Check scale target ref
	if hpa.Spec.ScaleTargetRef.Kind != "Deployment" {
		t.Errorf("expected kind 'Deployment', got %q", hpa.Spec.ScaleTargetRef.Kind)
	}
	if hpa.Spec.ScaleTargetRef.Name != "my-app-lookup" {
		t.Errorf("expected target name 'my-app-lookup', got %q", hpa.Spec.ScaleTargetRef.Name)
	}

	// Check min/max replicas
	if *hpa.Spec.MinReplicas != 2 {
		t.Errorf("expected min replicas 2, got %d", *hpa.Spec.MinReplicas)
	}
	if hpa.Spec.MaxReplicas != 10 {
		t.Errorf("expected max replicas 10, got %d", hpa.Spec.MaxReplicas)
	}

	// Check metrics
	if len(hpa.Spec.Metrics) != 2 {
		t.Fatalf("expected 2 metrics, got %d", len(hpa.Spec.Metrics))
	}

	// Find CPU metric
	var cpuMetric, memMetric bool
	for _, metric := range hpa.Spec.Metrics {
		if metric.Resource.Name == corev1.ResourceCPU {
			cpuMetric = true
			if *metric.Resource.Target.AverageUtilization != 70 {
				t.Errorf("expected CPU target 70, got %d", *metric.Resource.Target.AverageUtilization)
			}
		}
		if metric.Resource.Name == corev1.ResourceMemory {
			memMetric = true
			if *metric.Resource.Target.AverageUtilization != 80 {
				t.Errorf("expected memory target 80, got %d", *metric.Resource.Target.AverageUtilization)
			}
		}
	}

	if !cpuMetric {
		t.Error("CPU metric not found")
	}
	if !memMetric {
		t.Error("memory metric not found")
	}
}

func TestBuildHPA_OnlyCPU(t *testing.T) {
	r := &EndpointPolicyReconciler{}
	cpuTarget := int32(70)
	policy := &esv1alpha1.EndpointPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
		},
		Spec: esv1alpha1.EndpointPolicySpec{
			AppRef: esv1alpha1.AppReference{
				Name: "my-app",
			},
			GatewayRef: esv1alpha1.GatewayReference{
				Name: "my-gateway",
			},
			Endpoints: []esv1alpha1.EndpointSpec{
				{
					ID: "lookup",
					HPA: &esv1alpha1.HPASpec{
						Min:       1,
						Max:       5,
						CPUTarget: &cpuTarget,
					},
				},
			},
		},
	}
	endpoint := &policy.Spec.Endpoints[0]

	hpa := r.buildHPA(policy, endpoint)

	if len(hpa.Spec.Metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(hpa.Spec.Metrics))
	}

	if hpa.Spec.Metrics[0].Resource.Name != corev1.ResourceCPU {
		t.Errorf("expected CPU metric, got %v", hpa.Spec.Metrics[0].Resource.Name)
	}
}

func TestBuildHPA_NoMetrics(t *testing.T) {
	r := &EndpointPolicyReconciler{}
	policy := &esv1alpha1.EndpointPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
		},
		Spec: esv1alpha1.EndpointPolicySpec{
			AppRef: esv1alpha1.AppReference{
				Name: "my-app",
			},
			GatewayRef: esv1alpha1.GatewayReference{
				Name: "my-gateway",
			},
			Endpoints: []esv1alpha1.EndpointSpec{
				{
					ID: "lookup",
					HPA: &esv1alpha1.HPASpec{
						Min: 1,
						Max: 5,
						// No CPU or memory targets
					},
				},
			},
		},
	}
	endpoint := &policy.Spec.Endpoints[0]

	hpa := r.buildHPA(policy, endpoint)

	// Should have empty metrics
	if len(hpa.Spec.Metrics) != 0 {
		t.Errorf("expected 0 metrics, got %d", len(hpa.Spec.Metrics))
	}
}
