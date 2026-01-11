package controller

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	esv1alpha1 "github.com/example/endpoint-scaler/controller/pkg/apis/endpointscaler/v1alpha1"
)

func testEndpointPolicy() *esv1alpha1.EndpointPolicy {
	return &esv1alpha1.EndpointPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
		},
		Spec: esv1alpha1.EndpointPolicySpec{
			AppRef: esv1alpha1.AppReference{
				Name:  "my-app",
				Port:  8080,
				Image: "my-app:v1",
			},
			GatewayRef: esv1alpha1.GatewayReference{
				Name:      "my-gateway",
				Namespace: "gateway-ns",
			},
			Endpoints: []esv1alpha1.EndpointSpec{
				{
					ID:   "lookup",
					Type: "http",
					Match: esv1alpha1.MatchSpec{
						Path: "/api/lookup",
					},
					Strategy: "canary",
					CanaryWeight: func() *int32 {
						v := int32(10)
						return &v
					}(),
				},
				{
					ID:   "fallback-endpoint",
					Type: "http",
					Match: esv1alpha1.MatchSpec{
						Path: "/api/fallback",
					},
					Strategy: "fallback",
				},
			},
		},
	}
}

func testGRPCEndpointPolicy() *esv1alpha1.EndpointPolicy {
	return &esv1alpha1.EndpointPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-grpc-policy",
			Namespace: "default",
		},
		Spec: esv1alpha1.EndpointPolicySpec{
			AppRef: esv1alpha1.AppReference{
				Name:  "my-grpc-app",
				Port:  9090,
				Image: "my-grpc-app:v1",
			},
			GatewayRef: esv1alpha1.GatewayReference{
				Name: "my-gateway",
			},
			Endpoints: []esv1alpha1.EndpointSpec{
				{
					ID:   "user-service",
					Type: "grpc",
					Match: esv1alpha1.MatchSpec{
						Service: "com.example.UserService",
						Method:  "GetUser",
					},
					Strategy: "canary",
					CanaryWeight: func() *int32 {
						v := int32(5)
						return &v
					}(),
				},
			},
		},
	}
}

func TestBuildHTTPRoute_Canary(t *testing.T) {
	r := &EndpointPolicyReconciler{}
	policy := testEndpointPolicy()
	endpoint := &policy.Spec.Endpoints[0] // canary endpoint

	route := r.buildHTTPRoute(policy, endpoint)

	// Check name
	expectedName := "my-app-lookup"
	if route.Name != expectedName {
		t.Errorf("expected name %q, got %q", expectedName, route.Name)
	}

	// Check namespace
	if route.Namespace != "default" {
		t.Errorf("expected namespace 'default', got %q", route.Namespace)
	}

	// Check parent refs
	if len(route.Spec.ParentRefs) != 1 {
		t.Fatalf("expected 1 parent ref, got %d", len(route.Spec.ParentRefs))
	}
	if string(route.Spec.ParentRefs[0].Name) != "my-gateway" {
		t.Errorf("expected gateway 'my-gateway', got %q", route.Spec.ParentRefs[0].Name)
	}

	// Check rules
	if len(route.Spec.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(route.Spec.Rules))
	}

	rule := route.Spec.Rules[0]

	// Check path match
	if len(rule.Matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(rule.Matches))
	}
	if *rule.Matches[0].Path.Type != gatewayv1.PathMatchPathPrefix {
		t.Error("expected PathMatchPathPrefix")
	}
	if *rule.Matches[0].Path.Value != "/api/lookup" {
		t.Errorf("expected path '/api/lookup', got %q", *rule.Matches[0].Path.Value)
	}

	// Check backend refs - should have 2 for canary
	if len(rule.BackendRefs) != 2 {
		t.Fatalf("expected 2 backend refs for canary, got %d", len(rule.BackendRefs))
	}

	// First backend should be main service with weight 90
	mainBackend := rule.BackendRefs[0]
	if string(mainBackend.Name) != "my-app-svc" {
		t.Errorf("expected main backend 'my-app-svc', got %q", mainBackend.Name)
	}
	if *mainBackend.Weight != 90 {
		t.Errorf("expected main backend weight 90, got %d", *mainBackend.Weight)
	}

	// Second backend should be endpoint service with weight 10
	endpointBackend := rule.BackendRefs[1]
	if string(endpointBackend.Name) != "my-app-lookup-svc" {
		t.Errorf("expected endpoint backend 'my-app-lookup-svc', got %q", endpointBackend.Name)
	}
	if *endpointBackend.Weight != 10 {
		t.Errorf("expected endpoint backend weight 10, got %d", *endpointBackend.Weight)
	}
}

func TestBuildHTTPRoute_Fallback(t *testing.T) {
	r := &EndpointPolicyReconciler{}
	policy := testEndpointPolicy()
	endpoint := &policy.Spec.Endpoints[1] // fallback endpoint

	route := r.buildHTTPRoute(policy, endpoint)

	// Check backend refs - should have 1 for fallback (100% to endpoint)
	if len(route.Spec.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(route.Spec.Rules))
	}

	rule := route.Spec.Rules[0]
	if len(rule.BackendRefs) != 1 {
		t.Fatalf("expected 1 backend ref for fallback, got %d", len(rule.BackendRefs))
	}

	// Backend should be ENDPOINT service with weight 100
	// Fallback means endpoint handles this path exclusively
	backend := rule.BackendRefs[0]
	if string(backend.Name) != "my-app-fallback-endpoint-svc" {
		t.Errorf("expected backend 'my-app-fallback-endpoint-svc', got %q", backend.Name)
	}
	if *backend.Weight != 100 {
		t.Errorf("expected backend weight 100, got %d", *backend.Weight)
	}
}

func TestBuildHTTPBackendRefs_DefaultCanaryWeight(t *testing.T) {
	r := &EndpointPolicyReconciler{}
	policy := testEndpointPolicy()
	endpoint := &esv1alpha1.EndpointSpec{
		ID:       "test",
		Type:     "http",
		Strategy: "canary",
		// CanaryWeight is nil, should default to 5
	}

	refs := r.buildHTTPBackendRefs(policy, endpoint)

	if len(refs) != 2 {
		t.Fatalf("expected 2 backend refs, got %d", len(refs))
	}

	// Main should have weight 95 (100-5)
	if *refs[0].Weight != 95 {
		t.Errorf("expected main weight 95, got %d", *refs[0].Weight)
	}

	// Endpoint should have weight 5
	if *refs[1].Weight != 5 {
		t.Errorf("expected endpoint weight 5, got %d", *refs[1].Weight)
	}
}

func TestBuildGRPCRoute(t *testing.T) {
	r := &EndpointPolicyReconciler{}
	policy := testGRPCEndpointPolicy()
	endpoint := &policy.Spec.Endpoints[0]

	route := r.buildGRPCRoute(policy, endpoint)

	// Check name
	expectedName := "my-grpc-app-user-service"
	if route.Name != expectedName {
		t.Errorf("expected name %q, got %q", expectedName, route.Name)
	}

	// Check parent refs
	if len(route.Spec.ParentRefs) != 1 {
		t.Fatalf("expected 1 parent ref, got %d", len(route.Spec.ParentRefs))
	}

	// Check rules
	if len(route.Spec.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(route.Spec.Rules))
	}

	rule := route.Spec.Rules[0]

	// Check method match
	if len(rule.Matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(rule.Matches))
	}
	if rule.Matches[0].Method == nil {
		t.Fatal("expected method match to be set")
	}
	if *rule.Matches[0].Method.Service != "com.example.UserService" {
		t.Errorf("expected service 'com.example.UserService', got %q", *rule.Matches[0].Method.Service)
	}
	if *rule.Matches[0].Method.Method != "GetUser" {
		t.Errorf("expected method 'GetUser', got %q", *rule.Matches[0].Method.Method)
	}

	// Check backend refs - should have 2 for canary
	if len(rule.BackendRefs) != 2 {
		t.Fatalf("expected 2 backend refs for canary, got %d", len(rule.BackendRefs))
	}

	// Main should have weight 95
	if *rule.BackendRefs[0].Weight != 95 {
		t.Errorf("expected main weight 95, got %d", *rule.BackendRefs[0].Weight)
	}

	// Endpoint should have weight 5
	if *rule.BackendRefs[1].Weight != 5 {
		t.Errorf("expected endpoint weight 5, got %d", *rule.BackendRefs[1].Weight)
	}
}

func TestBuildGRPCBackendRefs_Fallback(t *testing.T) {
	r := &EndpointPolicyReconciler{}
	policy := testGRPCEndpointPolicy()
	endpoint := &esv1alpha1.EndpointSpec{
		ID:       "test",
		Type:     "grpc",
		Strategy: "fallback",
	}

	refs := r.buildGRPCBackendRefs(policy, endpoint)

	if len(refs) != 1 {
		t.Fatalf("expected 1 backend ref for fallback, got %d", len(refs))
	}

	// Fallback routes 100% to endpoint service
	if string(refs[0].Name) != "my-grpc-app-test-svc" {
		t.Errorf("expected endpoint service 'my-grpc-app-test-svc', got %q", refs[0].Name)
	}
	if *refs[0].Weight != 100 {
		t.Errorf("expected weight 100, got %d", *refs[0].Weight)
	}
}

func TestBuildHTTPRoute_WithHostname(t *testing.T) {
	r := &EndpointPolicyReconciler{}
	policy := testEndpointPolicy()
	policy.Spec.GatewayRef.Hostname = "api.example.com"
	endpoint := &policy.Spec.Endpoints[0]

	route := r.buildHTTPRoute(policy, endpoint)

	if len(route.Spec.Hostnames) != 1 {
		t.Fatalf("expected 1 hostname, got %d", len(route.Spec.Hostnames))
	}
	if string(route.Spec.Hostnames[0]) != "api.example.com" {
		t.Errorf("expected hostname 'api.example.com', got %q", route.Spec.Hostnames[0])
	}
}

func TestBuildHTTPRoute_Primary(t *testing.T) {
	r := &EndpointPolicyReconciler{}
	policy := testEndpointPolicy()
	endpoint := &esv1alpha1.EndpointSpec{
		ID:       "primary-endpoint",
		Type:     "http",
		Strategy: "primary",
		Match: esv1alpha1.MatchSpec{
			Path: "/api/primary",
		},
	}

	route := r.buildHTTPRoute(policy, endpoint)

	rule := route.Spec.Rules[0]
	if len(rule.BackendRefs) != 1 {
		t.Fatalf("expected 1 backend ref for primary, got %d", len(rule.BackendRefs))
	}

	// Primary routes 100% to endpoint service
	backend := rule.BackendRefs[0]
	if string(backend.Name) != "my-app-primary-endpoint-svc" {
		t.Errorf("expected backend 'my-app-primary-endpoint-svc', got %q", backend.Name)
	}
	if *backend.Weight != 100 {
		t.Errorf("expected weight 100, got %d", *backend.Weight)
	}
}

func TestEndpointResourceName(t *testing.T) {
	policy := testEndpointPolicy()
	endpoint := &policy.Spec.Endpoints[0]

	name := endpointResourceName(policy, endpoint)
	expected := "my-app-lookup"

	if name != expected {
		t.Errorf("expected %q, got %q", expected, name)
	}
}

func TestEndpointServiceName(t *testing.T) {
	policy := testEndpointPolicy()
	endpoint := &policy.Spec.Endpoints[0]

	name := endpointServiceName(policy, endpoint)
	expected := "my-app-lookup-svc"

	if name != expected {
		t.Errorf("expected %q, got %q", expected, name)
	}
}

func TestMainServiceName(t *testing.T) {
	policy := testEndpointPolicy()

	name := mainServiceName(policy)
	expected := "my-app-svc"

	if name != expected {
		t.Errorf("expected %q, got %q", expected, name)
	}
}
