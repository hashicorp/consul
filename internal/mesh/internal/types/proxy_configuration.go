// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/sdk/iptables"
)

func RegisterProxyConfiguration(r resource.Registry) {
	r.Register(resource.Registration{
		Type:  pbmesh.ProxyConfigurationType,
		Proto: &pbmesh.ProxyConfiguration{},
		Scope: resource.ScopeNamespace,
		// TODO(rb): add validation for proxy configuration
		Validate: nil,
		Mutate:   MutateProxyConfiguration,
	})
}

func MutateProxyConfiguration(res *pbresource.Resource) error {
	var proxyCfg pbmesh.ProxyConfiguration
	err := res.Data.UnmarshalTo(&proxyCfg)
	if err != nil {
		return resource.NewErrDataParse(&proxyCfg, err)
	}

	changed := false

	// Default the tproxy outbound port.
	if proxyCfg.IsTransparentProxy() {
		if proxyCfg.GetDynamicConfig().GetTransparentProxy() == nil {
			proxyCfg.DynamicConfig.TransparentProxy = &pbmesh.TransparentProxy{
				OutboundListenerPort: iptables.DefaultTProxyOutboundPort,
			}
			changed = true
		} else if proxyCfg.GetDynamicConfig().GetTransparentProxy().OutboundListenerPort == 0 {
			proxyCfg.DynamicConfig.TransparentProxy.OutboundListenerPort = iptables.DefaultTProxyOutboundPort
			changed = true
		}
	}

	if !changed {
		return nil
	}

	return res.Data.MarshalFrom(&proxyCfg)
}
