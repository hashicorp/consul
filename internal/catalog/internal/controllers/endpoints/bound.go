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
func GetBoundIdentities(res *pbresource.Resource) []string {
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
	if encoded == "" {
		return nil
	}

	identities := strings.Split(encoded, ",")

	// Ensure determinstic sort so we don't get into infinite-reconcile
	sort.Strings(identities)

	return identities
}
