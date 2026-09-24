// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package pbconfigentry

import (
	"testing"

	"github.com/hashicorp/consul/agent/structs"
)

func intPtrForLimits(i int) *int { return &i }

func TestUpstreamLimitsRoundTripPreservesUnset(t *testing.T) {
	orig := &structs.UpstreamLimits{MaxConnections: intPtrForLimits(5)}

	var pb UpstreamLimits
	UpstreamLimitsFromStructs(orig, &pb)

	var got structs.UpstreamLimits
	UpstreamLimitsToStructs(&pb, &got)

	if got.MaxConnections == nil || *got.MaxConnections != 5 {
		t.Fatalf("MaxConnections = %v, want 5", got.MaxConnections)
	}
	if got.MaxPendingRequests != nil {
		t.Fatalf("MaxPendingRequests = %d, want nil (unset)", *got.MaxPendingRequests)
	}
	if got.MaxConcurrentRequests != nil {
		t.Fatalf("MaxConcurrentRequests = %d, want nil (unset)", *got.MaxConcurrentRequests)
	}
}
