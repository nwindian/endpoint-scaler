package controller

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	esv1alpha1 "github.com/example/endpoint-scaler/controller/pkg/apis/endpointscaler/v1alpha1"
)

func TestBuildService(t *testing.T) {
	r := &EndpointPolicyReconciler{}
	policy := &esv1alpha1.EndpointPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
		},
		Spec: esv1alpha1.EndpointPolicySpec{
			AppRef: esv1alpha1.AppReference{
				Name: "my-app",
				Port: 8080,
			},
			GatewayRef: esv1alpha1.GatewayReference{
				Name: "my-gateway",
			},
			Endpoints: []esv1alpha1.EndpointSpec{
				{
					ID: "lookup",
				},
			},
		},
	}
	endpoint := &policy.Spec.Endpoints[0]

	service := r.buildService(policy, endpoint)

	// Check name
	expectedName := "my-app-lookup-svc"
	if service.Name != expectedName {
		t.Errorf("expected name %q, got %q", expectedName, service.Name)
	}

	// Check namespace
	if service.Namespace != "default" {
		t.Errorf("expected namespace 'default', got %q", service.Namespace)
	}

	// Check type
	if service.Spec.Type != corev1.ServiceTypeClusterIP {
		t.Errorf("expected ServiceTypeClusterIP, got %v", service.Spec.Type)
	}

	// Check ports
	if len(service.Spec.Ports) != 1 {
		t.Fatalf("expected 1 port, got %d", len(service.Spec.Ports))
	}
	if service.Spec.Ports[0].Port != 8080 {
		t.Errorf("expected port 8080, got %d", service.Spec.Ports[0].Port)
	}
	if service.Spec.Ports[0].TargetPort.IntVal != 8080 {
		t.Errorf("expected target port 8080, got %d", service.Spec.Ports[0].TargetPort.IntVal)
	}

	// Check selector matches deployment labels
	expectedLabels := map[string]string{
		"app.kubernetes.io/name":       "my-app",
		"app.kubernetes.io/component":  "lookup",
		"app.kubernetes.io/managed-by": "endpoint-scaler",
		"endpointscaler.io/policy":     "test-policy",
		"endpointscaler.io/endpoint":   "lookup",
	}

	for key, expectedValue := range expectedLabels {
		if service.Spec.Selector[key] != expectedValue {
			t.Errorf("expected selector %s=%s, got %s", key, expectedValue, service.Spec.Selector[key])
		}
	}
}

func TestBuildService_DefaultPort(t *testing.T) {
	r := &EndpointPolicyReconciler{}
	policy := &esv1alpha1.EndpointPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
		},
		Spec: esv1alpha1.EndpointPolicySpec{
			AppRef: esv1alpha1.AppReference{
				Name: "my-app",
				// No port specified
			},
			GatewayRef: esv1alpha1.GatewayReference{
				Name: "my-gateway",
			},
			Endpoints: []esv1alpha1.EndpointSpec{
				{
					ID: "lookup",
				},
			},
		},
	}
	endpoint := &policy.Spec.Endpoints[0]

	service := r.buildService(policy, endpoint)

	// Check default port is 80
	if service.Spec.Ports[0].Port != 80 {
		t.Errorf("expected default port 80, got %d", service.Spec.Ports[0].Port)
	}
}
