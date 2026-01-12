# EndpointScaler

A Kubernetes controller that creates per-endpoint deployments with independent scaling and routing. Uses Gateway API for traffic management.

## Overview

EndpointScaler lets you break out specific API endpoints from a monolithic application into independently scalable deployments. Each endpoint gets its own:

- Deployment (with dedicated resources)
- Service
- HTTPRoute or GRPCRoute
- HorizontalPodAutoscaler (optional)

Traffic routing is handled via Gateway API, supporting both primary (100% to endpoint) and canary (weighted split) strategies.

## Requirements

- Kubernetes 1.26+
- Gateway API CRDs installed
- A Gateway resource configured

## Installation

```bash
helm install endpoint-scaler ./charts/endpoint-scaler
```

## Usage

### Basic Example

```yaml
apiVersion: endpointscaler.io/v1alpha1
kind: EndpointPolicy
metadata:
  name: my-app-endpoints
spec:
  appRef:
    name: my-app
    image: my-app:v1.0.0
  gatewayRef:
    name: my-gateway
  endpoints:
    - id: compute
      type: http
      match:
        path: /api/v1/compute
      strategy: primary
      resources:
        cpuLimit: "4"
        memLimit: 1Gi
      hpa:
        min: 1
        max: 10
        cpuTarget: 80
```

### Canary Deployment

Route a percentage of traffic to the endpoint deployment while the rest goes to your main application:

```yaml
endpoints:
  - id: search
    type: http
    match:
      path: /api/v1/search
    strategy: canary
    canaryWeight: 10  # 10% to endpoint, 90% to main
    hpa:
      min: 1
      max: 20
      cpuTarget: 70
```

Canary strategy requires a main service named `{appRef.name}-svc` to exist.

### gRPC Endpoints

```yaml
endpoints:
  - id: payments
    type: grpc
    match:
      service: payments.PaymentService
      method: ProcessPayment
    strategy: primary
    hpa:
      min: 2
      max: 10
      cpuTarget: 75
```

## API Reference

### EndpointPolicySpec

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `appRef` | AppReference | Yes | Application configuration |
| `gatewayRef` | GatewayReference | Yes | Gateway for routing |
| `endpoints` | []EndpointSpec | Yes | List of endpoints (min 1) |

### AppReference

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | - | Application name (required) |
| `namespace` | string | policy namespace | Application namespace |
| `port` | int32 | 80 | Service port |
| `containerPort` | int32 | 8080 | Container port |
| `image` | string | - | Container image (required) |

### GatewayReference

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | - | Gateway name (required) |
| `namespace` | string | policy namespace | Gateway namespace |
| `hostname` | string | - | Hostname for routes |

### EndpointSpec

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `id` | string | - | Unique endpoint identifier (required) |
| `type` | string | http | Protocol: `http` or `grpc` |
| `match` | MatchSpec | - | Traffic matching rules (required) |
| `strategy` | string | primary | Routing: `primary` or `canary` |
| `canaryWeight` | int32 | 5 | Traffic percentage (1-100, canary only) |
| `resources` | ResourceSpec | - | CPU/memory limits |
| `hpa` | HPASpec | - | Autoscaling config |
| `replicas` | int32 | 1 | Replica count (ignored if HPA set) |

### MatchSpec

| Field | Type | Description |
|-------|------|-------------|
| `path` | string | HTTP path prefix (required for http type) |
| `service` | string | gRPC service name (required for grpc type) |
| `method` | string | gRPC method name (required for grpc type) |

### HPASpec

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `min` | int32 | 1 | Minimum replicas |
| `max` | int32 | - | Maximum replicas (required) |
| `cpuTarget` | int32 | - | Target CPU utilization % |
| `memoryTarget` | int32 | - | Target memory utilization % |

At least one of `cpuTarget` or `memoryTarget` is required when HPA is configured.

### ResourceSpec

| Field | Type | Description |
|-------|------|-------------|
| `cpuLimit` | string | CPU limit (e.g., "2", "500m") |
| `cpuRequest` | string | CPU request |
| `memLimit` | string | Memory limit (e.g., "1Gi", "512Mi") |
| `memRequest` | string | Memory request |

## Status

The controller reports status per-endpoint:

```yaml
status:
  endpointCount: 2
  conditions:
    - type: Ready
      status: "True"
      reason: AllEndpointsReady
      message: "All 2 endpoints ready"
  endpointStatuses:
    - id: compute
      ready: true
      deploymentName: my-app-compute
      serviceName: my-app-compute-svc
      routeName: my-app-compute-route
    - id: transform
      ready: true
      deploymentName: my-app-transform
      serviceName: my-app-transform-svc
      routeName: my-app-transform-route
```

## SDK

The Go SDK provides middleware for endpoint isolation:

```go
import "github.com/example/endpoint-scaler/sdk/go"

mux := http.NewServeMux()
mux.Handle("/api/v1/compute", endpointscaler.Guard("compute", computeHandler))
mux.Handle("/api/v1/search", endpointscaler.Guard("search", searchHandler))
```

The `Guard` middleware checks the `ENDPOINTSCALER_GUARDRAIL` environment variable (set by the controller) and only executes the handler if it matches the endpoint ID.

## Validation

The controller validates specs before reconciling:

- `appRef.name` and `appRef.image` required
- `gatewayRef.name` required
- At least one endpoint required
- Endpoint IDs must be unique
- HTTP endpoints require `match.path`
- gRPC endpoints require `match.service` and `match.method`
- HPA requires at least one metric target
- HPA `max` must be >= `min`
- Resource quantities must be valid Kubernetes formats

Invalid specs result in `Ready=False` with `Reason: ValidationFailed`.

## License

Apache 2.0
