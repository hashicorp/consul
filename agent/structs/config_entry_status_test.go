package structs

import "testing"

func TestNewGatewayConditionWithValidCombinations(t *testing.T) {
	testCases := map[string]struct {
		status   ConditionStatus
		reason   GatewayConditionReason
		condType GatewayConditionType
		message  string
		ref      ResourceReference
	}{
		"accepted all around": {
			status:   ConditionStatusTrue,
			reason:   GatewayReasonAccepted,
			condType: GatewayConditionAccepted,
			message:  "it's all good",
			ref:      ResourceReference{Name: "name", Kind: "httproute"},
		},
		"accepted false gateway reason invalid": {
			status:   ConditionStatusFalse,
			reason:   GatewayReasonInvalid,
			condType: GatewayConditionAccepted,
			message:  "no bueno",
			ref:      ResourceReference{Name: "name", Kind: "httproute"},
		},
		"accepted false gateway reason pending": {
			status:   ConditionStatusFalse,
			reason:   GatewayReasonPending,
			condType: GatewayConditionAccepted,
			message:  "pending",
			ref:      ResourceReference{Name: "name", Kind: "httproute"},
		},
		"listener configured true": {
			status:   ConditionStatusTrue,
			reason:   GatewayReasonListenersConfigured,
			condType: GatewayConditionListenersConfigured,
			message:  "listeners configured",
			ref:      ResourceReference{Name: "name", Kind: "httproute"},
		},
		"listener configured hostname conflict": {
			status:   ConditionStatusFalse,
			reason:   GatewayListenerReasonHostnameConflict,
			condType: GatewayConditionListenersConfigured,
			message:  "conflict",
			ref:      ResourceReference{Name: "name", Kind: "httproute"},
		},
		"listener configured port unvailable": {
			status:   ConditionStatusFalse,
			reason:   GatewayListenerReasonPortUnavailable,
			condType: GatewayConditionListenersConfigured,
			message:  "no port",
			ref:      ResourceReference{Name: "name", Kind: "httproute"},
		},

		"listener configured unsupported protocol": {
			status:   ConditionStatusFalse,
			reason:   GatewayListenerReasonUnsupportedProtocol,
			condType: GatewayConditionListenersConfigured,
			message:  "unsupported procotol",
			ref:      ResourceReference{Name: "name", Kind: "httproute"},
		},
		"listener configured unsupported address": {
			status:   ConditionStatusFalse,
			reason:   GatewayListenerReasonUnsupportedAddress,
			condType: GatewayConditionListenersConfigured,
			message:  "unsupported address",
			ref:      ResourceReference{Name: "name", Kind: "httproute"},
		},
		"listener configured status unknown": {
			status:   ConditionStatusUnknown,
			reason:   GatewayReasonPending,
			condType: GatewayConditionListenersConfigured,
			message:  "unknown",
			ref:      ResourceReference{Name: "name", Kind: "httproute"},
		},
		"resolved refs": {
			status:   ConditionStatusTrue,
			reason:   GatewayReasonResolvedRefs,
			condType: GatewayConditionResolvedRefs,
			message:  "resolved refs",
			ref:      ResourceReference{Name: "name", Kind: "httproute"},
		},
		"resolved refs invalid certificate ref": {
			status:   ConditionStatusFalse,
			reason:   GatewayListenerReasonInvalidCertificateRef,
			condType: GatewayConditionResolvedRefs,
			message:  "invalid certificate",
			ref:      ResourceReference{Name: "name", Kind: "httproute"},
		},
		"resolved refs invalid route kinds": {
			status:   ConditionStatusFalse,
			reason:   GatewayListenerReasonInvalidRouteKinds,
			condType: GatewayConditionResolvedRefs,
			message:  "invalid route kinds",
			ref:      ResourceReference{Name: "name", Kind: "httproute"},
		},
		"resolved refs ref not permitted": {
			status:   ConditionStatusFalse,
			reason:   GatewayListenerReasonRefNotPermitted,
			condType: GatewayConditionResolvedRefs,
			message:  "not permitted",
			ref:      ResourceReference{Name: "name", Kind: "httproute"},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			cond := NewGatewayCondition(tc.condType, tc.status, tc.reason, tc.message, tc.ref)
			expectedCond := Condition{
				Type:     string(tc.condType),
				Status:   tc.status,
				Reason:   string(tc.reason),
				Message:  tc.message,
				Resource: &tc.ref,
			}
			if !cond.IsSame(&expectedCond) {
				t.Errorf("Expected condition to be\n%+v\ngot\n%+v", expectedCond, cond)
			}
		})
	}
}

func TestNewGatewayInvalidCombinationsCausePanic(t *testing.T) {
	// This is not an exhaustive list of all invalid combinations, just a few to confirm
	testCases := map[string]struct {
		status   ConditionStatus
		reason   GatewayConditionReason
		condType GatewayConditionType
		message  string
		ref      ResourceReference
	}{
		"reason and condition type are valid but status is not": {
			status:   ConditionStatusTrue,
			reason:   GatewayReasonInvalid,
			condType: GatewayConditionAccepted,
			message:  "almost there",
			ref:      ResourceReference{Name: "name", Kind: "httproute"},
		},
		"reason and status are valid but condition type is not": {
			status:   ConditionStatusFalse,
			reason:   GatewayListenerReasonHostnameConflict,
			condType: GatewayConditionResolvedRefs,
			message:  "not quite",
			ref:      ResourceReference{Name: "name", Kind: "httproute"},
		},
		"condition type and status are valid but status is not": {
			status:   ConditionStatusUnknown,
			reason:   GatewayReasonInvalid,
			condType: GatewayConditionAccepted,
			message:  "still not working",
			ref:      ResourceReference{Name: "name", Kind: "httproute"},
		},
		"all are invalid": {
			status:   ConditionStatusUnknown,
			reason:   GatewayReasonInvalid,
			condType: GatewayConditionResolvedRefs,
			message:  "still not working",
			ref:      ResourceReference{Name: "name", Kind: "httproute"},
		},
		"pass something other than a condition status": {
			status:   ConditionStatus("hello"),
			reason:   GatewayReasonInvalid,
			condType: GatewayConditionResolvedRefs,
			message:  "still not working",
			ref:      ResourceReference{Name: "name", Kind: "httproute"},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("Expected combination %+v to be invalid", tc)
				}
			}()
			_ = NewGatewayCondition(tc.condType, tc.status, tc.reason, tc.message, tc.ref)
		})
	}
}

func TestNewRouteConditionWithValidCombinations(t *testing.T) {
	testCases := map[string]struct {
		status   ConditionStatus
		reason   RouteConditionReason
		condType RouteConditionType
		message  string
		ref      ResourceReference
	}{
		"accepted all around": {
			status:   ConditionStatusTrue,
			reason:   RouteReasonAccepted,
			condType: RouteConditionAccepted,
			message:  "it's all good",
			ref:      ResourceReference{Name: "name", Kind: "httproute"},
		},
		"accepted not allowed by listeners": {
			status:   ConditionStatusFalse,
			reason:   RouteReasonNotAllowedByListeners,
			condType: RouteConditionAccepted,
			message:  "not allowed by listeners",
			ref:      ResourceReference{Name: "name", Kind: "httproute"},
		},
		"accepted no matching hostname": {
			status:   ConditionStatusFalse,
			reason:   RouteReasonNoMatchingListenerHostname,
			condType: RouteConditionAccepted,
			message:  "no matching listener hostname",
			ref:      ResourceReference{Name: "name", Kind: "httproute"},
		},
		"accepted no matching parent": {
			status:   ConditionStatusFalse,
			reason:   RouteReasonNoMatchingParent,
			condType: RouteConditionAccepted,
			message:  "no matching parent",
			ref:      ResourceReference{Name: "name", Kind: "httproute"},
		},
		"accepted unsupported value": {
			status:   ConditionStatusFalse,
			reason:   RouteReasonUnsupportedValue,
			condType: RouteConditionAccepted,
			message:  "unsupported value",
			ref:      ResourceReference{Name: "name", Kind: "httproute"},
		},
		"accepted parent ref not permitted": {
			status:   ConditionStatusFalse,
			reason:   RouteReasonParentRefNotPermitted,
			condType: RouteConditionAccepted,
			message:  "parent ref not permitted",
			ref:      ResourceReference{Name: "name", Kind: "httproute"},
		},
		"accepted invalid discovery chain": {
			status:   ConditionStatusFalse,
			reason:   RouteReasonInvalidDiscoveryChain,
			condType: RouteConditionAccepted,
			message:  "invalid discovery chain",
			ref:      ResourceReference{Name: "name", Kind: "httproute"},
		},
		"accepted no upstream services targeted": {
			status:   ConditionStatusFalse,
			reason:   RouteReasonNoUpstreamServicesTargeted,
			condType: RouteConditionAccepted,
			message:  "no upstreams",
			ref:      ResourceReference{Name: "name", Kind: "httproute"},
		},
		"accepted pending": {
			status:   ConditionStatusUnknown,
			reason:   RouteReasonPending,
			condType: RouteConditionAccepted,
			message:  "pending",
			ref:      ResourceReference{Name: "name", Kind: "httproute"},
		},
		"resolved refs": {
			status:   ConditionStatusTrue,
			reason:   RouteReasonResolvedRefs,
			condType: RouteConditionResolvedRefs,
			message:  "resolved refs",
			ref:      ResourceReference{Name: "name", Kind: "httproute"},
		},
		"resolved refs not permitted": {
			status:   ConditionStatusFalse,
			reason:   RouteReasonRefNotPermitted,
			condType: RouteConditionResolvedRefs,
			message:  "not permitted",
			ref:      ResourceReference{Name: "name", Kind: "httproute"},
		},
		"resolved refs invalid kind": {
			status:   ConditionStatusFalse,
			reason:   RouteReasonInvalidKind,
			condType: RouteConditionResolvedRefs,
			message:  "invalid kind",
			ref:      ResourceReference{Name: "name", Kind: "httproute"},
		},
		"resolved refs bakend not found": {
			status:   ConditionStatusFalse,
			reason:   RouteReasonBackendNotFound,
			condType: RouteConditionResolvedRefs,
			message:  "backend not found",
			ref:      ResourceReference{Name: "name", Kind: "httproute"},
		},
		"route bound": {
			status:   ConditionStatusTrue,
			reason:   RouteReasonBound,
			condType: RouteConditionBound,
			message:  "bound",
			ref:      ResourceReference{Name: "name", Kind: "httproute"},
		},
		"route bound gateway not found": {
			status:   ConditionStatusFalse,
			reason:   RouteReasonGatewayNotFound,
			condType: RouteConditionBound,
			message:  "gateway not found",
			ref:      ResourceReference{Name: "name", Kind: "httproute"},
		},
		"route bound failed to bind": {
			status:   ConditionStatusFalse,
			reason:   RouteReasonFailedToBind,
			condType: RouteConditionBound,
			message:  "failed to bind",
			ref:      ResourceReference{Name: "name", Kind: "httproute"},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			cond := NewRouteCondition(tc.condType, tc.status, tc.reason, tc.message, tc.ref)
			expectedCond := Condition{
				Type:     string(tc.condType),
				Status:   tc.status,
				Reason:   string(tc.reason),
				Message:  tc.message,
				Resource: &tc.ref,
			}
			if !cond.IsSame(&expectedCond) {
				t.Errorf("Expected condition to be\n%+v\ngot\n%+v", expectedCond, cond)
			}
		})
	}
}

func TestNewRouteInvalidCombinationsCausePanic(t *testing.T) {
	// This is not an exhaustive list of all invalid combinations, just a few to confirm
	testCases := map[string]struct {
		status   ConditionStatus
		reason   RouteConditionReason
		condType RouteConditionType
		message  string
		ref      ResourceReference
	}{
		"reason and condition type are valid but status is not": {
			status:   ConditionStatusTrue,
			reason:   RouteReasonNotAllowedByListeners,
			condType: RouteConditionAccepted,
			message:  "almost there",
			ref:      ResourceReference{Name: "name", Kind: "httproute"},
		},
		"reason and status are valid but condition type is not": {
			status:   ConditionStatusFalse,
			reason:   RouteReasonRefNotPermitted,
			condType: RouteConditionBound,
			message:  "not quite",
			ref:      ResourceReference{Name: "name", Kind: "httproute"},
		},
		"condition type and status are valid but status is not": {
			status:   ConditionStatusUnknown,
			reason:   RouteReasonBound,
			condType: RouteConditionBound,
			message:  "still not working",
			ref:      ResourceReference{Name: "name", Kind: "httproute"},
		},
		"all are invalid": {
			status:   ConditionStatusUnknown,
			reason:   RouteReasonGatewayNotFound,
			condType: RouteConditionResolvedRefs,
			message:  "still not working",
			ref:      ResourceReference{Name: "name", Kind: "httproute"},
		},
		"pass something other than a condition status": {
			status:   ConditionStatus("hello"),
			reason:   RouteReasonAccepted,
			condType: RouteConditionAccepted,
			message:  "still not working",
			ref:      ResourceReference{Name: "name", Kind: "httproute"},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("Expected combination %+v to be invalid", tc)
				}
			}()
			_ = NewRouteCondition(tc.condType, tc.status, tc.reason, tc.message, tc.ref)
		})
	}
}
