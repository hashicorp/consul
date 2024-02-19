// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func TestTypeRegistration(t *testing.T) {
	// This test will ensure that all the required types have been registered
	// It is not trying to determine whether the settings for the registrations
	// are correct as that would amount more or less to hardcoding structs
	// from types.go a second time here.
	requiredKinds := []string{
		pbcatalog.WorkloadKind,
		pbcatalog.ServiceKind,
		pbcatalog.ServiceEndpointsKind,
		pbcatalog.NodeKind,
		pbcatalog.HealthStatusKind,
		// todo (ishustava): uncomment once we implement these
		//pbcatalog.HealthChecksKind,
		//pbcatalog.VirtualIPsKind,
	}

	r := resource.NewRegistry()
	Register(r)

	for _, kind := range requiredKinds {
		t.Run(kind, func(t *testing.T) {
			registration, ok := r.Resolve(&pbresource.Type{
				Group:        pbcatalog.GroupName,
				GroupVersion: pbcatalog.Version,
				Kind:         kind,
			})

			require.True(t, ok, "Resource kind %s has not been registered to the type registry", kind)
			require.NotNil(t, registration, "Registration for %s was found but is nil", kind)
		})
	}
}
