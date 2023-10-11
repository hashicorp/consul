// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package proxyconfiguration

import (
	"fmt"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto/private/prototest"
)

func TestSortProxyConfigurations(t *testing.T) {
	workloadName := "foo-123"
	cases := map[string]struct {
		selectors        []*pbcatalog.WorkloadSelector
		expSortedIndices []int
	}{
		"first matched by name, second by prefix": {
			selectors: []*pbcatalog.WorkloadSelector{
				{
					Names: []string{workloadName},
				},
				{
					Prefixes: []string{"foo-"},
				},
			},
			expSortedIndices: []int{0, 1},
		},
		"first matched by prefix, second by name": {
			selectors: []*pbcatalog.WorkloadSelector{
				{
					Prefixes: []string{"foo-"},
				},
				{
					Names: []string{workloadName},
				},
			},
			expSortedIndices: []int{1, 0},
		},
		"both matched by name (sorted order should match the order of creation)": {
			selectors: []*pbcatalog.WorkloadSelector{
				{
					Names: []string{workloadName},
				},
				{
					Names: []string{workloadName},
				},
			},
			expSortedIndices: []int{0, 1},
		},
		"both matched by different prefix": {
			selectors: []*pbcatalog.WorkloadSelector{
				{
					Prefixes: []string{"foo"},
				},
				{
					Prefixes: []string{"foo-"},
				},
			},
			expSortedIndices: []int{1, 0},
		},
		"both matched by the same prefix": {
			selectors: []*pbcatalog.WorkloadSelector{
				{
					Prefixes: []string{"foo-"},
				},
				{
					Prefixes: []string{"foo-"},
				},
			},
			expSortedIndices: []int{0, 1},
		},
		"both matched by the multiple different prefixes": {
			selectors: []*pbcatalog.WorkloadSelector{
				{
					Prefixes: []string{"foo-1", "foo-"},
				},
				{
					Prefixes: []string{"foo-1", "foo-12"},
				},
			},
			expSortedIndices: []int{1, 0},
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			resourceClient := svctest.RunResourceService(t, types.Register)

			var decProxyCfgs []*types.DecodedProxyConfiguration
			for i, ws := range c.selectors {
				proxyCfg := &pbmesh.ProxyConfiguration{
					Workloads: ws,
				}
				resName := fmt.Sprintf("cfg-%d", i)
				proxyCfgRes := resourcetest.Resource(pbmesh.ProxyConfigurationType, resName).
					WithData(t, proxyCfg).
					// We need to run it through resource service so that ULIDs are set.
					Write(t, resourceClient)
				decProxyCfgs = append(decProxyCfgs, resourcetest.MustDecode[*pbmesh.ProxyConfiguration](t, proxyCfgRes))

				// Wait for a few milliseconds so that creation timestamp will always be different between resources.
				time.Sleep(2 * time.Millisecond)
			}

			sortedCfgs := SortProxyConfigurations(decProxyCfgs, workloadName)

			for i, idx := range c.expSortedIndices {
				prototest.AssertDeepEqual(t, decProxyCfgs[idx], sortedCfgs[i])
			}
		})
	}
}

func TestSortProxyConfigurations_SameCreationTime(t *testing.T) {
	var decProxyCfgs []*types.DecodedProxyConfiguration

	proxyCfg := &pbmesh.ProxyConfiguration{
		Workloads: &pbcatalog.WorkloadSelector{
			Names: []string{"foo-123"},
		},
	}

	// Make cfg1 name such that it should appear after cfg2 lexicographically.
	cfg1 := resourcetest.Resource(pbmesh.ProxyConfigurationType, "def-cfg-1").
		WithData(t, proxyCfg).
		Build()
	// Explicitly set ulid. For the first one, we'll just the current timestamp.
	cfg1.Id.Uid = ulid.Make().String()

	decProxyCfgs = append(decProxyCfgs, resourcetest.MustDecode[*pbmesh.ProxyConfiguration](t, cfg1))

	cfg2 := resourcetest.Resource(pbmesh.ProxyConfigurationType, "abc-cfg-2").
		WithData(t, proxyCfg).
		Build()
	// Explicitly set ulid. For the second one, we'll the timestamp of the first one.
	parsedCfg1Ulid := ulid.MustParse(cfg1.Id.Uid)
	cfg2.Id.Uid = ulid.MustNew(parsedCfg1Ulid.Time(), ulid.DefaultEntropy()).String()

	decProxyCfgs = append(decProxyCfgs, resourcetest.MustDecode[*pbmesh.ProxyConfiguration](t, cfg2))

	sortedCfgs := SortProxyConfigurations(decProxyCfgs, "foo-123")

	// We expect that given the same creation timestamp, the second proxy cfg should be first
	// in the sorted order because of its name.
	prototest.AssertDeepEqual(t, decProxyCfgs[0], sortedCfgs[1])
	prototest.AssertDeepEqual(t, decProxyCfgs[1], sortedCfgs[0])
}
