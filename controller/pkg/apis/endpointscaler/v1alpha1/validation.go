package v1alpha1

import (
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// Validate validates the EndpointPolicySpec and returns nil if valid,
// or an aggregate error containing all validation failures.
func (s *EndpointPolicySpec) Validate() error {
	allErrs := s.validate(field.NewPath("spec"))
	return allErrs.ToAggregate()
}

func (s *EndpointPolicySpec) validate(fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, s.AppRef.validate(fldPath.Child("appRef"))...)
	allErrs = append(allErrs, s.GatewayRef.validate(fldPath.Child("gatewayRef"))...)
	allErrs = append(allErrs, validateEndpoints(s.Endpoints, fldPath.Child("endpoints"))...)

	return allErrs
}

func (a *AppReference) validate(fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if a.Name == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "application name is required"))
	}
	if a.Image == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("image"), "container image is required"))
	}
	if a.Port < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("port"), a.Port, "must be a positive integer"))
	}
	if a.ContainerPort < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("containerPort"), a.ContainerPort, "must be a positive integer"))
	}

	return allErrs
}

func (g *GatewayReference) validate(fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if g.Name == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "gateway name is required"))
	}

	return allErrs
}

func validateEndpoints(endpoints []EndpointSpec, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if len(endpoints) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, "at least one endpoint is required"))
		return allErrs
	}

	seen := make(map[string]bool)
	for i, ep := range endpoints {
		if seen[ep.ID] {
			allErrs = append(allErrs, field.Duplicate(fldPath.Index(i).Child("id"), ep.ID))
		}
		seen[ep.ID] = true
	}

	for i := range endpoints {
		allErrs = append(allErrs, endpoints[i].validate(fldPath.Index(i))...)
	}

	return allErrs
}

func (e *EndpointSpec) validate(fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if e.ID == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("id"), "endpoint id is required"))
	}

	allErrs = append(allErrs, e.validateMatch(fldPath.Child("match"))...)

	if e.Replicas != nil && *e.Replicas < 1 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("replicas"), *e.Replicas, "must be at least 1"))
	}

	if e.Resources != nil {
		allErrs = append(allErrs, e.Resources.validate(fldPath.Child("resources"))...)
	}

	if e.HPA != nil {
		allErrs = append(allErrs, e.HPA.validate(fldPath.Child("hpa"))...)
	}

	return allErrs
}

func (e *EndpointSpec) validateMatch(fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	epType := e.Type
	if epType == "" {
		epType = "http"
	}

	switch epType {
	case "http":
		if e.Match.Path == "" {
			allErrs = append(allErrs, field.Required(fldPath.Child("path"), "path is required for HTTP endpoints"))
		}
	case "grpc":
		if e.Match.Service == "" {
			allErrs = append(allErrs, field.Required(fldPath.Child("service"), "service is required for gRPC endpoints"))
		}
		if e.Match.Method == "" {
			allErrs = append(allErrs, field.Required(fldPath.Child("method"), "method is required for gRPC endpoints"))
		}
	}

	return allErrs
}

func (r *ResourceSpec) validate(fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, validateResourceQuantity(r.CPULimit, fldPath.Child("cpuLimit"))...)
	allErrs = append(allErrs, validateResourceQuantity(r.CPURequest, fldPath.Child("cpuRequest"))...)
	allErrs = append(allErrs, validateResourceQuantity(r.MemLimit, fldPath.Child("memLimit"))...)
	allErrs = append(allErrs, validateResourceQuantity(r.MemRequest, fldPath.Child("memRequest"))...)

	return allErrs
}

func validateResourceQuantity(value string, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if value == "" {
		return allErrs
	}

	if _, err := resource.ParseQuantity(value); err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, value, "invalid resource quantity: "+err.Error()))
	}

	return allErrs
}

func (h *HPASpec) validate(fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if h.Min < 1 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("min"), h.Min, "must be at least 1"))
	}

	if h.Max < 1 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("max"), h.Max, "must be at least 1"))
	}

	if h.Max < h.Min {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("max"), h.Max, "must be greater than or equal to min"))
	}

	if h.CPUTarget == nil && h.MemoryTarget == nil {
		allErrs = append(allErrs, field.Required(fldPath, "at least one of cpuTarget or memoryTarget is required"))
	}

	return allErrs
}
