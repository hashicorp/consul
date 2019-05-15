package structs

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestServiceResolverConfigEntry(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name         string
		entry        *ServiceResolverConfigEntry
		normalizeErr string
		validateErr  string
		// check is called between normalize and validate
		check func(t *testing.T, entry *ServiceResolverConfigEntry)
	}{
		{
			name:         "nil",
			entry:        nil,
			normalizeErr: "config entry is nil",
		},
		{
			name:        "no name",
			entry:       &ServiceResolverConfigEntry{},
			validateErr: "Name is required",
		},
		{
			name: "empty",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
			},
		},
		{
			name: "empty subset name",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Subsets: map[string]ServiceResolverSubset{
					"": {OnlyPassing: true},
				},
			},
			validateErr: "Subset defined with empty name",
		},
		{
			name: "default subset does not exist",
			entry: &ServiceResolverConfigEntry{
				Kind:          ServiceResolver,
				Name:          "test",
				DefaultSubset: "gone",
				Subsets: map[string]ServiceResolverSubset{
					"v1": {Filter: "ServiceMeta.version == v1"},
				},
			},
			validateErr: `DefaultSubset "gone" is not a valid subset`,
		},
		{
			name: "default subset does exist",
			entry: &ServiceResolverConfigEntry{
				Kind:          ServiceResolver,
				Name:          "test",
				DefaultSubset: "v1",
				Subsets: map[string]ServiceResolverSubset{
					"v1": {Filter: "ServiceMeta.version == v1"},
				},
			},
		},
		{
			name: "empty redirect",
			entry: &ServiceResolverConfigEntry{
				Kind:     ServiceResolver,
				Name:     "test",
				Redirect: &ServiceResolverRedirect{},
			},
			validateErr: "Redirect is empty",
		},
		{
			name: "redirect subset with no service",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Redirect: &ServiceResolverRedirect{
					ServiceSubset: "next",
				},
			},
			validateErr: "Redirect.ServiceSubset defined without Redirect.Service",
		},
		{
			name: "redirect namespace with no service",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Redirect: &ServiceResolverRedirect{
					Namespace: "alternate",
				},
			},
			validateErr: "Redirect.Namespace defined without Redirect.Service",
		},
		{
			name: "self redirect with invalid subset",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Redirect: &ServiceResolverRedirect{
					Service:       "test",
					ServiceSubset: "gone",
				},
			},
			validateErr: `Redirect.ServiceSubset "gone" is not a valid subset of "test"`,
		},
		{
			name: "self redirect with valid subset",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Redirect: &ServiceResolverRedirect{
					Service:       "test",
					ServiceSubset: "v1",
				},
				Subsets: map[string]ServiceResolverSubset{
					"v1": {Filter: "ServiceMeta.version == v1"},
				},
			},
		},
		{
			name: "simple wildcard failover",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Failover: map[string]ServiceResolverFailover{
					"*": ServiceResolverFailover{
						Datacenters: []string{"dc2"},
					},
				},
			},
		},
		{
			name: "failover for missing subset",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Failover: map[string]ServiceResolverFailover{
					"gone": ServiceResolverFailover{
						Datacenters: []string{"dc2"},
					},
				},
			},
			validateErr: `Bad Failover["gone"]: not a valid subset`,
		},
		{
			name: "failover for present subset",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Subsets: map[string]ServiceResolverSubset{
					"v1": {Filter: "ServiceMeta.version == v1"},
				},
				Failover: map[string]ServiceResolverFailover{
					"v1": ServiceResolverFailover{
						Datacenters: []string{"dc2"},
					},
				},
			},
		},
		{
			name: "failover empty",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Subsets: map[string]ServiceResolverSubset{
					"v1": {Filter: "ServiceMeta.version == v1"},
				},
				Failover: map[string]ServiceResolverFailover{
					"v1": ServiceResolverFailover{},
				},
			},
			validateErr: `Bad Failover["v1"] one of Service, ServiceSubset, or Datacenters is required`,
		},
		{
			name: "failover to self using invalid subset",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Subsets: map[string]ServiceResolverSubset{
					"v1": {Filter: "ServiceMeta.version == v1"},
				},
				Failover: map[string]ServiceResolverFailover{
					"v1": ServiceResolverFailover{
						Service:       "test",
						ServiceSubset: "gone",
					},
				},
			},
			validateErr: `Bad Failover["v1"].ServiceSubset "gone" is not a valid subset of "test"`,
		},
		{
			name: "failover to self using valid subset",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Subsets: map[string]ServiceResolverSubset{
					"v1": {Filter: "ServiceMeta.version == v1"},
					"v2": {Filter: "ServiceMeta.version == v2"},
				},
				Failover: map[string]ServiceResolverFailover{
					"v1": ServiceResolverFailover{
						Service:       "test",
						ServiceSubset: "v2",
					},
				},
			},
		},
		{
			name: "failover with invalid overprovisioning factor",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Failover: map[string]ServiceResolverFailover{
					"*": ServiceResolverFailover{
						Service:                "backup",
						OverprovisioningFactor: -1,
					},
				},
			},
			validateErr: `Bad Failover["*"].OverprovisioningFactor '-1', must be >= 0`,
		},
		{
			name: "failover with empty datacenters in list",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Failover: map[string]ServiceResolverFailover{
					"*": ServiceResolverFailover{
						Service:     "backup",
						Datacenters: []string{"", "dc2", "dc3"},
					},
				},
			},
			validateErr: `Bad Failover["*"].Datacenters: found empty datacenter`,
		},
		{
			name: "bad connect timeout",
			entry: &ServiceResolverConfigEntry{
				Kind:           ServiceResolver,
				Name:           "test",
				ConnectTimeout: -1 * time.Second,
			},
			validateErr: "Bad ConnectTimeout",
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := tc.entry.Normalize()
			if tc.normalizeErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.normalizeErr)
				return
			}
			require.NoError(t, err)

			if tc.check != nil {
				tc.check(t, tc.entry)
			}

			err = tc.entry.Validate()
			if tc.validateErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.validateErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestServiceSplitterConfigEntry(t *testing.T) {
	t.Parallel()

	makesplitter := func(splits ...ServiceSplit) *ServiceSplitterConfigEntry {
		return &ServiceSplitterConfigEntry{
			Kind:   ServiceSplitter,
			Name:   "test",
			Splits: splits,
		}
	}

	makesplit := func(weight float32, service, serviceSubset, namespace string) ServiceSplit {
		return ServiceSplit{
			Weight:        weight,
			Service:       service,
			ServiceSubset: serviceSubset,
			Namespace:     namespace,
		}
	}

	for _, tc := range []struct {
		name         string
		entry        *ServiceSplitterConfigEntry
		normalizeErr string
		validateErr  string
		// check is called between normalize and validate
		check func(t *testing.T, entry *ServiceSplitterConfigEntry)
	}{
		{
			name:         "nil",
			entry:        nil,
			normalizeErr: "config entry is nil",
		},
		{
			name:        "no name",
			entry:       &ServiceSplitterConfigEntry{},
			validateErr: "Name is required",
		},
		{
			name:        "empty",
			entry:       makesplitter(),
			validateErr: "no splits configured",
		},
		{
			name: "1 split",
			entry: makesplitter(
				makesplit(100, "test", "", ""),
			),
			check: func(t *testing.T, entry *ServiceSplitterConfigEntry) {
				require.Equal(t, float32(100), entry.Splits[0].Weight)
			},
		},
		{
			name: "1 split not enough weight",
			entry: makesplitter(
				makesplit(99.99, "test", "", ""),
			),
			check: func(t *testing.T, entry *ServiceSplitterConfigEntry) {
				require.Equal(t, float32(99.99), entry.Splits[0].Weight)
			},
			validateErr: "the sum of all split weights must be 100",
		},
		{
			name: "1 split too much weight",
			entry: makesplitter(
				makesplit(100.01, "test", "", ""),
			),
			check: func(t *testing.T, entry *ServiceSplitterConfigEntry) {
				require.Equal(t, float32(100.01), entry.Splits[0].Weight)
			},
			validateErr: "the sum of all split weights must be 100",
		},
		{
			name: "2 splits",
			entry: makesplitter(
				makesplit(99, "test", "v1", ""),
				makesplit(1, "test", "v2", ""),
			),
			check: func(t *testing.T, entry *ServiceSplitterConfigEntry) {
				require.Equal(t, float32(99), entry.Splits[0].Weight)
				require.Equal(t, float32(1), entry.Splits[1].Weight)
			},
		},
		{
			name: "2 splits - rounded up to smallest units",
			entry: makesplitter(
				makesplit(99.999, "test", "v1", ""),
				makesplit(0.001, "test", "v2", ""),
			),
			check: func(t *testing.T, entry *ServiceSplitterConfigEntry) {
				require.Equal(t, float32(100), entry.Splits[0].Weight)
				require.Equal(t, float32(0), entry.Splits[1].Weight)
			},
		},
		{
			name: "2 splits not enough weight",
			entry: makesplitter(
				makesplit(99.98, "test", "v1", ""),
				makesplit(0.01, "test", "v2", ""),
			),
			check: func(t *testing.T, entry *ServiceSplitterConfigEntry) {
				require.Equal(t, float32(99.98), entry.Splits[0].Weight)
				require.Equal(t, float32(0.01), entry.Splits[1].Weight)
			},
			validateErr: "the sum of all split weights must be 100",
		},
		{
			name: "2 splits too much weight",
			entry: makesplitter(
				makesplit(100, "test", "v1", ""),
				makesplit(0.01, "test", "v2", ""),
			),
			check: func(t *testing.T, entry *ServiceSplitterConfigEntry) {
				require.Equal(t, float32(100), entry.Splits[0].Weight)
				require.Equal(t, float32(0.01), entry.Splits[1].Weight)
			},
			validateErr: "the sum of all split weights must be 100",
		},
		{
			name: "3 splits",
			entry: makesplitter(
				makesplit(34, "test", "v1", ""),
				makesplit(33, "test", "v2", ""),
				makesplit(33, "test", "v3", ""),
			),
			check: func(t *testing.T, entry *ServiceSplitterConfigEntry) {
				require.Equal(t, float32(34), entry.Splits[0].Weight)
				require.Equal(t, float32(33), entry.Splits[1].Weight)
				require.Equal(t, float32(33), entry.Splits[2].Weight)
			},
		},
		{
			name: "3 splits one duplicated same weights",
			entry: makesplitter(
				makesplit(34, "test", "v1", ""),
				makesplit(33, "test", "v2", ""),
				makesplit(33, "test", "v2", ""),
			),
			check: func(t *testing.T, entry *ServiceSplitterConfigEntry) {
				require.Equal(t, float32(34), entry.Splits[0].Weight)
				require.Equal(t, float32(33), entry.Splits[1].Weight)
				require.Equal(t, float32(33), entry.Splits[2].Weight)
			},
			validateErr: "split destination occurs more than once",
		},
		{
			name: "3 splits one duplicated diff weights",
			entry: makesplitter(
				makesplit(34, "test", "v1", ""),
				makesplit(33, "test", "v2", ""),
				makesplit(33, "test", "v1", ""),
			),
			check: func(t *testing.T, entry *ServiceSplitterConfigEntry) {
				require.Equal(t, float32(34), entry.Splits[0].Weight)
				require.Equal(t, float32(33), entry.Splits[1].Weight)
				require.Equal(t, float32(33), entry.Splits[2].Weight)
			},
			validateErr: "split destination occurs more than once",
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := tc.entry.Normalize()
			if tc.normalizeErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.normalizeErr)
				return
			}
			require.NoError(t, err)

			if tc.check != nil {
				tc.check(t, tc.entry)
			}

			err = tc.entry.Validate()
			if tc.validateErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.validateErr)
				return
			}
			require.NoError(t, err)
		})
	}
}
