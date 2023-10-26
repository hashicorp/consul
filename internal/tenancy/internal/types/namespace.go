// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"fmt"
	"strings"

	"github.com/hashicorp/consul/agent/dns"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
	pbtenancy "github.com/hashicorp/consul/proto-public/pbtenancy/v2beta1"
)

func RegisterNamespace(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     pbtenancy.NamespaceType,
		Proto:    &pbtenancy.Namespace{},
		Scope:    resource.ScopePartition,
		Validate: ValidateNamespace,
		Mutate:   MutateNamespace,
		// ACLs: TODO
	})
}

func MutateNamespace(res *pbresource.Resource) error {
	res.Id.Name = strings.ToLower(res.Id.Name)
	return nil
}

func ValidateNamespace(res *pbresource.Resource) error {
	var ns pbtenancy.Namespace

	if err := res.Data.UnmarshalTo(&ns); err != nil {
		return resource.NewErrDataParse(&ns, err)
	}
	if res.Owner != nil {
		return errOwnerNonEmpty
	}

	// it's not allowed to create default/default tenancy
	if res.Id.Name == resource.DefaultNamespaceName && res.Id.Tenancy.Partition == resource.DefaultPartitionName {
		return errInvalidName
	}

	if !dns.IsValidLabel(res.Id.Name) {
		return fmt.Errorf("namespace name %q is not a valid DNS hostname", res.Id.Name)
	}

	switch strings.ToLower(res.Id.Name) {
	case "system", "universal", "operator", "root":
		return fmt.Errorf("namespace %q is reserved for future internal use", res.Id.Name)
	default:

		return nil
	}
}
