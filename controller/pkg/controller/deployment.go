package controller

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	esv1alpha1 "github.com/example/endpoint-scaler/controller/pkg/apis/endpointscaler/v1alpha1"
)

func (r *EndpointPolicyReconciler) reconcileDeployment(
	ctx context.Context,
	policy *esv1alpha1.EndpointPolicy,
	endpoint *esv1alpha1.EndpointSpec,
) (string, error) {
	logger := log.FromContext(ctx)
	name := endpointResourceName(policy, endpoint)

	desired, err := r.buildDeployment(policy, endpoint)
	if err != nil {
		return "", err
	}
	if err := ctrl.SetControllerReference(policy, desired, r.Scheme); err != nil {
		return "", err
	}

	existing := &appsv1.Deployment{}
	err = r.Get(ctx, types.NamespacedName{Name: name, Namespace: policy.Namespace}, existing)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			return "", err
		}
		logger.Info("Creating Deployment", "name", name)
		return name, r.Create(ctx, desired)
	}

	existing.Spec = desired.Spec
	existing.Labels = desired.Labels
	logger.Info("Updating Deployment", "name", name)
	return name, r.Update(ctx, existing)
}

func (r *EndpointPolicyReconciler) buildDeployment(
	policy *esv1alpha1.EndpointPolicy,
	endpoint *esv1alpha1.EndpointSpec,
) (*appsv1.Deployment, error) {
	name := endpointResourceName(policy, endpoint)
	labels := generateLabels(policy, endpoint)

	var replicas int32 = 1
	if endpoint.Replicas != nil {
		replicas = *endpoint.Replicas
	}
	if endpoint.HPA != nil {
		replicas = endpoint.HPA.Min
	}

	containerPort := policy.Spec.AppRef.ContainerPort
	if containerPort == 0 {
		containerPort = 8080
	}

	image := policy.Spec.AppRef.Image
	if image == "" {
		return nil, fmt.Errorf("appRef.image is required")
	}

	container := corev1.Container{
		Name:  endpoint.ID,
		Image: image,
		Ports: []corev1.ContainerPort{{
			Name:          "http",
			ContainerPort: containerPort,
			Protocol:      corev1.ProtocolTCP,
		}},
		Env: []corev1.EnvVar{{
			Name:  "MICROAPI_GUARDRAIL",
			Value: endpoint.ID,
		}},
	}

	if endpoint.Resources != nil {
		container.Resources = buildResourceRequirements(endpoint.Resources)
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: policy.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{container}},
			},
		},
	}, nil
}

func buildResourceRequirements(res *esv1alpha1.ResourceSpec) corev1.ResourceRequirements {
	reqs := corev1.ResourceRequirements{
		Limits:   corev1.ResourceList{},
		Requests: corev1.ResourceList{},
	}

	if res.CPULimit != "" {
		if q, err := resource.ParseQuantity(res.CPULimit); err == nil {
			reqs.Limits[corev1.ResourceCPU] = q
			if res.CPURequest == "" {
				reqs.Requests[corev1.ResourceCPU] = q
			}
		}
	}

	if res.CPURequest != "" {
		if q, err := resource.ParseQuantity(res.CPURequest); err == nil {
			reqs.Requests[corev1.ResourceCPU] = q
		}
	}

	if res.MemLimit != "" {
		if q, err := resource.ParseQuantity(res.MemLimit); err == nil {
			reqs.Limits[corev1.ResourceMemory] = q
			if res.MemRequest == "" {
				reqs.Requests[corev1.ResourceMemory] = q
			}
		}
	}

	if res.MemRequest != "" {
		if q, err := resource.ParseQuantity(res.MemRequest); err == nil {
			reqs.Requests[corev1.ResourceMemory] = q
		}
	}

	return reqs
}
