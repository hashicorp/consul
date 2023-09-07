// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package meshv1alpha1

// GetUnderlyingBackendRefs will collect BackendReferences from all rules and
// bundle them up in one slice, unwrapping the HTTP-specifics in the process.
//
// This implements an XRouteWithRefs interface in the internal/mesh package.
//
// NOTE: no deduplication occurs.
func (x *HTTPRoute) GetUnderlyingBackendRefs() []*BackendReference {
	if x == nil {
		return nil
	}

	estimate := 0
	for _, rule := range x.Rules {
		estimate += len(rule.BackendRefs)
	}

	backendRefs := make([]*BackendReference, 0, estimate)
	for _, rule := range x.Rules {
		for _, backendRef := range rule.BackendRefs {
			backendRefs = append(backendRefs, backendRef.BackendRef)
		}
	}
	return backendRefs
}

// GetUnderlyingBackendRefs will collect BackendReferences from all rules and
// bundle them up in one slice, unwrapping the GRPC-specifics in the process.
//
// This implements an XRouteWithRefs interface in the internal/mesh package.
//
// NOTE: no deduplication occurs.
func (x *GRPCRoute) GetUnderlyingBackendRefs() []*BackendReference {
	if x == nil {
		return nil
	}

	estimate := 0
	for _, rule := range x.Rules {
		estimate += len(rule.BackendRefs)
	}

	backendRefs := make([]*BackendReference, 0, estimate)
	for _, rule := range x.Rules {
		for _, backendRef := range rule.BackendRefs {
			backendRefs = append(backendRefs, backendRef.BackendRef)
		}
	}
	return backendRefs
}

// GetUnderlyingBackendRefs will collect BackendReferences from all rules and
// bundle them up in one slice, unwrapping the TCP-specifics in the process.
//
// This implements an XRouteWithRefs interface in the internal/mesh package.
//
// NOTE: no deduplication occurs.
func (x *TCPRoute) GetUnderlyingBackendRefs() []*BackendReference {
	if x == nil {
		return nil
	}

	estimate := 0
	for _, rule := range x.Rules {
		estimate += len(rule.BackendRefs)
	}

	backendRefs := make([]*BackendReference, 0, estimate)

	for _, rule := range x.Rules {
		for _, backendRef := range rule.BackendRefs {
			backendRefs = append(backendRefs, backendRef.BackendRef)
		}
	}
	return backendRefs
}

// IsHashBased returns true if the policy is a hash-based policy such as maglev
// or ring hash.
func (p LoadBalancerPolicy) IsHashBased() bool {
	switch p {
	case LoadBalancerPolicy_LOAD_BALANCER_POLICY_MAGLEV,
		LoadBalancerPolicy_LOAD_BALANCER_POLICY_RING_HASH:
		return true
	}
	return false
}
