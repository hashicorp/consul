// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package catalogv1alpha1

import pbresource "github.com/hashicorp/consul/proto-public/pbresource"

// GetUnderlyingDestinations will collect FailoverDestinations from all
// internal fields and bundle them up in one slice.
//
// NOTE: no deduplication occurs.
func (x *FailoverPolicy) GetUnderlyingDestinations() []*FailoverDestination {
	if x == nil {
		return nil
	}

	estimate := 0
	if x.Config != nil {
		estimate += len(x.Config.Destinations)
	}
	for _, pc := range x.PortConfigs {
		estimate += len(pc.Destinations)
	}

	out := make([]*FailoverDestination, 0, estimate)
	if x.Config != nil {
		out = append(out, x.Config.Destinations...)
	}
	for _, pc := range x.PortConfigs {
		out = append(out, pc.Destinations...)
	}
	return out
}

// GetUnderlyingDestinationRefs is like GetUnderlyingDestinations except it
// returns a slice of References.
//
// NOTE: no deduplication occurs.
func (x *FailoverPolicy) GetUnderlyingDestinationRefs() []*pbresource.Reference {
	if x == nil {
		return nil
	}

	dests := x.GetUnderlyingDestinations()

	out := make([]*pbresource.Reference, 0, len(dests))
	for _, dest := range dests {
		if dest.Ref != nil {
			out = append(out, dest.Ref)
		}
	}

	return out
}

// IsEmpty returns true if a config has no definition.
func (x *FailoverConfig) IsEmpty() bool {
	if x == nil {
		return true
	}
	return len(x.Destinations) == 0 &&
		x.Mode == 0 &&
		len(x.Regions) == 0 &&
		x.SamenessGroup == ""
}
