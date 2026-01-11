package controller

import (
	"context"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	esv1alpha1 "github.com/example/endpoint-scaler/controller/pkg/apis/endpointscaler/v1alpha1"
)

func (r *EndpointPolicyReconciler) reconcileHPA(
	ctx context.Context,
	policy *esv1alpha1.EndpointPolicy,
	endpoint *esv1alpha1.EndpointSpec,
) error {
	if endpoint.HPA == nil {
		return nil
	}

	logger := log.FromContext(ctx)
	name := endpointResourceName(policy, endpoint)

	desired := r.buildHPA(policy, endpoint)
	if err := ctrl.SetControllerReference(policy, desired, r.Scheme); err != nil {
		return err
	}

	existing := &autoscalingv2.HorizontalPodAutoscaler{}
	err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: policy.Namespace}, existing)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			return err
		}
		logger.Info("Creating HPA", "name", name)
		return r.Create(ctx, desired)
	}

	existing.Spec = desired.Spec
	existing.Labels = desired.Labels
	logger.Info("Updating HPA", "name", name)
	return r.Update(ctx, existing)
}

func (r *EndpointPolicyReconciler) buildHPA(
	policy *esv1alpha1.EndpointPolicy,
	endpoint *esv1alpha1.EndpointSpec,
) *autoscalingv2.HorizontalPodAutoscaler {
	name := endpointResourceName(policy, endpoint)
	labels := generateLabels(policy, endpoint)

	hpa := &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: policy.Namespace,
			Labels:    labels,
		},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       name,
			},
			MinReplicas: &endpoint.HPA.Min,
			MaxReplicas: endpoint.HPA.Max,
			Metrics:     []autoscalingv2.MetricSpec{},
		},
	}

	if endpoint.HPA.CPUTarget != nil && *endpoint.HPA.CPUTarget > 0 {
		hpa.Spec.Metrics = append(hpa.Spec.Metrics, autoscalingv2.MetricSpec{
			Type: autoscalingv2.ResourceMetricSourceType,
			Resource: &autoscalingv2.ResourceMetricSource{
				Name: corev1.ResourceCPU,
				Target: autoscalingv2.MetricTarget{
					Type:               autoscalingv2.UtilizationMetricType,
					AverageUtilization: endpoint.HPA.CPUTarget,
				},
			},
		})
	}

	if endpoint.HPA.MemoryTarget != nil && *endpoint.HPA.MemoryTarget > 0 {
		hpa.Spec.Metrics = append(hpa.Spec.Metrics, autoscalingv2.MetricSpec{
			Type: autoscalingv2.ResourceMetricSourceType,
			Resource: &autoscalingv2.ResourceMetricSource{
				Name: corev1.ResourceMemory,
				Target: autoscalingv2.MetricTarget{
					Type:               autoscalingv2.UtilizationMetricType,
					AverageUtilization: endpoint.HPA.MemoryTarget,
				},
			},
		})
	}

	return hpa
}
