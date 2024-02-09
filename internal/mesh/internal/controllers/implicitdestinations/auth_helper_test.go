// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package implicitdestinations

import (
	"strings"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/internal/auth"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// TODO: do this properly and export it from internal/auth/exports.go
// This is a crude approximation suitable for this test.
func ReconcileComputedTrafficPermissions(
	t *testing.T,
	client *rtest.Client,
	id *pbresource.ID,
	tpList ...*pbauth.TrafficPermissions,
) *types.DecodedComputedTrafficPermissions {
	// TODO: allow this to take a nil client and still execute all of the proper validations etc.

	require.True(t, resource.EqualType(pbauth.ComputedTrafficPermissionsType, id.GetType()))

	registry := resource.NewRegistry()
	auth.RegisterTypes(registry)

	merged := &pbauth.ComputedTrafficPermissions{}
	added := false
	for _, tp := range tpList {
		name := strings.ToLower(ulid.Make().String())

		// Default to request aligned.
		if tp.Destination == nil {
			tp.Destination = &pbauth.Destination{}
		}
		if tp.Destination.IdentityName == "" {
			tp.Destination.IdentityName = id.Name
		}
		require.Equal(t, id.Name, tp.Destination.IdentityName)

		res := rtest.Resource(pbauth.TrafficPermissionsType, name).
			WithTenancy(id.Tenancy).
			WithData(t, tp).
			Build()
		resourcetest.ValidateAndNormalize(t, registry, res)

		dec := rtest.MustDecode[*pbauth.TrafficPermissions](t, res)

		added = true

		switch dec.Data.Action {
		case pbauth.Action_ACTION_ALLOW:
			merged.AllowPermissions = append(merged.AllowPermissions, dec.Data.Permissions...)
		case pbauth.Action_ACTION_DENY:
			merged.DenyPermissions = append(merged.DenyPermissions, dec.Data.Permissions...)
		default:
			t.Fatalf("Unexpected action: %v", dec.Data.Action)
		}
	}

	if !added {
		merged.IsDefault = true
	}

	var res *pbresource.Resource
	if client != nil {
		res = rtest.ResourceID(id).
			WithData(t, merged).
			Write(t, client)
	} else {
		res = rtest.ResourceID(id).
			WithData(t, merged).
			Build()
		resourcetest.ValidateAndNormalize(t, registry, res)
	}

	return rtest.MustDecode[*pbauth.ComputedTrafficPermissions](t, res)
}
