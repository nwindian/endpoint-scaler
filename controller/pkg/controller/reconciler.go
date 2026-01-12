package controller

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	esv1alpha1 "github.com/example/endpoint-scaler/controller/pkg/apis/endpointscaler/v1alpha1"
)

const finalizerName = "endpointscaler.io/finalizer"

// EndpointPolicyReconciler reconciles EndpointPolicy resources
type EndpointPolicyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=endpointscaler.io,resources=endpointpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=endpointscaler.io,resources=endpointpolicies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=endpointscaler.io,resources=endpointpolicies/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes;grpcroutes,verbs=get;list;watch;create;update;patch;delete

func (r *EndpointPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	policy := &esv1alpha1.EndpointPolicy{}
	if err := r.Get(ctx, req.NamespacedName, policy); err != nil {
		if apierrors.IsNotFound(err) {
			RemovePolicyMetrics(req.Namespace, req.Name)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if err := policy.Spec.Validate(); err != nil {
		logger.Error(err, "spec validation failed")
		meta.SetStatusCondition(&policy.Status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			ObservedGeneration: policy.Generation,
			LastTransitionTime: metav1.Now(),
			Reason:             "ValidationFailed",
			Message:            err.Error(),
		})
		if statusErr := r.Status().Update(ctx, policy); statusErr != nil {
			logger.Error(statusErr, "failed to update status after validation failure")
			return ctrl.Result{}, statusErr
		}
		return ctrl.Result{}, nil
	}

	logger.Info("Reconciling EndpointPolicy",
		"name", policy.Name,
		"endpoints", len(policy.Spec.Endpoints))

	endpointStatuses := make([]esv1alpha1.EndpointStatus, 0, len(policy.Spec.Endpoints))

	labels := client.MatchingLabels{
		"endpointscaler.io/policy":     policy.Name,
		"app.kubernetes.io/managed-by": "endpoint-scaler",
	}

	listofdeployments := &appsv1.DeploymentList{}
	listofservices := &corev1.ServiceList{}
	listofroutes := &gatewayv1.HTTPRouteList{}
	listofgrpcroutes := &gatewayv1.GRPCRouteList{}
	listofhpas := &autoscalingv2.HorizontalPodAutoscalerList{}

	r.List(ctx, listofdeployments, client.InNamespace(policy.Namespace), labels)
	r.List(ctx, listofservices, client.InNamespace(policy.Namespace), labels)
	r.List(ctx, listofroutes, client.InNamespace(policy.Namespace), labels)
	r.List(ctx, listofgrpcroutes, client.InNamespace(policy.Namespace), labels)
	r.List(ctx, listofhpas, client.InNamespace(policy.Namespace), labels)

	desired := map[string]bool{}
	for _, endpoint := range policy.Spec.Endpoints {
		status := esv1alpha1.EndpointStatus{ID: endpoint.ID}

		deploymentName, err := r.reconcileDeployment(ctx, policy, &endpoint)
		if err != nil {
			logger.Error(err, "failed to reconcile Deployment", "endpoint", endpoint.ID)
			status.Message = fmt.Sprintf("Deployment error: %v", err)
			endpointStatuses = append(endpointStatuses, status)
			desired[endpoint.ID] = true
			continue
		}
		status.DeploymentName = deploymentName

		serviceName, err := r.reconcileService(ctx, policy, &endpoint)
		if err != nil {
			logger.Error(err, "failed to reconcile Service", "endpoint", endpoint.ID)
			status.Message = fmt.Sprintf("Service error: %v", err)
			endpointStatuses = append(endpointStatuses, status)
			desired[endpoint.ID] = true
			continue
		}
		status.ServiceName = serviceName

		routeName, err := r.reconcileRoute(ctx, policy, &endpoint)
		if err != nil {
			logger.Error(err, "failed to reconcile Route", "endpoint", endpoint.ID)
			status.Message = fmt.Sprintf("Route error: %v", err)
			endpointStatuses = append(endpointStatuses, status)
			desired[endpoint.ID] = true
			continue
		}
		status.RouteName = routeName

		if endpoint.HPA != nil {
			if err := r.reconcileHPA(ctx, policy, &endpoint); err != nil {
				logger.Error(err, "failed to reconcile HPA", "endpoint", endpoint.ID)
				status.Message = fmt.Sprintf("HPA error: %v", err)
				endpointStatuses = append(endpointStatuses, status)
				desired[endpoint.ID] = true
				continue
			}

		}

		status.Ready = true
		RecordEndpointInfo(policy.Namespace, policy.Name, endpoint.ID, endpoint.Type, endpoint.Strategy)
		endpointStatuses = append(endpointStatuses, status)
		desired[endpoint.ID] = true
	}

	for i := range listofdeployments.Items {
		dep := &listofdeployments.Items[i]
		eid := dep.Labels["endpointscaler.io/endpoint"]
		if !desired[eid] {
			_ = r.Delete(ctx, dep)
		}
	}

	for i := range listofservices.Items {
		dep := &listofservices.Items[i]
		eid := dep.Labels["endpointscaler.io/endpoint"]
		if !desired[eid] {
			_ = r.Delete(ctx, dep)
		}
	}

	for i := range listofroutes.Items {
		dep := &listofroutes.Items[i]
		eid := dep.Labels["endpointscaler.io/endpoint"]
		if !desired[eid] {
			_ = r.Delete(ctx, dep)
		}
	}

	for i := range listofgrpcroutes.Items {
		dep := &listofgrpcroutes.Items[i]
		eid := dep.Labels["endpointscaler.io/endpoint"]
		if !desired[eid] {
			_ = r.Delete(ctx, dep)
		}
	}

	for i := range listofhpas.Items {
		dep := &listofhpas.Items[i]
		eid := dep.Labels["endpointscaler.io/endpoint"]
		if !desired[eid] {
			_ = r.Delete(ctx, dep)
		}
	}

	policy.Status.EndpointCount = len(policy.Spec.Endpoints)
	policy.Status.EndpointStatuses = endpointStatuses

	readyCount := 0
	for _, s := range endpointStatuses {
		if s.Ready {
			readyCount++
		}
	}
	RecordEndpointMetrics(policy.Namespace, policy.Name, len(policy.Spec.Endpoints), readyCount)

	condition := metav1.Condition{
		Type:               "Ready",
		ObservedGeneration: policy.Generation,
		LastTransitionTime: metav1.Now(),
	}

	if readyCount == len(policy.Spec.Endpoints) {
		condition.Status = metav1.ConditionTrue
		condition.Reason = "AllEndpointsReady"
		condition.Message = fmt.Sprintf("All %d endpoints ready", readyCount)
	} else {
		condition.Status = metav1.ConditionFalse
		condition.Reason = "EndpointsNotReady"
		condition.Message = fmt.Sprintf("%d/%d endpoints ready", readyCount, len(policy.Spec.Endpoints))
	}

	meta.SetStatusCondition(&policy.Status.Conditions, condition)

	if err := r.Status().Update(ctx, policy); err != nil {
		logger.Error(err, "failed to update status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *EndpointPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&esv1alpha1.EndpointPolicy{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&autoscalingv2.HorizontalPodAutoscaler{}).
		Owns(&gatewayv1.HTTPRoute{}).
		Owns(&gatewayv1.GRPCRoute{}).
		Complete(r)
}

func endpointResourceName(policy *esv1alpha1.EndpointPolicy, endpoint *esv1alpha1.EndpointSpec) string {
	return fmt.Sprintf("%s-%s", policy.Spec.AppRef.Name, endpoint.ID)
}

func endpointServiceName(policy *esv1alpha1.EndpointPolicy, endpoint *esv1alpha1.EndpointSpec) string {
	return fmt.Sprintf("%s-%s-svc", policy.Spec.AppRef.Name, endpoint.ID)
}

func mainServiceName(policy *esv1alpha1.EndpointPolicy) string {
	return fmt.Sprintf("%s-svc", policy.Spec.AppRef.Name)
}

func generateLabels(policy *esv1alpha1.EndpointPolicy, endpoint *esv1alpha1.EndpointSpec) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       policy.Spec.AppRef.Name,
		"app.kubernetes.io/component":  endpoint.ID,
		"app.kubernetes.io/managed-by": "endpoint-scaler",
		"endpointscaler.io/policy":     policy.Name,
		"endpointscaler.io/endpoint":   endpoint.ID,
	}
}
