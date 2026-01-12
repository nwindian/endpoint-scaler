// Package endpointscaler provides middleware for endpoint-scaler controlled routing.
package endpointscaler

import (
	"net/http"
	"os"
)

const (
	// GuardrailEnvVar is the environment variable name that controls which endpoint
	// handler should be active. When set, only handlers with matching endpoint IDs
	// will process requests.
	GuardrailEnvVar = "ENDPOINTSCALER_GUARDRAIL"
)

// Guard wraps an HTTP handler and only executes it if the ENDPOINTSCALER_GUARDRAIL
// environment variable matches the specified endpoint ID.
//
// When running in an endpoint-scaler managed deployment, each endpoint container
// has ENDPOINTSCALER_GUARDRAIL set to its endpoint ID. This ensures that only the
// appropriate handler processes requests for that endpoint.
//
// Usage:
//
//	mux := http.NewServeMux()
//	mux.Handle("/lookup", endpointscaler.Guard("lookup", lookupHandler))
//	mux.Handle("/search", endpointscaler.Guard("search", searchHandler))
//
// When ENDPOINTSCALER_GUARDRAIL is not set (e.g., in development), all handlers are active.
// When ENDPOINTSCALER_GUARDRAIL is set to "lookup", only the lookup handler processes requests.
func Guard(endpointID string, handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		guardrail := os.Getenv(GuardrailEnvVar)

		// If no guardrail is set, allow all handlers (development mode)
		if guardrail == "" {
			handler.ServeHTTP(w, r)
			return
		}

		// Only execute if this handler's endpoint ID matches the guardrail
		if guardrail == endpointID {
			handler.ServeHTTP(w, r)
			return
		}

		// This handler is not active for the current guardrail
		// Return 503 to indicate the service is not available at this endpoint
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("endpoint not active"))
	})
}

// GuardFunc is a convenience wrapper for Guard that accepts an http.HandlerFunc.
func GuardFunc(endpointID string, handler http.HandlerFunc) http.Handler {
	return Guard(endpointID, handler)
}

// IsActiveEndpoint returns true if the current process should handle requests
// for the specified endpoint ID.
//
// Usage:
//
//	if endpointscaler.IsActiveEndpoint("lookup") {
//	    // Initialize lookup-specific resources
//	}
func IsActiveEndpoint(endpointID string) bool {
	guardrail := os.Getenv(GuardrailEnvVar)
	return guardrail == "" || guardrail == endpointID
}

// ActiveEndpoint returns the currently active endpoint ID from the environment.
// Returns an empty string if no guardrail is set (development mode).
func ActiveEndpoint() string {
	return os.Getenv(GuardrailEnvVar)
}

// GRPCUnaryInterceptor returns a gRPC unary interceptor that only processes
// requests when the ENDPOINTSCALER_GUARDRAIL matches the endpoint ID.
//
// Usage with gRPC:
//
//	server := grpc.NewServer(
//	    grpc.UnaryInterceptor(endpointscaler.GRPCUnaryInterceptor("lookup")),
//	)
//
// Note: This requires importing google.golang.org/grpc. The interceptor
// signature is provided here as a reference implementation pattern.
type UnaryServerInterceptor func(
	ctx interface{},
	req interface{},
	info interface{},
	handler interface{},
) (interface{}, error)
