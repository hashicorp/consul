// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package local

import (
	"github.com/hashicorp/consul/agent/grpc-external/limiter"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/proxycfg-sources/catalog"
	structs "github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// ConfigSource wraps a proxycfg.Manager to create watches on services
// local to the agent (pre-registered by Sync).
type ConfigSource struct {
	manager ConfigManager
}

// NewConfigSource builds a ConfigSource with the given proxycfg.Manager.
func NewConfigSource(cfgMgr ConfigManager) *ConfigSource {
	return &ConfigSource{cfgMgr}
}

func (m *ConfigSource) Watch(proxyID *pbresource.ID, nodeName string, _ string) (<-chan proxycfg.ProxySnapshot,
	limiter.SessionTerminatedChan, proxycfg.CancelFunc, error) {
	serviceID := structs.NewServiceID(proxyID.Name, catalog.GetEnterpriseMetaFromResourceID(proxyID))
	watchCh, cancelWatch := m.manager.Watch(proxycfg.ProxyID{
		ServiceID: serviceID,
		NodeName:  nodeName,

		// Note: we *intentionally* don't set Token here. All watches on local
		// services use the same ACL token, regardless of whatever token is
		// presented in the xDS stream (the token presented to the xDS server
		// is checked before the watch is created).
		Token: "",
	})
	return watchCh, nil, cancelWatch, nil
}
