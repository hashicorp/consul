// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

import "testing"

func TestValidateGatewayConditionReasonWithValidCombinations(t *testing.T) {
	testCases := map[string]struct {
		status   ConditionStatus
		reason   GatewayConditionReason
		condType GatewayConditionType
	}{
		"accepted": {
			status:   ConditionStatusTrue,
			reason:   GatewayReasonAccepted,
			condType: GatewayConditionAccepted,
		},
		"accepted invalid certificates": {
			status:   ConditionStatusFalse,
			reason:   GatewayReasonInvalidCertificates,
			condType: GatewayConditionAccepted,
		},
		"conflicted": {
			status:   ConditionStatusTrue,
			reason:   GatewayReasonRouteConflict,
			condType: GatewayConditionConflicted,
		},
		"conflicted no conflicts": {
			status:   ConditionStatusFalse,
			reason:   GatewayReasonNoConflict,
			condType: GatewayConditionConflicted,
		},

		"resolved refs": {
			status:   ConditionStatusTrue,
			reason:   GatewayReasonResolvedRefs,
			condType: GatewayConditionResolvedRefs,
		},
		"resolved refs invalid certificate ref": {
			status:   ConditionStatusFalse,
			reason:   GatewayListenerReasonInvalidCertificateRef,
			condType: GatewayConditionResolvedRefs,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := ValidateGatewayConditionReason(tc.condType, tc.status, tc.reason)
			if err != nil {
				t.Error("Expected gateway condition reason to be valid but it was not")
			}
		})
	}
}

func TestValidateGatewayConditionReasonWithInvalidCombinationsReturnsError(t *testing.T) {
	// This is not an exhaustive list of all invalid combinations, just a few to confirm
	testCases := map[string]struct {
		status   ConditionStatus
		reason   GatewayConditionReason
		condType GatewayConditionType
	}{
		"reason and condition type are valid but status is not": {
			status:   ConditionStatusTrue,
			reason:   GatewayReasonNoConflict,
			condType: GatewayConditionConflicted,
		},
		"reason and status are valid but condition type is not": {
			status:   ConditionStatusFalse,
			reason:   GatewayReasonNoConflict,
			condType: GatewayConditionResolvedRefs,
		},
		"condition type and status are valid but status is not": {
			status:   ConditionStatusTrue,
			reason:   GatewayReasonNoConflict,
			condType: GatewayConditionAccepted,
		},
		"all are invalid": {
			status:   ConditionStatusUnknown,
			reason:   GatewayReasonAccepted,
			condType: GatewayConditionResolvedRefs,
		},
		"pass something other than a condition status": {
			status:   ConditionStatus("hello"),
			reason:   GatewayReasonAccepted,
			condType: GatewayConditionResolvedRefs,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := ValidateGatewayConditionReason(tc.condType, tc.status, tc.reason)
			if err == nil {
				t.Error("Expected route condition reason to be invalid, but it was valid")
			}
		})
	}
}

func TestValidateRouteConfigReasonWithValidCombinations(t *testing.T) {
	testCases := map[string]struct {
		status   ConditionStatus
		reason   RouteConditionReason
		condType RouteConditionType
	}{
		"accepted all around": {
			status:   ConditionStatusTrue,
			reason:   RouteReasonAccepted,
			condType: RouteConditionAccepted,
		},
		"accepted invalid discovery chain": {
			status:   ConditionStatusFalse,
			reason:   RouteReasonInvalidDiscoveryChain,
			condType: RouteConditionAccepted,
		},
		"accepted no upstream services targeted": {
			status:   ConditionStatusFalse,
			reason:   RouteReasonNoUpstreamServicesTargeted,
			condType: RouteConditionAccepted,
		},
		"route bound": {
			status:   ConditionStatusTrue,
			reason:   RouteReasonBound,
			condType: RouteConditionBound,
		},
		"route bound gateway not found": {
			status:   ConditionStatusFalse,
			reason:   RouteReasonGatewayNotFound,
			condType: RouteConditionBound,
		},
		"route bound failed to bind": {
			status:   ConditionStatusFalse,
			reason:   RouteReasonFailedToBind,
			condType: RouteConditionBound,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := ValidateRouteConditionReason(tc.condType, tc.status, tc.reason)
			if err != nil {
				t.Errorf("Expected route condition reason to be valid, it was not")
			}
		})
	}
}

func TestValidateRouteConditionReasonInvalidCombinationsCausePanic(t *testing.T) {
	// This is not an exhaustive list of all invalid combinations, just a few to confirm
	testCases := map[string]struct {
		status   ConditionStatus
		reason   RouteConditionReason
		condType RouteConditionType
	}{
		"reason and condition type are valid but status is not": {
			status:   ConditionStatusTrue,
			reason:   RouteReasonNoUpstreamServicesTargeted,
			condType: RouteConditionAccepted,
		},
		"reason and status are valid but condition type is not": {
			status:   ConditionStatusFalse,
			reason:   RouteReasonInvalidDiscoveryChain,
			condType: RouteConditionBound,
		},
		"condition type and status are valid but status is not": {
			status:   ConditionStatusUnknown,
			reason:   RouteReasonBound,
			condType: RouteConditionBound,
		},
		"all are invalid": {
			status:   ConditionStatusUnknown,
			reason:   RouteReasonGatewayNotFound,
			condType: RouteConditionBound,
		},
		"pass something other than a condition status": {
			status:   ConditionStatus("hello"),
			reason:   RouteReasonAccepted,
			condType: RouteConditionAccepted,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := ValidateRouteConditionReason(tc.condType, tc.status, tc.reason)
			if err == nil {
				t.Error("Expected route condition reason to be invalid, it was valid")
			}
		})
	}
}
