// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package types

import (
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/go-multierror"
)

const (
	VirtualIPsKind = "VirtualIPs"
)

var (
	VirtualIPsV1Alpha1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV1Alpha1,
		Kind:         VirtualIPsKind,
	}

	VirtualIPsType = VirtualIPsV1Alpha1Type
)

func RegisterVirtualIPs(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     VirtualIPsV1Alpha1Type,
		Proto:    &pbcatalog.VirtualIPs{},
		Validate: ValidateVirtualIPs,
		Scope:    resource.ScopeNamespace,
	})
}

func ValidateVirtualIPs(res *pbresource.Resource) error {
	var vips pbcatalog.VirtualIPs

	if err := res.Data.UnmarshalTo(&vips); err != nil {
		return resource.NewErrDataParse(&vips, err)
	}

	var err error
	for idx, ip := range vips.Ips {
		if vipErr := validateIPAddress(ip.Address); vipErr != nil {
			err = multierror.Append(err, resource.ErrInvalidListElement{
				Name:  "ips",
				Index: idx,
				Wrapped: resource.ErrInvalidField{
					Name:    "address",
					Wrapped: vipErr,
				},
			})
		}
	}
	return err
}
