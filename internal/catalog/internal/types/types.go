// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/consul/internal/resource"
)

func Register(r resource.Registry) {
	RegisterWorkload(r)
	RegisterService(r)
	RegisterServiceEndpoints(r)
	RegisterNode(r)
	RegisterHealthStatus(r)
	RegisterFailoverPolicy(r)
	RegisterNodeHealthStatus(r)
	RegisterComputedFailoverPolicy(r)
	// todo (v2): re-register once these resources are implemented.
	//RegisterHealthChecks(r)
	//RegisterDNSPolicy(r)
	//RegisterVirtualIPs(r)
}
