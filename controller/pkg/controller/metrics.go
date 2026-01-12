package controller

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	policiesTotal = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "endpointscaler_policies_total",
		Help: "Total number of EndpointPolicy resources",
	})

	endpointsTotal = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "endpointscaler_endpoints_total",
		Help: "Total number of endpoints per policy",
	}, []string{"namespace", "policy"})

	endpointsReady = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "endpointscaler_endpoints_ready",
		Help: "Number of ready endpoints per policy",
	}, []string{"namespace", "policy"})

	endpointInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "endpointscaler_endpoint_info",
		Help: "Information about individual endpoints",
	}, []string{"namespace", "policy", "endpoint", "type", "strategy"})

	reconcileErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "endpointscaler_reconcile_errors_total",
		Help: "Total number of reconciliation errors by type",
	}, []string{"namespace", "policy", "error_type"})
)

func init() {
	metrics.Registry.MustRegister(
		policiesTotal,
		endpointsTotal,
		endpointsReady,
		endpointInfo,
		reconcileErrors,
	)
}

func RecordPolicyCount(count int) {
	policiesTotal.Set(float64(count))
}

func RecordEndpointMetrics(namespace, policy string, total, ready int) {
	endpointsTotal.WithLabelValues(namespace, policy).Set(float64(total))
	endpointsReady.WithLabelValues(namespace, policy).Set(float64(ready))
}

func RecordEndpointInfo(namespace, policy, endpoint, epType, strategy string) {
	endpointInfo.WithLabelValues(namespace, policy, endpoint, epType, strategy).Set(1)
}

func RemoveEndpointInfo(namespace, policy, endpoint, epType, strategy string) {
	endpointInfo.DeleteLabelValues(namespace, policy, endpoint, epType, strategy)
}

func RecordReconcileError(namespace, policy, errorType string) {
	reconcileErrors.WithLabelValues(namespace, policy, errorType).Inc()
}

func RemovePolicyMetrics(namespace, policy string) {
	endpointsTotal.DeleteLabelValues(namespace, policy)
	endpointsReady.DeleteLabelValues(namespace, policy)
}
