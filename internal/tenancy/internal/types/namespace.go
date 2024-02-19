// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/internal/dnsutil"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
	pbtenancy "github.com/hashicorp/consul/proto-public/pbtenancy/v2beta1"
)

func RegisterNamespace(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     pbtenancy.NamespaceType,
		Proto:    &pbtenancy.Namespace{},
		Scope:    resource.ScopePartition,
		Mutate:   MutateNamespace,
		Validate: ValidateNamespace,
		// ACLs: TODO
	})
}

// MutateNamespace sets the resource data if nil since description and
// other fields are optional.
func MutateNamespace(res *pbresource.Resource) error {
	if res.Data != nil {
		return nil
	}
	data, err := anypb.New(&pbtenancy.Namespace{})
	if err != nil {
		return err
	}
	res.Data = data
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

	if !dnsutil.IsValidLabel(res.Id.Name) {
		return fmt.Errorf("namespace name %q is not a valid DNS hostname", res.Id.Name)
	}

	switch strings.ToLower(res.Id.Name) {
	case "system", "universal", "operator", "root":
		return fmt.Errorf("namespace %q is reserved for future internal use", res.Id.Name)
	default:

		return nil
	}
}
