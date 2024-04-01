// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package resource

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
	pbtenancy "github.com/hashicorp/consul/proto-public/pbtenancy/v2beta1"
)

func v2TenancyToV1EntMeta(tenancy *pbresource.Tenancy) *acl.EnterpriseMeta {
	return acl.DefaultEnterpriseMeta()
}

func v1EntMetaToV2Tenancy(reg *resource.Registration, entMeta *acl.EnterpriseMeta, tenancy *pbresource.Tenancy) {
	if (reg.Scope == resource.ScopeNamespace || reg.Scope == resource.ScopePartition) && tenancy.Partition == "" {
		tenancy.Partition = entMeta.PartitionOrDefault()
	}

	if reg.Scope == resource.ScopeNamespace && tenancy.Namespace == "" {
		tenancy.Namespace = entMeta.NamespaceOrDefault()
	}
}

// checkV2Tenancy returns FailedPrecondition error for namespace resource type
// when the "v2tenancy" feature flag is not enabled.
func checkV2Tenancy(useV2Tenancy bool, rtype *pbresource.Type) error {
	if resource.EqualType(rtype, pbtenancy.NamespaceType) && !useV2Tenancy {
		return status.Errorf(codes.FailedPrecondition, "use of the v2 namespace resource requires the \"v2tenancy\" feature flag")
	}
	return nil
}

type Config struct {
	Logger   hclog.Logger
	Registry Registry

	// Backend is the storage backend that will be used for resource persistence.
	Backend     Backend
	ACLResolver ACLResolver
	// TenancyBridge temporarily allows us to use V1 implementations of
	// partitions and namespaces until V2 implementations are available.
	TenancyBridge TenancyBridge

	// UseV2Tenancy is true if the "v2tenancy" experiment is active, false otherwise.
	// Attempts to create v2 tenancy resources (partition or namespace) will fail when the
	// flag is false.
	UseV2Tenancy bool
}

// FeatureCheck does not apply to the community edition.
func (s *Server) FeatureCheck(reg *resource.Registration) error {
	return nil
}
