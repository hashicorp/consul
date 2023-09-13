// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"testing"

	"github.com/stretchr/testify/require"

	meshapi "github.com/hashicorp/consul/api/mesh/v2beta1"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func TestTypeRegistration(t *testing.T) {
	// This test will ensure that all the required types have been registered
	// It is not trying to determine whether the settings for the registrations
	// are correct as that would amount more or less to hardcoding structs
	// from types.go a second time here.
	requiredKinds := []string{
		meshapi.ProxyConfigurationKind,
		meshapi.UpstreamsKind,
		meshapi.UpstreamsConfigurationKind,
		meshapi.ProxyStateTemplateKind,
		meshapi.HTTPRouteKind,
		meshapi.TCPRouteKind,
		meshapi.GRPCRouteKind,
		meshapi.DestinationPolicyKind,
		meshapi.ComputedRoutesKind,
	}

	r := resource.NewRegistry()
	Register(r)

	for _, kind := range requiredKinds {
		t.Run(kind, func(t *testing.T) {
			registration, ok := r.Resolve(&pbresource.Type{
				Group:        meshapi.GroupName,
				GroupVersion: meshapi.CurrentVersion,
				Kind:         kind,
			})

			require.True(t, ok, "Resource kind %s has not been registered to the type registry", kind)
			require.NotNil(t, registration, "Registration for %s was found but is nil", kind)
		})
	}
}
