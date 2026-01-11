package controller

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	esv1alpha1 "github.com/example/endpoint-scaler/controller/pkg/apis/endpointscaler/v1alpha1"
)

func TestBuildDeployment(t *testing.T) {
	r := &EndpointPolicyReconciler{}
	policy := &esv1alpha1.EndpointPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
		},
		Spec: esv1alpha1.EndpointPolicySpec{
			AppRef: esv1alpha1.AppReference{
				Name:  "my-app",
				Port:  8080,
				Image: "my-app:v1.0.0",
			},
			GatewayRef: esv1alpha1.GatewayReference{
				Name: "my-gateway",
			},
			Endpoints: []esv1alpha1.EndpointSpec{
				{
					ID:   "lookup",
					Type: "http",
					Match: esv1alpha1.MatchSpec{
						Path: "/api/lookup",
					},
					Resources: &esv1alpha1.ResourceSpec{
						CPULimit:   "2",
						CPURequest: "100m",
						MemLimit:   "512Mi",
						MemRequest: "128Mi",
					},
					HPA: &esv1alpha1.HPASpec{
						Min: 2,
						Max: 10,
					},
				},
			},
		},
	}
	endpoint := &policy.Spec.Endpoints[0]

	deployment, err := r.buildDeployment(policy, endpoint)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check name
	if deployment.Name != "my-app-lookup" {
		t.Errorf("expected name 'my-app-lookup', got %q", deployment.Name)
	}

	// Check namespace
	if deployment.Namespace != "default" {
		t.Errorf("expected namespace 'default', got %q", deployment.Namespace)
	}

	// Check replicas (should use HPA.Min when HPA is set)
	if *deployment.Spec.Replicas != 2 {
		t.Errorf("expected replicas 2 (from HPA.Min), got %d", *deployment.Spec.Replicas)
	}

	// Check container
	if len(deployment.Spec.Template.Spec.Containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(deployment.Spec.Template.Spec.Containers))
	}

	container := deployment.Spec.Template.Spec.Containers[0]

	// Check container name
	if container.Name != "lookup" {
		t.Errorf("expected container name 'lookup', got %q", container.Name)
	}

	// Check image
	if container.Image != "my-app:v1.0.0" {
		t.Errorf("expected image 'my-app:v1.0.0', got %q", container.Image)
	}

	// Check MICROAPI_GUARDRAIL env var
	foundGuardrail := false
	for _, env := range container.Env {
		if env.Name == "MICROAPI_GUARDRAIL" {
			foundGuardrail = true
			if env.Value != "lookup" {
				t.Errorf("expected MICROAPI_GUARDRAIL='lookup', got %q", env.Value)
			}
		}
	}
	if !foundGuardrail {
		t.Error("MICROAPI_GUARDRAIL env var not found")
	}

	// Check resources
	cpuLimit := container.Resources.Limits["cpu"]
	if cpuLimit.String() != "2" {
		t.Errorf("expected CPU limit '2', got %q", cpuLimit.String())
	}

	memLimit := container.Resources.Limits["memory"]
	if memLimit.String() != "512Mi" {
		t.Errorf("expected memory limit '512Mi', got %q", memLimit.String())
	}

	cpuRequest := container.Resources.Requests["cpu"]
	if cpuRequest.String() != "100m" {
		t.Errorf("expected CPU request '100m', got %q", cpuRequest.String())
	}

	memRequest := container.Resources.Requests["memory"]
	if memRequest.String() != "128Mi" {
		t.Errorf("expected memory request '128Mi', got %q", memRequest.String())
	}
}

func TestBuildDeployment_MissingImage(t *testing.T) {
	r := &EndpointPolicyReconciler{}
	policy := &esv1alpha1.EndpointPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
		},
		Spec: esv1alpha1.EndpointPolicySpec{
			AppRef: esv1alpha1.AppReference{
				Name: "my-app",
				// No image specified
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

	_, err := r.buildDeployment(policy, endpoint)

	// Should return error when image is not specified
	if err == nil {
		t.Error("expected error when image is not specified, got nil")
	}
}

func TestBuildDeployment_DefaultPort(t *testing.T) {
	r := &EndpointPolicyReconciler{}
	policy := &esv1alpha1.EndpointPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
		},
		Spec: esv1alpha1.EndpointPolicySpec{
			AppRef: esv1alpha1.AppReference{
				Name:  "my-app",
				Image: "my-app:v1.0.0",
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

	deployment, err := r.buildDeployment(policy, endpoint)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check default port
	container := deployment.Spec.Template.Spec.Containers[0]
	if len(container.Ports) != 1 {
		t.Fatalf("expected 1 port, got %d", len(container.Ports))
	}
	if container.Ports[0].ContainerPort != 8080 {
		t.Errorf("expected default port 8080, got %d", container.Ports[0].ContainerPort)
	}
}

func TestBuildDeployment_ExplicitReplicas(t *testing.T) {
	r := &EndpointPolicyReconciler{}
	replicas := int32(5)
	policy := &esv1alpha1.EndpointPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
		},
		Spec: esv1alpha1.EndpointPolicySpec{
			AppRef: esv1alpha1.AppReference{
				Name:  "my-app",
				Image: "my-app:v1.0.0",
			},
			GatewayRef: esv1alpha1.GatewayReference{
				Name: "my-gateway",
			},
			Endpoints: []esv1alpha1.EndpointSpec{
				{
					ID:       "lookup",
					Replicas: &replicas,
				},
			},
		},
	}
	endpoint := &policy.Spec.Endpoints[0]

	deployment, err := r.buildDeployment(policy, endpoint)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if *deployment.Spec.Replicas != 5 {
		t.Errorf("expected replicas 5, got %d", *deployment.Spec.Replicas)
	}
}

func TestBuildDeployment_Labels(t *testing.T) {
	r := &EndpointPolicyReconciler{}
	policy := &esv1alpha1.EndpointPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
		},
		Spec: esv1alpha1.EndpointPolicySpec{
			AppRef: esv1alpha1.AppReference{
				Name:  "my-app",
				Image: "my-app:v1.0.0",
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

	deployment, err := r.buildDeployment(policy, endpoint)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check labels
	expectedLabels := map[string]string{
		"app.kubernetes.io/name":       "my-app",
		"app.kubernetes.io/component":  "lookup",
		"app.kubernetes.io/managed-by": "endpoint-scaler",
		"endpointscaler.io/policy":     "test-policy",
		"endpointscaler.io/endpoint":   "lookup",
	}

	for key, expectedValue := range expectedLabels {
		if deployment.Labels[key] != expectedValue {
			t.Errorf("expected label %s=%s, got %s", key, expectedValue, deployment.Labels[key])
		}
	}

	// Check selector labels match pod template labels
	for key, value := range deployment.Spec.Selector.MatchLabels {
		if deployment.Spec.Template.Labels[key] != value {
			t.Errorf("selector label %s=%s doesn't match pod template label %s", key, value, deployment.Spec.Template.Labels[key])
		}
	}
}
