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
			reason:   GatewayReasonNoConflicts,
			condType: GatewayConditionConflicted,
			message:  "almost there",
			ref:      ResourceReference{Name: "name", Kind: "httproute"},
		},
		"reason and status are valid but condition type is not": {
			status:   ConditionStatusFalse,
			reason:   GatewayReasonNoConflicts,
			condType: GatewayConditionResolvedRefs,
			message:  "not quite",
			ref:      ResourceReference{Name: "name", Kind: "httproute"},
		},
		"condition type and status are valid but status is not": {
			status:   ConditionStatusTrue,
			reason:   GatewayReasonNoConflicts,
			condType: GatewayConditionAccepted,
			message:  "still not working",
			ref:      ResourceReference{Name: "name", Kind: "httproute"},
		},
		"all are invalid": {
			status:   ConditionStatusUnknown,
			reason:   GatewayReasonAccepted,
			condType: GatewayConditionResolvedRefs,
			message:  "still not working",
			ref:      ResourceReference{Name: "name", Kind: "httproute"},
		},
		"pass something other than a condition status": {
			status:   ConditionStatus("hello"),
			reason:   GatewayReasonAccepted,
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
			reason:   RouteReasonNoUpstreamServicesTargeted,
			condType: RouteConditionAccepted,
			message:  "almost there",
			ref:      ResourceReference{Name: "name", Kind: "httproute"},
		},
		"reason and status are valid but condition type is not": {
			status:   ConditionStatusFalse,
			reason:   RouteReasonInvalidDiscoveryChain,
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
			condType: RouteConditionBound,
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
