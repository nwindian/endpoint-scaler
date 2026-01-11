package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=ep
// +kubebuilder:printcolumn:name="App",type=string,JSONPath=`.spec.appRef.name`
// +kubebuilder:printcolumn:name="Endpoints",type=integer,JSONPath=`.status.endpointCount`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// EndpointPolicy defines routing and scaling policies for application endpoints
type EndpointPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EndpointPolicySpec   `json:"spec,omitempty"`
	Status EndpointPolicyStatus `json:"status,omitempty"`
}

// EndpointPolicySpec defines the desired state
type EndpointPolicySpec struct {
	// AppRef references the main application service
	AppRef AppReference `json:"appRef"`

	// GatewayRef references the Gateway for routing
	GatewayRef GatewayReference `json:"gatewayRef"`

	// Endpoints defines the list of endpoint configurations
	// +kubebuilder:validation:MinItems=1
	Endpoints []EndpointSpec `json:"endpoints"`
}

// AppReference identifies the main application
type AppReference struct {
	// Name of the main application service
	Name string `json:"name"`

	// Namespace of the application (defaults to policy namespace)
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Port is the service port (external-facing)
	// +kubebuilder:default=80
	Port int32 `json:"port,omitempty"`

	// ContainerPort is the port the container listens on
	// +kubebuilder:default=8080
	ContainerPort int32 `json:"containerPort,omitempty"`

	// Image for endpoint-specific deployments (required)
	Image string `json:"image"`
}

// GatewayReference identifies the Gateway for routing
type GatewayReference struct {
	// Name of the Gateway
	Name string `json:"name"`

	// Namespace of the Gateway
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Hostname for the routes (e.g., "api.example.com")
	// +optional
	Hostname string `json:"hostname,omitempty"`
}

// EndpointSpec defines a single endpoint configuration
type EndpointSpec struct {
	// ID is the unique identifier for this endpoint
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	ID string `json:"id"`

	// Type is the protocol type: "http" or "grpc"
	// +kubebuilder:validation:Enum=http;grpc
	// +kubebuilder:default=http
	Type string `json:"type,omitempty"`

	// Match defines how traffic is routed to this endpoint
	Match MatchSpec `json:"match"`

	// Strategy defines routing strategy:
	// - "canary": split traffic (canaryWeight% to endpoint, rest to main)
	// - "primary": 100% to endpoint (endpoint exclusively handles this path)
	// +kubebuilder:validation:Enum=canary;primary
	// +kubebuilder:default=primary
	Strategy string `json:"strategy,omitempty"`

	// CanaryWeight is the percentage of traffic to endpoint (1-100)
	// Only used when strategy is "canary"
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:default=5
	// +optional
	CanaryWeight *int32 `json:"canaryWeight,omitempty"`

	// Resources defines compute resources for this endpoint's deployment
	// +optional
	Resources *ResourceSpec `json:"resources,omitempty"`

	// HPA defines autoscaling configuration
	// +optional
	HPA *HPASpec `json:"hpa,omitempty"`

	// Replicas is the desired number of replicas (ignored if HPA is set)
	// +kubebuilder:default=1
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`
}

// MatchSpec defines traffic matching rules
type MatchSpec struct {
	// Path for HTTP endpoints (prefix match)
	// +optional
	Path string `json:"path,omitempty"`

	// Service for gRPC endpoints (e.g., "payments.Payments")
	// +optional
	Service string `json:"service,omitempty"`

	// Method for gRPC endpoints (e.g., "Authorize")
	// +optional
	Method string `json:"method,omitempty"`
}

// ResourceSpec defines compute resource limits
type ResourceSpec struct {
	// CPULimit is the CPU limit (e.g., "1", "500m", "2")
	// +optional
	CPULimit string `json:"cpuLimit,omitempty"`

	// CPURequest is the CPU request
	// +optional
	CPURequest string `json:"cpuRequest,omitempty"`

	// MemLimit is the memory limit (e.g., "512Mi", "1Gi")
	// +optional
	MemLimit string `json:"memLimit,omitempty"`

	// MemRequest is the memory request
	// +optional
	MemRequest string `json:"memRequest,omitempty"`
}

// HPASpec defines horizontal pod autoscaler configuration
type HPASpec struct {
	// Min is the minimum number of replicas
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	Min int32 `json:"min"`

	// Max is the maximum number of replicas
	// +kubebuilder:validation:Minimum=1
	Max int32 `json:"max"`

	// CPUTarget is the target CPU utilization percentage
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	// +optional
	CPUTarget *int32 `json:"cpuTarget,omitempty"`

	// MemoryTarget is the target memory utilization percentage
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	// +optional
	MemoryTarget *int32 `json:"memoryTarget,omitempty"`
}

// EndpointPolicyStatus defines the observed state
type EndpointPolicyStatus struct {
	// EndpointCount is the number of configured endpoints
	EndpointCount int `json:"endpointCount,omitempty"`

	// Conditions represent the current state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// EndpointStatuses contains status for each endpoint
	// +optional
	EndpointStatuses []EndpointStatus `json:"endpointStatuses,omitempty"`
}

// EndpointStatus represents the status of a single endpoint
type EndpointStatus struct {
	// ID of the endpoint
	ID string `json:"id"`

	// Ready indicates if the endpoint is ready
	Ready bool `json:"ready"`

	// DeploymentName is the name of the created deployment
	DeploymentName string `json:"deploymentName,omitempty"`

	// ServiceName is the name of the created service
	ServiceName string `json:"serviceName,omitempty"`

	// RouteName is the name of the created HTTPRoute/GRPCRoute
	RouteName string `json:"routeName,omitempty"`

	// Message contains additional status information
	Message string `json:"message,omitempty"`
}

// +kubebuilder:object:root=true

// EndpointPolicyList contains a list of EndpointPolicy
type EndpointPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EndpointPolicy `json:"items"`
}
