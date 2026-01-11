package controller

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	esv1alpha1 "github.com/example/endpoint-scaler/controller/pkg/apis/endpointscaler/v1alpha1"
)

func (r *EndpointPolicyReconciler) reconcileService(
	ctx context.Context,
	policy *esv1alpha1.EndpointPolicy,
	endpoint *esv1alpha1.EndpointSpec,
) (string, error) {
	logger := log.FromContext(ctx)
	name := endpointServiceName(policy, endpoint)

	desired := r.buildService(policy, endpoint)
	if err := ctrl.SetControllerReference(policy, desired, r.Scheme); err != nil {
		return "", err
	}

	existing := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: policy.Namespace}, existing)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			return "", err
		}
		logger.Info("Creating Service", "name", name)
		return name, r.Create(ctx, desired)
	}

	existing.Spec.Ports = desired.Spec.Ports
	existing.Spec.Selector = desired.Spec.Selector
	existing.Labels = desired.Labels
	logger.Info("Updating Service", "name", name)
	return name, r.Update(ctx, existing)
}

func (r *EndpointPolicyReconciler) buildService(
	policy *esv1alpha1.EndpointPolicy,
	endpoint *esv1alpha1.EndpointSpec,
) *corev1.Service {
	name := endpointServiceName(policy, endpoint)
	labels := generateLabels(policy, endpoint)

	servicePort := policy.Spec.AppRef.Port
	if servicePort == 0 {
		servicePort = 80
	}

	containerPort := policy.Spec.AppRef.ContainerPort
	if containerPort == 0 {
		containerPort = 8080
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: policy.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: labels,
			Ports: []corev1.ServicePort{{
				Name:       "http",
				Port:       servicePort,
				TargetPort: intstr.FromInt32(containerPort),
				Protocol:   corev1.ProtocolTCP,
			}},
		},
	}
}
