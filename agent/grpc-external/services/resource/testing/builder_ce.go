// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package testing

import (
	"github.com/hashicorp/go-hclog"

	svc "github.com/hashicorp/consul/agent/grpc-external/services/resource"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type Builder struct {
	registry    resource.Registry
	registerFns []func(resource.Registry)
	tenancies   []*pbresource.Tenancy
	aclResolver svc.ACLResolver
	serviceImpl *svc.Server
	cloning     bool
}

func (b *Builder) ensureLicenseManager() {
}

func (b *Builder) newConfig(logger hclog.Logger, backend svc.Backend, tenancyBridge resource.TenancyBridge) *svc.Config {
	return &svc.Config{
		Logger:        logger,
		Registry:      b.registry,
		Backend:       backend,
		ACLResolver:   b.aclResolver,
		TenancyBridge: tenancyBridge,
	}
}
