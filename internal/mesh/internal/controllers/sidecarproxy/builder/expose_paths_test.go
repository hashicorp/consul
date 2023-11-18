// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package builder

import (
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"testing"

	"github.com/stretchr/testify/require"

	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/sdk/testutil"
)

// This file contains tests only for error and edge cases cases. The happy case is tested in local_app_test.go

func TestBuildExposePaths_NilChecks(t *testing.T) {
	resourcetest.RunWithTenancies(func(tenancy *pbresource.Tenancy) {
		testutil.RunStep(t, "proxy cfg is nil", func(t *testing.T) {
			b := New(testProxyStateTemplateID(tenancy), testIdentityRef(tenancy), "foo.consul", "dc1", true, nil)
			require.NotPanics(t, func() {
				b.buildExposePaths(nil)
			})
		})

		testutil.RunStep(t, "dynamic cfg is nil", func(t *testing.T) {
			b := New(testProxyStateTemplateID(tenancy), testIdentityRef(tenancy), "foo.consul", "dc1", true, &pbmesh.ComputedProxyConfiguration{})
			require.NotPanics(t, func() {
				b.buildExposePaths(nil)
			})
		})

		testutil.RunStep(t, "expose cfg is nil", func(t *testing.T) {
			b := New(testProxyStateTemplateID(tenancy), testIdentityRef(tenancy), "foo.consul", "dc1", true, &pbmesh.ComputedProxyConfiguration{
				DynamicConfig: &pbmesh.DynamicConfig{},
			})
			require.NotPanics(t, func() {
				b.buildExposePaths(nil)
			})
		})
	}, t)
}

func TestBuildExposePaths_NoExternalMeshWorkloadAddress(t *testing.T) {
	resourcetest.RunWithTenancies(func(tenancy *pbresource.Tenancy) {
		workload := &pbcatalog.Workload{
			Addresses: []*pbcatalog.WorkloadAddress{
				{Host: "1.1.1.1", External: true},
			},
			Ports: map[string]*pbcatalog.WorkloadPort{
				"tcp":  {Port: 8080},
				"mesh": {Port: 20000},
			},
		}

		proxycfg := &pbmesh.ComputedProxyConfiguration{
			DynamicConfig: &pbmesh.DynamicConfig{
				ExposeConfig: &pbmesh.ExposeConfig{
					ExposePaths: []*pbmesh.ExposePath{
						{
							ListenerPort:  1234,
							LocalPathPort: 9090,
							Path:          "/health",
						},
					},
				},
			},
		}

		b := New(testProxyStateTemplateID(tenancy), testIdentityRef(tenancy), "foo.consul", "dc1", true, proxycfg)
		b.buildExposePaths(workload)
		require.Empty(t, b.proxyStateTemplate.ProxyState.Listeners)
	}, t)
}

func TestBuildExposePaths_InvalidProtocol(t *testing.T) {
	resourcetest.RunWithTenancies(func(tenancy *pbresource.Tenancy) {
		workload := &pbcatalog.Workload{
			Addresses: []*pbcatalog.WorkloadAddress{
				{Host: "1.1.1.1"},
			},
			Ports: map[string]*pbcatalog.WorkloadPort{
				"tcp":  {Port: 8080},
				"mesh": {Port: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
			},
		}

		proxycfg := &pbmesh.ComputedProxyConfiguration{
			DynamicConfig: &pbmesh.DynamicConfig{
				ExposeConfig: &pbmesh.ExposeConfig{
					ExposePaths: []*pbmesh.ExposePath{
						{
							ListenerPort:  1234,
							LocalPathPort: 9090,
							Path:          "/health",
							Protocol:      3,
						},
					},
				},
			},
		}

		b := New(testProxyStateTemplateID(tenancy), testIdentityRef(tenancy), "foo.consul", "dc1", true, proxycfg)
		require.PanicsWithValue(t, "unsupported expose paths protocol", func() {
			b.buildExposePaths(workload)
		})
	}, t)
}
