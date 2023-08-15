package mesh

import (
	"github.com/hashicorp/consul/acl"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
)

// This file contains the implementation of the xds.ConfigSnapshot interface for proxycfg.ConfigSnapshot

// todo (ishustava): this can probably live in the same package as proxy tracker.

type ProxyState struct {
	*pbmesh.ProxyState
}

func NewProxyState(ps *pbmesh.ProxyState) *ProxyState {
	return &ProxyState{ps}
}

// todo (ishustava): for sidecar proxies, all Allow* functions
// 	should return false, but for different gateways we'd need to add it to IR.

func (p *ProxyState) AllowEmptyListeners() bool {
	return false
}

func (p *ProxyState) AllowEmptyRoutes() bool {
	return false
}

func (p *ProxyState) AllowEmptyClusters() bool {
	return false
}

func (p *ProxyState) Authorize(authz acl.Authorizer) error {
	// todo (ishustava): we'll need to implement this once identity policy is implemented

	// Authed OK!
	return nil
}

func (p *ProxyState) LoggerName() string {
	return ""
}
