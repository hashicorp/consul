// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/internal/resource"
	pbmulticluster "github.com/hashicorp/consul/proto-public/pbmulticluster/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func RegisterExportedServices(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     pbmulticluster.ExportedServicesType,
		Proto:    &pbmulticluster.ExportedServices{},
		Scope:    resource.ScopeNamespace,
		Mutate:   MutateExportedServices,
		Validate: ValidateExportedServices,
		ACLs: &resource.ACLHooks{
			Read:  aclReadHookExportedServices,
			Write: aclWriteHookExportedServices,
			List:  aclListHookExportedServices,
		},
	})
}

func aclReadHookExportedServices(authorizer acl.Authorizer, authzContext *acl.AuthorizerContext, _ *pbresource.ID, res *pbresource.Resource) error {
	if res == nil {
		return resource.ErrNeedResource
	}

	var exportedService pbmulticluster.ExportedServices

	if err := res.Data.UnmarshalTo(&exportedService); err != nil {
		return resource.NewErrDataParse(&exportedService, err)
	}

	for _, serviceName := range exportedService.Services {
		if err := authorizer.ToAllowAuthorizer().ServiceReadAllowed(serviceName, authzContext); err != nil {
			return err
		}
	}
	return nil
}

func aclWriteHookExportedServices(authorizer acl.Authorizer, authzContext *acl.AuthorizerContext, res *pbresource.Resource) error {
	var exportedService pbmulticluster.ExportedServices

	if err := res.Data.UnmarshalTo(&exportedService); err != nil {
		return resource.NewErrDataParse(&exportedService, err)
	}

	for _, serviceName := range exportedService.Services {
		if err := authorizer.ToAllowAuthorizer().ServiceWriteAllowed(serviceName, authzContext); err != nil {
			return err
		}
	}
	return nil
}

func aclListHookExportedServices(authorizer acl.Authorizer, authzContext *acl.AuthorizerContext) error {
	// No-op List permission as we want to default to filtering resources
	// from the list using the Read enforcement.
	return nil
}
