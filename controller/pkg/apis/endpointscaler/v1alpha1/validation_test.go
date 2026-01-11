package v1alpha1

import (
	"strings"
	"testing"
)

func TestValidate_ValidSpec(t *testing.T) {
	spec := &EndpointPolicySpec{
		AppRef: AppReference{
			Name:  "my-app",
			Image: "my-app:v1.0.0",
		},
		GatewayRef: GatewayReference{
			Name: "my-gateway",
		},
		Endpoints: []EndpointSpec{
			{
				ID:   "lookup",
				Type: "http",
				Match: MatchSpec{
					Path: "/api/lookup",
				},
			},
		},
	}

	if err := spec.Validate(); err != nil {
		t.Errorf("expected valid spec, got error: %v", err)
	}
}

func TestValidate_AppRefRequired(t *testing.T) {
	tests := []struct {
		name    string
		appRef  AppReference
		wantErr string
	}{
		{
			name:    "missing name",
			appRef:  AppReference{Image: "my-app:v1"},
			wantErr: "appRef.name",
		},
		{
			name:    "missing image",
			appRef:  AppReference{Name: "my-app"},
			wantErr: "appRef.image",
		},
		{
			name:    "both missing",
			appRef:  AppReference{},
			wantErr: "appRef.name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := &EndpointPolicySpec{
				AppRef:     tt.appRef,
				GatewayRef: GatewayReference{Name: "gw"},
				Endpoints: []EndpointSpec{
					{ID: "ep1", Type: "http", Match: MatchSpec{Path: "/api"}},
				},
			}
			err := spec.Validate()
			if err == nil {
				t.Error("expected error, got nil")
				return
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestValidate_GatewayRefRequired(t *testing.T) {
	spec := &EndpointPolicySpec{
		AppRef: AppReference{Name: "my-app", Image: "img:v1"},
		GatewayRef: GatewayReference{
			Name: "",
		},
		Endpoints: []EndpointSpec{
			{ID: "ep1", Type: "http", Match: MatchSpec{Path: "/api"}},
		},
	}

	err := spec.Validate()
	if err == nil {
		t.Error("expected error for missing gateway name")
		return
	}
	if !strings.Contains(err.Error(), "gatewayRef.name") {
		t.Errorf("expected error about gatewayRef.name, got %v", err)
	}
}

func TestValidate_EndpointsRequired(t *testing.T) {
	spec := &EndpointPolicySpec{
		AppRef:     AppReference{Name: "my-app", Image: "img:v1"},
		GatewayRef: GatewayReference{Name: "gw"},
		Endpoints:  []EndpointSpec{},
	}

	err := spec.Validate()
	if err == nil {
		t.Error("expected error for empty endpoints")
		return
	}
	if !strings.Contains(err.Error(), "endpoints") {
		t.Errorf("expected error about endpoints, got %v", err)
	}
}

func TestValidate_DuplicateEndpointIDs(t *testing.T) {
	spec := &EndpointPolicySpec{
		AppRef:     AppReference{Name: "my-app", Image: "img:v1"},
		GatewayRef: GatewayReference{Name: "gw"},
		Endpoints: []EndpointSpec{
			{ID: "lookup", Type: "http", Match: MatchSpec{Path: "/api/v1"}},
			{ID: "lookup", Type: "http", Match: MatchSpec{Path: "/api/v2"}},
		},
	}

	err := spec.Validate()
	if err == nil {
		t.Error("expected error for duplicate endpoint IDs")
		return
	}
	if !strings.Contains(err.Error(), "Duplicate") {
		t.Errorf("expected Duplicate error, got %v", err)
	}
}

func TestValidate_HTTPRequiresPath(t *testing.T) {
	spec := &EndpointPolicySpec{
		AppRef:     AppReference{Name: "my-app", Image: "img:v1"},
		GatewayRef: GatewayReference{Name: "gw"},
		Endpoints: []EndpointSpec{
			{ID: "ep1", Type: "http", Match: MatchSpec{}},
		},
	}

	err := spec.Validate()
	if err == nil {
		t.Error("expected error for HTTP endpoint without path")
		return
	}
	if !strings.Contains(err.Error(), "match.path") {
		t.Errorf("expected error about match.path, got %v", err)
	}
}

func TestValidate_GRPCRequiresServiceAndMethod(t *testing.T) {
	tests := []struct {
		name    string
		match   MatchSpec
		wantErr string
	}{
		{
			name:    "missing both",
			match:   MatchSpec{},
			wantErr: "match.service",
		},
		{
			name:    "missing method",
			match:   MatchSpec{Service: "foo.Bar"},
			wantErr: "match.method",
		},
		{
			name:    "missing service",
			match:   MatchSpec{Method: "DoThing"},
			wantErr: "match.service",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := &EndpointPolicySpec{
				AppRef:     AppReference{Name: "my-app", Image: "img:v1"},
				GatewayRef: GatewayReference{Name: "gw"},
				Endpoints: []EndpointSpec{
					{ID: "ep1", Type: "grpc", Match: tt.match},
				},
			}
			err := spec.Validate()
			if err == nil {
				t.Error("expected error, got nil")
				return
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestValidate_GRPCValid(t *testing.T) {
	spec := &EndpointPolicySpec{
		AppRef:     AppReference{Name: "my-app", Image: "img:v1"},
		GatewayRef: GatewayReference{Name: "gw"},
		Endpoints: []EndpointSpec{
			{
				ID:   "payments",
				Type: "grpc",
				Match: MatchSpec{
					Service: "payments.Payments",
					Method:  "Authorize",
				},
			},
		},
	}

	if err := spec.Validate(); err != nil {
		t.Errorf("expected valid gRPC spec, got error: %v", err)
	}
}

func TestValidate_InvalidReplicas(t *testing.T) {
	zero := int32(0)
	spec := &EndpointPolicySpec{
		AppRef:     AppReference{Name: "my-app", Image: "img:v1"},
		GatewayRef: GatewayReference{Name: "gw"},
		Endpoints: []EndpointSpec{
			{
				ID:       "ep1",
				Type:     "http",
				Match:    MatchSpec{Path: "/api"},
				Replicas: &zero,
			},
		},
	}

	err := spec.Validate()
	if err == nil {
		t.Error("expected error for zero replicas")
		return
	}
	if !strings.Contains(err.Error(), "replicas") {
		t.Errorf("expected error about replicas, got %v", err)
	}
}

func TestValidate_InvalidResourceQuantity(t *testing.T) {
	spec := &EndpointPolicySpec{
		AppRef:     AppReference{Name: "my-app", Image: "img:v1"},
		GatewayRef: GatewayReference{Name: "gw"},
		Endpoints: []EndpointSpec{
			{
				ID:    "ep1",
				Type:  "http",
				Match: MatchSpec{Path: "/api"},
				Resources: &ResourceSpec{
					CPULimit: "invalid-cpu",
				},
			},
		},
	}

	err := spec.Validate()
	if err == nil {
		t.Error("expected error for invalid resource quantity")
		return
	}
	if !strings.Contains(err.Error(), "cpuLimit") {
		t.Errorf("expected error about cpuLimit, got %v", err)
	}
}

func TestValidate_ValidResourceQuantities(t *testing.T) {
	spec := &EndpointPolicySpec{
		AppRef:     AppReference{Name: "my-app", Image: "img:v1"},
		GatewayRef: GatewayReference{Name: "gw"},
		Endpoints: []EndpointSpec{
			{
				ID:    "ep1",
				Type:  "http",
				Match: MatchSpec{Path: "/api"},
				Resources: &ResourceSpec{
					CPULimit:   "2",
					CPURequest: "100m",
					MemLimit:   "1Gi",
					MemRequest: "256Mi",
				},
			},
		},
	}

	if err := spec.Validate(); err != nil {
		t.Errorf("expected valid resources, got error: %v", err)
	}
}

func TestValidate_HPAMaxLessThanMin(t *testing.T) {
	cpu := int32(80)
	spec := &EndpointPolicySpec{
		AppRef:     AppReference{Name: "my-app", Image: "img:v1"},
		GatewayRef: GatewayReference{Name: "gw"},
		Endpoints: []EndpointSpec{
			{
				ID:    "ep1",
				Type:  "http",
				Match: MatchSpec{Path: "/api"},
				HPA: &HPASpec{
					Min:       5,
					Max:       2,
					CPUTarget: &cpu,
				},
			},
		},
	}

	err := spec.Validate()
	if err == nil {
		t.Error("expected error for max < min")
		return
	}
	if !strings.Contains(err.Error(), "hpa.max") {
		t.Errorf("expected error about hpa.max, got %v", err)
	}
}

func TestValidate_HPANoMetrics(t *testing.T) {
	spec := &EndpointPolicySpec{
		AppRef:     AppReference{Name: "my-app", Image: "img:v1"},
		GatewayRef: GatewayReference{Name: "gw"},
		Endpoints: []EndpointSpec{
			{
				ID:    "ep1",
				Type:  "http",
				Match: MatchSpec{Path: "/api"},
				HPA: &HPASpec{
					Min: 1,
					Max: 10,
				},
			},
		},
	}

	err := spec.Validate()
	if err == nil {
		t.Error("expected error for HPA without metrics")
		return
	}
	if !strings.Contains(err.Error(), "cpuTarget") || !strings.Contains(err.Error(), "memoryTarget") {
		t.Errorf("expected error about cpuTarget/memoryTarget, got %v", err)
	}
}

func TestValidate_HPAValid(t *testing.T) {
	cpu := int32(80)
	mem := int32(70)
	spec := &EndpointPolicySpec{
		AppRef:     AppReference{Name: "my-app", Image: "img:v1"},
		GatewayRef: GatewayReference{Name: "gw"},
		Endpoints: []EndpointSpec{
			{
				ID:    "ep1",
				Type:  "http",
				Match: MatchSpec{Path: "/api"},
				HPA: &HPASpec{
					Min:          2,
					Max:          10,
					CPUTarget:    &cpu,
					MemoryTarget: &mem,
				},
			},
		},
	}

	if err := spec.Validate(); err != nil {
		t.Errorf("expected valid HPA spec, got error: %v", err)
	}
}

func TestValidate_DefaultTypeIsHTTP(t *testing.T) {
	spec := &EndpointPolicySpec{
		AppRef:     AppReference{Name: "my-app", Image: "img:v1"},
		GatewayRef: GatewayReference{Name: "gw"},
		Endpoints: []EndpointSpec{
			{
				ID:    "ep1",
				Type:  "", // empty defaults to http
				Match: MatchSpec{Path: "/api"},
			},
		},
	}

	if err := spec.Validate(); err != nil {
		t.Errorf("expected valid spec with default type, got error: %v", err)
	}
}

func TestValidate_MultipleEndpoints(t *testing.T) {
	cpu := int32(80)
	spec := &EndpointPolicySpec{
		AppRef:     AppReference{Name: "my-app", Image: "img:v1"},
		GatewayRef: GatewayReference{Name: "gw"},
		Endpoints: []EndpointSpec{
			{
				ID:    "lookup",
				Type:  "http",
				Match: MatchSpec{Path: "/api/lookup"},
			},
			{
				ID:    "search",
				Type:  "http",
				Match: MatchSpec{Path: "/api/search"},
				HPA: &HPASpec{
					Min:       1,
					Max:       5,
					CPUTarget: &cpu,
				},
			},
			{
				ID:   "payments",
				Type: "grpc",
				Match: MatchSpec{
					Service: "payments.Payments",
					Method:  "Process",
				},
			},
		},
	}

	if err := spec.Validate(); err != nil {
		t.Errorf("expected valid multi-endpoint spec, got error: %v", err)
	}
}
