// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
)

func TestWorkloadIdentityACLs(t *testing.T) {
	registry := resource.NewRegistry()
	Register(registry)

	wid := &pbauth.WorkloadIdentity{}

	cases := map[string]resourcetest.ACLTestCase{
		"no rules": {
			Rules:   ``,
			Typ:     pbauth.WorkloadIdentityType,
			Data:    wid,
			ReadOK:  resourcetest.DENY,
			WriteOK: resourcetest.DENY,
			ListOK:  resourcetest.DEFAULT,
		},
		"workload identity wi1 read, no intentions": {
			Rules:   `identity "test" { policy = "read" }`,
			Typ:     pbauth.WorkloadIdentityType,
			Data:    wid,
			ReadOK:  resourcetest.ALLOW,
			WriteOK: resourcetest.DENY,
			ListOK:  resourcetest.DEFAULT,
		},
		"workload identity wi1 read, deny intentions has no effect": {
			Rules:   `identity "test" { policy = "read", intentions = "deny" }`,
			Typ:     pbauth.WorkloadIdentityType,
			Data:    wid,
			ReadOK:  resourcetest.ALLOW,
			WriteOK: resourcetest.DENY,
			ListOK:  resourcetest.DEFAULT,
		},
		"workload identity wi1 read, intentions read has no effect": {
			Rules:   `identity "test" { policy = "read", intentions = "read" }`,
			Typ:     pbauth.WorkloadIdentityType,
			Data:    wid,
			ReadOK:  resourcetest.ALLOW,
			WriteOK: resourcetest.DENY,
			ListOK:  resourcetest.DEFAULT,
		},
		"workload identity wi1 write, write intentions has no effect": {
			Rules:   `identity "test" { policy = "read", intentions = "write" }`,
			Typ:     pbauth.WorkloadIdentityType,
			Data:    wid,
			ReadOK:  resourcetest.ALLOW,
			WriteOK: resourcetest.DENY,
			ListOK:  resourcetest.DEFAULT,
		},
		"workload identity wi1 write, deny intentions has no effect": {
			Rules:   `identity "test" { policy = "write", intentions = "deny" }`,
			Typ:     pbauth.WorkloadIdentityType,
			Data:    wid,
			ReadOK:  resourcetest.ALLOW,
			WriteOK: resourcetest.ALLOW,
			ListOK:  resourcetest.DEFAULT,
		},
		"workload identity wi1 write, intentions read has no effect": {
			Rules:   `identity "test" { policy = "write", intentions = "read" }`,
			Typ:     pbauth.WorkloadIdentityType,
			Data:    wid,
			ReadOK:  resourcetest.ALLOW,
			WriteOK: resourcetest.ALLOW,
			ListOK:  resourcetest.DEFAULT,
		},
		"workload identity wi1 write, intentions write": {
			Rules:   `identity "test" { policy = "write", intentions = "write" }`,
			Typ:     pbauth.WorkloadIdentityType,
			Data:    wid,
			ReadOK:  resourcetest.ALLOW,
			WriteOK: resourcetest.ALLOW,
			ListOK:  resourcetest.DEFAULT,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			resourcetest.RunACLTestCase(t, tc, registry)
		})
	}
}

func TestWorkloadIdentity_ParseError(t *testing.T) {
	rsc := resourcetest.Resource(pbauth.WorkloadIdentityType, "example").
		WithData(t, &pbauth.TrafficPermissions{}).
		Build()

	err := ValidateWorkloadIdentity(rsc)
	var parseErr resource.ErrDataParse
	require.ErrorAs(t, err, &parseErr)
}
