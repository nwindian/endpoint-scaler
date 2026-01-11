package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	esv1alpha1 "github.com/example/endpoint-scaler/controller/pkg/apis/endpointscaler/v1alpha1"
)

const (
	StrategyCanary  = "canary"
	StrategyPrimary = "primary"
)

func (r *EndpointPolicyReconciler) reconcileRoute(
	ctx context.Context,
	policy *esv1alpha1.EndpointPolicy,
	endpoint *esv1alpha1.EndpointSpec,
) (string, error) {
	if policy.Spec.GatewayRef.Name == "" {
		return "", fmt.Errorf("gatewayRef.name is required")
	}

	strategy := endpoint.Strategy
	if strategy == "" {
		strategy = StrategyPrimary
	}
	if strategy == StrategyCanary {
		if err := r.validateMainServiceExists(ctx, policy); err != nil {
			return "", err
		}
	}

	endpointType := endpoint.Type
	if endpointType == "" {
		endpointType = "http"
	}

	switch endpointType {
	case "grpc":
		return r.reconcileGRPCRoute(ctx, policy, endpoint)
	default:
		return r.reconcileHTTPRoute(ctx, policy, endpoint)
	}
}

func (r *EndpointPolicyReconciler) validateMainServiceExists(
	ctx context.Context,
	policy *esv1alpha1.EndpointPolicy,
) error {
	mainSvc := mainServiceName(policy)
	svc := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      mainSvc,
		Namespace: policy.Namespace,
	}, svc)
	if err != nil {
		return fmt.Errorf("canary strategy requires main service %q to exist: %w", mainSvc, err)
	}
	return nil
}

func (r *EndpointPolicyReconciler) reconcileHTTPRoute(
	ctx context.Context,
	policy *esv1alpha1.EndpointPolicy,
	endpoint *esv1alpha1.EndpointSpec,
) (string, error) {
	logger := log.FromContext(ctx)
	name := endpointResourceName(policy, endpoint)

	desired := r.buildHTTPRoute(policy, endpoint)
	if err := ctrl.SetControllerReference(policy, desired, r.Scheme); err != nil {
		return "", err
	}

	existing := &gatewayv1.HTTPRoute{}
	err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: policy.Namespace}, existing)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			return "", err
		}
		logger.Info("Creating HTTPRoute", "name", name)
		return name, r.Create(ctx, desired)
	}

	existing.Spec = desired.Spec
	existing.Labels = desired.Labels
	logger.Info("Updating HTTPRoute", "name", name)
	return name, r.Update(ctx, existing)
}

func (r *EndpointPolicyReconciler) buildHTTPRoute(
	policy *esv1alpha1.EndpointPolicy,
	endpoint *esv1alpha1.EndpointSpec,
) *gatewayv1.HTTPRoute {
	name := endpointResourceName(policy, endpoint)
	labels := generateLabels(policy, endpoint)

	gatewayKind := gatewayv1.Kind("Gateway")
	parentRef := gatewayv1.ParentReference{
		Kind: &gatewayKind,
		Name: gatewayv1.ObjectName(policy.Spec.GatewayRef.Name),
	}
	if policy.Spec.GatewayRef.Namespace != "" {
		gatewayNS := gatewayv1.Namespace(policy.Spec.GatewayRef.Namespace)
		parentRef.Namespace = &gatewayNS
	}

	pathMatch := gatewayv1.PathMatchPathPrefix
	path := endpoint.Match.Path
	backendRefs := r.buildHTTPBackendRefs(policy, endpoint)

	route := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: policy.Namespace,
			Labels:    labels,
		},
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: []gatewayv1.ParentReference{parentRef},
			},
			Rules: []gatewayv1.HTTPRouteRule{{
				Matches: []gatewayv1.HTTPRouteMatch{{
					Path: &gatewayv1.HTTPPathMatch{
						Type:  &pathMatch,
						Value: &path,
					},
				}},
				BackendRefs: backendRefs,
			}},
		},
	}

	if policy.Spec.GatewayRef.Hostname != "" {
		route.Spec.Hostnames = []gatewayv1.Hostname{
			gatewayv1.Hostname(policy.Spec.GatewayRef.Hostname),
		}
	}

	return route
}

func (r *EndpointPolicyReconciler) buildHTTPBackendRefs(
	policy *esv1alpha1.EndpointPolicy,
	endpoint *esv1alpha1.EndpointSpec,
) []gatewayv1.HTTPBackendRef {
	mainSvc := mainServiceName(policy)
	endpointSvc := endpointServiceName(policy, endpoint)
	servicePort := gatewayv1.PortNumber(policy.Spec.AppRef.Port)
	if servicePort == 0 {
		servicePort = 80
	}

	kind := gatewayv1.Kind("Service")
	strategy := endpoint.Strategy
	if strategy == "" {
		strategy = StrategyPrimary
	}

	switch strategy {
	case StrategyCanary:
		canaryWeight := int32(5)
		if endpoint.CanaryWeight != nil {
			canaryWeight = *endpoint.CanaryWeight
		}
		mainWeight := int32(100 - canaryWeight)

		return []gatewayv1.HTTPBackendRef{
			{
				BackendRef: gatewayv1.BackendRef{
					BackendObjectReference: gatewayv1.BackendObjectReference{
						Kind: &kind,
						Name: gatewayv1.ObjectName(mainSvc),
						Port: &servicePort,
					},
					Weight: &mainWeight,
				},
			},
			{
				BackendRef: gatewayv1.BackendRef{
					BackendObjectReference: gatewayv1.BackendObjectReference{
						Kind: &kind,
						Name: gatewayv1.ObjectName(endpointSvc),
						Port: &servicePort,
					},
					Weight: &canaryWeight,
				},
			},
		}

	case StrategyPrimary:
		weight := int32(100)
		return []gatewayv1.HTTPBackendRef{{
			BackendRef: gatewayv1.BackendRef{
				BackendObjectReference: gatewayv1.BackendObjectReference{
					Kind: &kind,
					Name: gatewayv1.ObjectName(endpointSvc),
					Port: &servicePort,
				},
				Weight: &weight,
			},
		}}

	default:
		weight := int32(100)
		return []gatewayv1.HTTPBackendRef{{
			BackendRef: gatewayv1.BackendRef{
				BackendObjectReference: gatewayv1.BackendObjectReference{
					Kind: &kind,
					Name: gatewayv1.ObjectName(endpointSvc),
					Port: &servicePort,
				},
				Weight: &weight,
			},
		}}
	}
}

func (r *EndpointPolicyReconciler) reconcileGRPCRoute(
	ctx context.Context,
	policy *esv1alpha1.EndpointPolicy,
	endpoint *esv1alpha1.EndpointSpec,
) (string, error) {
	logger := log.FromContext(ctx)
	name := endpointResourceName(policy, endpoint)

	desired := r.buildGRPCRoute(policy, endpoint)
	if err := ctrl.SetControllerReference(policy, desired, r.Scheme); err != nil {
		return "", err
	}

	existing := &gatewayv1.GRPCRoute{}
	err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: policy.Namespace}, existing)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			return "", err
		}
		logger.Info("Creating GRPCRoute", "name", name)
		return name, r.Create(ctx, desired)
	}

	existing.Spec = desired.Spec
	existing.Labels = desired.Labels
	logger.Info("Updating GRPCRoute", "name", name)
	return name, r.Update(ctx, existing)
}

func (r *EndpointPolicyReconciler) buildGRPCRoute(
	policy *esv1alpha1.EndpointPolicy,
	endpoint *esv1alpha1.EndpointSpec,
) *gatewayv1.GRPCRoute {
	name := endpointResourceName(policy, endpoint)
	labels := generateLabels(policy, endpoint)

	gatewayKind := gatewayv1.Kind("Gateway")
	parentRef := gatewayv1.ParentReference{
		Kind: &gatewayKind,
		Name: gatewayv1.ObjectName(policy.Spec.GatewayRef.Name),
	}
	if policy.Spec.GatewayRef.Namespace != "" {
		gatewayNS := gatewayv1.Namespace(policy.Spec.GatewayRef.Namespace)
		parentRef.Namespace = &gatewayNS
	}

	grpcService := gatewayv1.GRPCMethodMatch{}
	if endpoint.Match.Service != "" {
		svc := endpoint.Match.Service
		grpcService.Service = &svc
	}
	if endpoint.Match.Method != "" {
		method := endpoint.Match.Method
		grpcService.Method = &method
	}

	backendRefs := r.buildGRPCBackendRefs(policy, endpoint)

	route := &gatewayv1.GRPCRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: policy.Namespace,
			Labels:    labels,
		},
		Spec: gatewayv1.GRPCRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: []gatewayv1.ParentReference{parentRef},
			},
			Rules: []gatewayv1.GRPCRouteRule{{
				Matches: []gatewayv1.GRPCRouteMatch{{
					Method: &grpcService,
				}},
				BackendRefs: backendRefs,
			}},
		},
	}

	if policy.Spec.GatewayRef.Hostname != "" {
		route.Spec.Hostnames = []gatewayv1.Hostname{
			gatewayv1.Hostname(policy.Spec.GatewayRef.Hostname),
		}
	}

	return route
}

func (r *EndpointPolicyReconciler) buildGRPCBackendRefs(
	policy *esv1alpha1.EndpointPolicy,
	endpoint *esv1alpha1.EndpointSpec,
) []gatewayv1.GRPCBackendRef {
	mainSvc := mainServiceName(policy)
	endpointSvc := endpointServiceName(policy, endpoint)
	servicePort := gatewayv1.PortNumber(policy.Spec.AppRef.Port)
	if servicePort == 0 {
		servicePort = 9090
	}

	kind := gatewayv1.Kind("Service")
	strategy := endpoint.Strategy
	if strategy == "" {
		strategy = StrategyPrimary
	}

	switch strategy {
	case StrategyCanary:
		canaryWeight := int32(5)
		if endpoint.CanaryWeight != nil {
			canaryWeight = *endpoint.CanaryWeight
		}
		mainWeight := int32(100 - canaryWeight)

		return []gatewayv1.GRPCBackendRef{
			{
				BackendRef: gatewayv1.BackendRef{
					BackendObjectReference: gatewayv1.BackendObjectReference{
						Kind: &kind,
						Name: gatewayv1.ObjectName(mainSvc),
						Port: &servicePort,
					},
					Weight: &mainWeight,
				},
			},
			{
				BackendRef: gatewayv1.BackendRef{
					BackendObjectReference: gatewayv1.BackendObjectReference{
						Kind: &kind,
						Name: gatewayv1.ObjectName(endpointSvc),
						Port: &servicePort,
					},
					Weight: &canaryWeight,
				},
			},
		}

	case StrategyPrimary:
		weight := int32(100)
		return []gatewayv1.GRPCBackendRef{{
			BackendRef: gatewayv1.BackendRef{
				BackendObjectReference: gatewayv1.BackendObjectReference{
					Kind: &kind,
					Name: gatewayv1.ObjectName(endpointSvc),
					Port: &servicePort,
				},
				Weight: &weight,
			},
		}}

	default:
		weight := int32(100)
		return []gatewayv1.GRPCBackendRef{{
			BackendRef: gatewayv1.BackendRef{
				BackendObjectReference: gatewayv1.BackendObjectReference{
					Kind: &kind,
					Name: gatewayv1.ObjectName(endpointSvc),
					Port: &servicePort,
				},
				Weight: &weight,
			},
		}}
	}
}
