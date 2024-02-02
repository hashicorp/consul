// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package endpoints

import (
	"sort"
	"strings"

	"github.com/hashicorp/consul/proto-public/pbresource"
)

// GetBoundIdentities returns the unique list of workload identity references
// encoded into a data-bearing status condition on a Service resource by the
// endpoints controller.
//
// This allows a controller to skip watching ServiceEndpoints (which is
// expensive) to discover this data.
func GetBoundIdentities(typ *pbresource.Type, res *pbresource.Resource) []*pbresource.Reference {
	if res.GetStatus() == nil {
		return nil
	}

	status, ok := res.GetStatus()[ControllerID]
	if !ok {
		return nil
	}

	var encoded string
	for _, cond := range status.GetConditions() {
		if cond.GetType() == StatusConditionBoundIdentities && cond.GetState() == pbresource.Condition_STATE_TRUE {
			encoded = cond.GetMessage()
			break
		}
	}
	if len(encoded) <= 2 {
		return nil // it could only ever be [] which is nothing
	}

	n := len(encoded)

	if encoded[0] != '[' || encoded[n-1] != ']' {
		return nil
	}

	trimmed := encoded[1 : n-1]

	identities := strings.Split(trimmed, ",")

	// Ensure determinstic sort so we don't get into infinite-reconcile
	sort.Strings(identities)

	var out []*pbresource.Reference
	for _, id := range identities {
		out = append(out, &pbresource.Reference{
			Type:    typ,
			Name:    id,
			Tenancy: res.Id.Tenancy,
		})
	}

	return out
}
