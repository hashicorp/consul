// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package agent

import (
	"context"
	"fmt"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
)

// enterpriseAgent embeds fields that we only access in consul-enterprise builds
type enterpriseAgent struct{}

// fillAgentServiceEnterpriseMeta is a noop stub for the func defined agent_ent.go
func fillAgentServiceEnterpriseMeta(_ *api.AgentService, _ *acl.EnterpriseMeta) {}

// fillHealthCheckEnterpriseMeta is a noop stub for the func defined agent_ent.go
func fillHealthCheckEnterpriseMeta(_ *api.HealthCheck, _ *acl.EnterpriseMeta) {}

// initEnterprise is a noop stub for the func defined agent_ent.go
func (a *Agent) initEnterprise(consulCfg *consul.Config) error {
	return nil
}

// reloadEnterprise is a noop stub for the func defined agent_ent.go
func (a *Agent) reloadEnterprise(conf *config.RuntimeConfig) error {
	return nil
}

// enterpriseConsulConfig is a noop stub for the func defined in agent_ent.go
func enterpriseConsulConfig(_ *consul.Config, _ *config.RuntimeConfig) {
}

// validateFIPSConfig is a noop stub for the func defined in agent_ent.go
func validateFIPSConfig(_ *config.RuntimeConfig) error {
	return nil
}

// WriteEvent is a noop stub for the func defined agent_ent.go
func (a *Agent) WriteEvent(eventType string, payload interface{}) {
}

// startLicenseManager is used to start the license management process
func (a *Agent) startLicenseManager(_ context.Context) error {
	return nil
}

// stopLicenseManager is used to stop the license management go routines
func (a *Agent) stopLicenseManager() {}

// enterpriseStats outputs all the Agent stats specific to Consul Enterprise
func (a *Agent) enterpriseStats() map[string]map[string]string {
	return nil
}

func (a *Agent) AgentEnterpriseMeta() *acl.EnterpriseMeta {
	return structs.NodeEnterpriseMetaInDefaultPartition()
}

func (a *Agent) registerEntCache() {}

func (*Agent) fillEnterpriseProxyDataSources(*proxycfg.DataSources) {}

func (a *Agent) writeAuditRPCEvent(_ string, _ string) interface{} {
	return nil
}

func validateEnterpriseMeshPortConfig(service *structs.NodeService) error {
	if service == nil {
		return nil
	}

	if len(service.Ports) > 0 {
		if service.LocallyRegisteredAsSidecar || service.Kind == structs.ServiceKindConnectProxy || service.Connect.SidecarService != nil {
			return fmt.Errorf("named service ports in the service mesh require Consul Enterprise")
		}
	}

	if service.Kind == structs.ServiceKindConnectProxy {
		if len(service.Proxy.LocalServicePorts) > 0 {
			return fmt.Errorf("named service ports in the service mesh require Consul Enterprise")
		}
		for _, upstream := range service.Proxy.Upstreams {
			if upstream.DestinationPort != "" {
				return fmt.Errorf("destination port routing requires Consul Enterprise")
			}
		}
	}

	return nil
}

func copyEnterpriseSidecarServicePorts(_, _ *structs.NodeService) {}
