// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package catalogv2beta1

import (
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

// GetIdentities returns a list of unique identities that this service endpoints points to.
func (s *ServiceEndpoints) GetIdentities() []string {
	uniqueIdentities := make(map[string]struct{})

	for _, ep := range s.GetEndpoints() {
		if ep.GetIdentity() != "" {
			uniqueIdentities[ep.GetIdentity()] = struct{}{}
		}
	}

	if len(uniqueIdentities) == 0 {
		return nil
	}

	identities := maps.Keys(uniqueIdentities)
	slices.Sort(identities)

	return identities
}
