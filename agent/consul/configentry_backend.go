// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/acl/resolver"
	"github.com/hashicorp/consul/agent/grpc-external/services/configentry"
)

type ConfigEntryBackend struct {
	srv *Server
}

var _ configentry.Backend = (*ConfigEntryBackend)(nil)

// NewConfigEntryBackend returns a configentry.Backend implementation that is bound to the given server.
func NewConfigEntryBackend(srv *Server) *ConfigEntryBackend {
	return &ConfigEntryBackend{
		srv: srv,
	}
}

func (b *ConfigEntryBackend) EnterpriseCheckPartitions(partition string) error {
	return b.enterpriseCheckPartitions(partition)
}

func (b *ConfigEntryBackend) ResolveTokenAndDefaultMeta(token string, entMeta *acl.EnterpriseMeta, authzCtx *acl.AuthorizerContext) (resolver.Result, error) {
	return b.srv.ResolveTokenAndDefaultMeta(token, entMeta, authzCtx)
}
