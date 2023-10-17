// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package proxytracker

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
)

// ProxyState is an implementation of the ProxySnapshot interface for pbmesh.ProxyState.
// It is a simple wrapper around pbmesh.ProxyState so that it can be used
// by the ProxyWatcher interface in XDS processing.  This struct is necessary
// because pbmesh.ProxyState is a proto definition and there were complications
// adding these functions directly to that proto definition.
type ProxyState struct {
	*pbmesh.ProxyState
}

// TODO(proxystate): need to modify ProxyState to carry a type/kind (connect proxy, mesh gateway, etc.)
// for sidecar proxies, all Allow* functions
// should return false, but for different gateways we'd need to add it to IR.

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
	// authorize for mesh proxies.
	// TODO(proxystate): implement differently for gateways
	allow := authz.ToAllowAuthorizer()
	if err := allow.IdentityWriteAllowed(p.Identity.Name, resource.AuthorizerContext(p.Identity.Tenancy)); err != nil {
		return err
	}

	return nil
}

func (p *ProxyState) LoggerName() string {
	return ""
}
