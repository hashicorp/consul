// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package consul

import (
	"github.com/hashicorp/consul/agent/connect"
)

func (c *CAManager) validateSupportedIdentityScopesInCertificate(spiffeID connect.CertURI) error {
	switch v := spiffeID.(type) {
	case *connect.SpiffeIDService:
		if v.Namespace != "default" || v.Partition != "default" {
			return connect.InvalidCSRError("Non default partition or namespace is supported in Enterprise only."+
				"Provided namespace is %s and partition is %s", v.Namespace, v.Partition)
		}
	case *connect.SpiffeIDMeshGateway:
		if v.Partition != "default" {
			return connect.InvalidCSRError("Non default partition is supported in Enterprise only."+
				"Provided partition is %s", v.Partition)
		}
	case *connect.SpiffeIDAgent, *connect.SpiffeIDServer:
		return nil
	default:
		c.logger.Trace("spiffe ID type is not expected", "spiffeID", spiffeID, "spiffeIDType", v)
		return connect.InvalidCSRError("SPIFFE ID in CSR must be a service, mesh-gateway, or agent ID")
	}
	return nil
}
