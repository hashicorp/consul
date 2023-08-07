// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package util

import (
	"github.com/hashicorp/consul/testing/deployer/util/internal/ipamutils"
)

// GetPossibleDockerNetworkSubnets returns a copy of the global-scope network list.
func GetPossibleDockerNetworkSubnets() map[string]struct{} {
	list := ipamutils.GetGlobalScopeDefaultNetworks()

	out := make(map[string]struct{})
	for _, ipnet := range list {
		subnet := ipnet.String()
		out[subnet] = struct{}{}
	}
	return out
}
