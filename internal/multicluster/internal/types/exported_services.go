// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"fmt"
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/internal/resource"
	pbmulticluster "github.com/hashicorp/consul/proto-public/pbmulticluster/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/go-multierror"
)

const (
	ExportedServicesKind = "ExportedServices"
)

var (
	ExportedServicesType = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV1Alpha1,
		Kind:         ExportedServicesKind,
	}
)

func RegisterExportedServices(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     pbmulticluster.ExportedServicesType,
		Proto:    &pbmulticluster.ExportedServices{},
		Scope:    resource.ScopeNamespace,
		Validate: ValidateExportedServices,
		ACLs: &resource.ACLHooks{
			Read:  aclReadHookExportedServices,
			Write: aclWriteHookExportedServices,
			List:  aclListHookExportedServices,
		},
	})
}

func ValidateExportedServices(res *pbresource.Resource) error {
	var exportedService pbmulticluster.ExportedServices

	if err := res.Data.UnmarshalTo(&exportedService); err != nil {
		return resource.NewErrDataParse(&exportedService, err)
	}

	var merr error

	if exportedService.Services == nil || len(exportedService.Services) == 0 {
		merr = multierror.Append(merr, resource.ErrInvalidField{
			Name:    "services",
			Wrapped: fmt.Errorf("at least one service must be set"),
		})
	}
	return merr
}

func aclReadHookExportedServices(authorizer acl.Authorizer, authzContext *acl.AuthorizerContext, id *pbresource.ID, _ *pbresource.Resource) error {
	serviceName := id.Name

	return authorizer.ToAllowAuthorizer().ServiceReadAllowed(serviceName, authzContext)
}

func aclWriteHookExportedServices(authorizer acl.Authorizer, authzContext *acl.AuthorizerContext, res *pbresource.Resource) error {
	serviceName := res.Id.Name

	return authorizer.ToAllowAuthorizer().ServiceWriteAllowed(serviceName, authzContext)
}

func aclListHookExportedServices(authorizer acl.Authorizer, authzContext *acl.AuthorizerContext) error {
	return nil
}
