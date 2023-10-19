// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestMutateFailoverPolicy(t *testing.T) {
	type testcase struct {
		policyTenancy *pbresource.Tenancy
		failover      *pbcatalog.FailoverPolicy
		expect        *pbcatalog.FailoverPolicy
		expectErr     string
	}

	run := func(t *testing.T, tc testcase) {
		res := resourcetest.Resource(pbcatalog.FailoverPolicyType, "api").
			WithTenancy(tc.policyTenancy).
			WithData(t, tc.failover).
			Build()

		err := MutateFailoverPolicy(res)

		got := resourcetest.MustDecode[*pbcatalog.FailoverPolicy](t, res)

		if tc.expectErr == "" {
			require.NoError(t, err)
			prototest.AssertDeepEqual(t, tc.expect, got.Data)
		} else {
			testutil.RequireErrorContains(t, err, tc.expectErr)
		}
	}

	cases := map[string]testcase{
		"empty-1": {
			failover: &pbcatalog.FailoverPolicy{},
			expect:   &pbcatalog.FailoverPolicy{},
		},
		"empty-config-1": {
			failover: &pbcatalog.FailoverPolicy{
				Config: &pbcatalog.FailoverConfig{},
			},
			expect: &pbcatalog.FailoverPolicy{},
		},
		"empty-config-2": {
			failover: &pbcatalog.FailoverPolicy{
				Config: &pbcatalog.FailoverConfig{
					Destinations: make([]*pbcatalog.FailoverDestination, 0),
				},
			},
			expect: &pbcatalog.FailoverPolicy{},
		},
		"empty-map-1": {
			failover: &pbcatalog.FailoverPolicy{
				PortConfigs: make(map[string]*pbcatalog.FailoverConfig),
			},
			expect: &pbcatalog.FailoverPolicy{},
		},
		"empty-map-config-1": {
			failover: &pbcatalog.FailoverPolicy{
				PortConfigs: map[string]*pbcatalog.FailoverConfig{
					"http": {},
				},
			},
			expect: &pbcatalog.FailoverPolicy{},
		},
		"empty-map-config-2": {
			failover: &pbcatalog.FailoverPolicy{
				PortConfigs: map[string]*pbcatalog.FailoverConfig{
					"http": {
						Destinations: make([]*pbcatalog.FailoverDestination, 0),
					},
				},
			},
			expect: &pbcatalog.FailoverPolicy{},
		},
		"normal": {
			failover: &pbcatalog.FailoverPolicy{
				Config: &pbcatalog.FailoverConfig{
					Mode:    pbcatalog.FailoverMode_FAILOVER_MODE_SEQUENTIAL,
					Regions: []string{"foo", "bar"},
					Destinations: []*pbcatalog.FailoverDestination{
						{Ref: newRef(pbcatalog.ServiceType, "a")},
						{Ref: newRef(pbcatalog.ServiceType, "b")},
					},
				},
				PortConfigs: map[string]*pbcatalog.FailoverConfig{
					"http": {
						Destinations: []*pbcatalog.FailoverDestination{
							{Ref: newRef(pbcatalog.ServiceType, "foo")},
							{Ref: newRef(pbcatalog.ServiceType, "bar")},
						},
					},
					"admin": {
						Destinations: []*pbcatalog.FailoverDestination{
							{Ref: newRef(pbcatalog.ServiceType, "y")},
							{Ref: newRef(pbcatalog.ServiceType, "z")},
						},
					},
				},
			},
			expect: &pbcatalog.FailoverPolicy{
				Config: &pbcatalog.FailoverConfig{
					Mode:    pbcatalog.FailoverMode_FAILOVER_MODE_SEQUENTIAL,
					Regions: []string{"foo", "bar"},
					Destinations: []*pbcatalog.FailoverDestination{
						{Ref: newRef(pbcatalog.ServiceType, "a")},
						{Ref: newRef(pbcatalog.ServiceType, "b")},
					},
				},
				PortConfigs: map[string]*pbcatalog.FailoverConfig{
					"http": {
						Destinations: []*pbcatalog.FailoverDestination{
							{Ref: newRef(pbcatalog.ServiceType, "foo")},
							{Ref: newRef(pbcatalog.ServiceType, "bar")},
						},
					},
					"admin": {
						Destinations: []*pbcatalog.FailoverDestination{
							{Ref: newRef(pbcatalog.ServiceType, "y")},
							{Ref: newRef(pbcatalog.ServiceType, "z")},
						},
					},
				},
			},
		},
		"dest ref tenancy defaulting": {
			policyTenancy: resourcetest.Tenancy("foo.bar"),
			failover: &pbcatalog.FailoverPolicy{
				Config: &pbcatalog.FailoverConfig{
					Mode:    pbcatalog.FailoverMode_FAILOVER_MODE_SEQUENTIAL,
					Regions: []string{"foo", "bar"},
					Destinations: []*pbcatalog.FailoverDestination{
						{Ref: newRefWithTenancy(pbcatalog.ServiceType, "", "api")},
						{Ref: newRefWithTenancy(pbcatalog.ServiceType, ".zim", "api")},
						{Ref: newRefWithTenancy(pbcatalog.ServiceType, "gir.zim", "api")},
					},
				},
				PortConfigs: map[string]*pbcatalog.FailoverConfig{
					"http": {
						Destinations: []*pbcatalog.FailoverDestination{
							{Ref: newRefWithTenancy(pbcatalog.ServiceType, "", "api")},
							{Ref: newRefWithTenancy(pbcatalog.ServiceType, ".luthor", "api")},
							{Ref: newRefWithTenancy(pbcatalog.ServiceType, "lex.luthor", "api")},
						},
					},
				},
			},
			expect: &pbcatalog.FailoverPolicy{
				Config: &pbcatalog.FailoverConfig{
					Mode:    pbcatalog.FailoverMode_FAILOVER_MODE_SEQUENTIAL,
					Regions: []string{"foo", "bar"},
					Destinations: []*pbcatalog.FailoverDestination{
						{Ref: newRefWithTenancy(pbcatalog.ServiceType, "foo.bar", "api")},
						{Ref: newRefWithTenancy(pbcatalog.ServiceType, "foo.zim", "api")},
						{Ref: newRefWithTenancy(pbcatalog.ServiceType, "gir.zim", "api")},
					},
				},
				PortConfigs: map[string]*pbcatalog.FailoverConfig{
					"http": {
						Destinations: []*pbcatalog.FailoverDestination{
							{Ref: newRefWithTenancy(pbcatalog.ServiceType, "foo.bar", "api")},
							{Ref: newRefWithTenancy(pbcatalog.ServiceType, "foo.luthor", "api")},
							{Ref: newRefWithTenancy(pbcatalog.ServiceType, "lex.luthor", "api")},
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestValidateFailoverPolicy(t *testing.T) {
	type configTestcase struct {
		config    *pbcatalog.FailoverConfig
		expectErr string
	}

	type testcase struct {
		failover  *pbcatalog.FailoverPolicy
		expectErr string
	}

	run := func(t *testing.T, tc testcase) {
		res := resourcetest.Resource(pbcatalog.FailoverPolicyType, "api").
			WithTenancy(resource.DefaultNamespacedTenancy()).
			WithData(t, tc.failover).
			Build()

		require.NoError(t, MutateFailoverPolicy(res))

		// Verify that mutate didn't actually change the object.
		got := resourcetest.MustDecode[*pbcatalog.FailoverPolicy](t, res)
		prototest.AssertDeepEqual(t, tc.failover, got.Data)

		err := ValidateFailoverPolicy(res)

		// Verify that validate didn't actually change the object.
		got = resourcetest.MustDecode[*pbcatalog.FailoverPolicy](t, res)
		prototest.AssertDeepEqual(t, tc.failover, got.Data)

		if tc.expectErr == "" {
			require.NoError(t, err)
		} else {
			testutil.RequireErrorContains(t, err, tc.expectErr)
		}
	}

	configCases := map[string]configTestcase{
		"dest with sameness": {
			config: &pbcatalog.FailoverConfig{
				Destinations: []*pbcatalog.FailoverDestination{
					{Ref: newRef(pbcatalog.ServiceType, "api-backup")},
				},
				SamenessGroup: "blah",
			},
			// TODO(v2): uncomment after this is supported
			// expectErr: `invalid "destinations" field: exactly one of destinations or sameness_group should be set`,
			expectErr: `invalid "sameness_group" field: not supported in this release`,
		},
		"dest without sameness": {
			config: &pbcatalog.FailoverConfig{
				Destinations: []*pbcatalog.FailoverDestination{
					{Ref: newRef(pbcatalog.ServiceType, "api-backup")},
				},
			},
		},
		"sameness without dest": {
			config: &pbcatalog.FailoverConfig{
				SamenessGroup: "blah",
			},
			// TODO(v2): remove after this is supported
			expectErr: `invalid "sameness_group" field: not supported in this release`,
		},
		"regions without dest": {
			config: &pbcatalog.FailoverConfig{
				Regions: []string{"us-east1", "us-west2"},
			},
			// TODO(v2): remove after this is supported
			expectErr: `invalid "regions" field: not supported in this release`,
		},
		"mode without dest": {
			config: &pbcatalog.FailoverConfig{
				Mode: pbcatalog.FailoverMode_FAILOVER_MODE_SEQUENTIAL,
			},
			// TODO(v2): remove after this is supported
			expectErr: `invalid "mode" field: not supported in this release`,
		},
		// TODO(v2): uncomment after this is supported
		// "mode: invalid": {
		// 	config: &pbcatalog.FailoverConfig{
		// 		Mode: 99,
		// 		Destinations: []*pbcatalog.FailoverDestination{
		// 			{Ref: newRef(pbcatalog.ServiceType, "api-backup")},
		// 		},
		// 	},
		// 	expectErr: `invalid "mode" field: not a supported enum value: 99`,
		// },
		"dest: no ref": {
			config: &pbcatalog.FailoverConfig{
				Destinations: []*pbcatalog.FailoverDestination{
					{},
				},
			},
			expectErr: `invalid element at index 0 of list "destinations": invalid "ref" field: missing required field`,
		},
		"dest: non-service ref": {
			config: &pbcatalog.FailoverConfig{
				Destinations: []*pbcatalog.FailoverDestination{
					{Ref: newRef(pbcatalog.WorkloadType, "api-backup")},
				},
			},
			expectErr: `invalid element at index 0 of list "destinations": invalid "ref" field: invalid "type" field: reference must have type catalog.v2beta1.Service`,
		},
		"dest: ref with section": {
			config: &pbcatalog.FailoverConfig{
				Destinations: []*pbcatalog.FailoverDestination{
					{Ref: resourcetest.Resource(pbcatalog.ServiceType, "api").WithTenancy(resource.DefaultNamespacedTenancy()).Reference("blah")},
				},
			},
			expectErr: `invalid element at index 0 of list "destinations": invalid "ref" field: invalid "section" field: section cannot be set here`,
		},
		// TODO(v2/peering): re-enable when peering can exist
		// "dest: ref peer and datacenter": {
		// 	config: &pbcatalog.FailoverConfig{
		// 		Destinations: []*pbcatalog.FailoverDestination{
		// 			{Ref: newRefWithPeer(pbcatalog.ServiceType, "api", "peer1"), Datacenter: "dc2"},
		// 		},
		// 	},
		// 	expectErr: `invalid element at index 0 of list "destinations": invalid "datacenter" field: ref.tenancy.peer_name and datacenter are mutually exclusive fields`,
		// },
		// TODO(v2/peering): re-enable when peering can exist
		// "dest: ref peer without datacenter": {
		// 	config: &pbcatalog.FailoverConfig{
		// 		Destinations: []*pbcatalog.FailoverDestination{
		// 			{Ref: newRefWithPeer(pbcatalog.ServiceType, "api", "peer1")},
		// 		},
		// 	},
		// },
		"dest: ref datacenter without peer": {
			config: &pbcatalog.FailoverConfig{
				Destinations: []*pbcatalog.FailoverDestination{
					{Ref: newRef(pbcatalog.ServiceType, "api"), Datacenter: "dc2"},
				},
			},
		},
	}

	cases := map[string]testcase{
		// emptiness
		"empty": {
			failover:  &pbcatalog.FailoverPolicy{},
			expectErr: `invalid "config" field: at least one of config or port_configs must be set`,
		},
		"non-empty: one port config but no plain config": {
			failover: &pbcatalog.FailoverPolicy{
				PortConfigs: map[string]*pbcatalog.FailoverConfig{
					"http": {
						Destinations: []*pbcatalog.FailoverDestination{
							{Ref: newRef(pbcatalog.ServiceType, "api-backup")},
						},
					},
				},
			},
		},
		"non-empty: some plain config but no port configs": {
			failover: &pbcatalog.FailoverPolicy{
				Config: &pbcatalog.FailoverConfig{
					Destinations: []*pbcatalog.FailoverDestination{
						{Ref: newRef(pbcatalog.ServiceType, "api-backup")},
					},
				},
			},
		},
		// plain config
		"plain config: bad dest: any port name": {
			failover: &pbcatalog.FailoverPolicy{
				Config: &pbcatalog.FailoverConfig{
					Destinations: []*pbcatalog.FailoverDestination{
						{Ref: newRef(pbcatalog.ServiceType, "api-backup"), Port: "web"},
					},
				},
			},
			expectErr: `invalid "config" field: invalid element at index 0 of list "destinations": invalid "port" field: ports cannot be specified explicitly for the general failover section since it relies upon port alignment`,
		},
		// ported config
		"ported config: bad dest: invalid port name": {
			failover: &pbcatalog.FailoverPolicy{
				PortConfigs: map[string]*pbcatalog.FailoverConfig{
					"http": {
						Destinations: []*pbcatalog.FailoverDestination{
							{Ref: newRef(pbcatalog.ServiceType, "api-backup"), Port: "$bad$"},
						},
					},
				},
			},
			expectErr: `invalid value of key "http" within port_configs: invalid element at index 0 of list "destinations": invalid "port" field: value must match regex: ^[a-z0-9]([a-z0-9\-_]*[a-z0-9])?$`,
		},
		"ported config: bad ported in map": {
			failover: &pbcatalog.FailoverPolicy{
				PortConfigs: map[string]*pbcatalog.FailoverConfig{
					"$bad$": {
						Destinations: []*pbcatalog.FailoverDestination{
							{Ref: newRef(pbcatalog.ServiceType, "api-backup"), Port: "http"},
						},
					},
				},
			},
			expectErr: `map port_configs contains an invalid key - "$bad$": value must match regex: ^[a-z0-9]([a-z0-9\-_]*[a-z0-9])?$`,
		},
	}

	maybeWrap := func(wrapPrefix, base string) string {
		if base != "" {
			return wrapPrefix + base
		}
		return ""
	}

	for name, tc := range configCases {
		cases["plain config: "+name] = testcase{
			failover: &pbcatalog.FailoverPolicy{
				Config: proto.Clone(tc.config).(*pbcatalog.FailoverConfig),
			},
			expectErr: maybeWrap(`invalid "config" field: `, tc.expectErr),
		}

		cases["ported config: "+name] = testcase{
			failover: &pbcatalog.FailoverPolicy{
				PortConfigs: map[string]*pbcatalog.FailoverConfig{
					"http": proto.Clone(tc.config).(*pbcatalog.FailoverConfig),
				},
			},
			expectErr: maybeWrap(`invalid value of key "http" within port_configs: `, tc.expectErr),
		}
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestSimplifyFailoverPolicy(t *testing.T) {
	registry := resource.NewRegistry()
	Register(registry)

	type testcase struct {
		svc      *pbresource.Resource
		failover *pbresource.Resource
		expect   *pbresource.Resource
	}
	run := func(t *testing.T, tc testcase) {
		// Ensure we only use valid inputs.
		resourcetest.ValidateAndNormalize(t, registry, tc.svc)
		resourcetest.ValidateAndNormalize(t, registry, tc.failover)
		resourcetest.ValidateAndNormalize(t, registry, tc.expect)

		svc := resourcetest.MustDecode[*pbcatalog.Service](t, tc.svc)
		failover := resourcetest.MustDecode[*pbcatalog.FailoverPolicy](t, tc.failover)
		expect := resourcetest.MustDecode[*pbcatalog.FailoverPolicy](t, tc.expect)

		inputFailoverCopy := proto.Clone(failover.Data).(*pbcatalog.FailoverPolicy)

		got := SimplifyFailoverPolicy(svc.Data, failover.Data)
		prototest.AssertDeepEqual(t, expect.Data, got)

		// verify input was not altered
		prototest.AssertDeepEqual(t, inputFailoverCopy, failover.Data)
	}

	newPort := func(name string, virtualPort uint32, protocol pbcatalog.Protocol) *pbcatalog.ServicePort {
		return &pbcatalog.ServicePort{
			VirtualPort: virtualPort,
			TargetPort:  name,
			Protocol:    protocol,
		}
	}

	cases := map[string]testcase{
		"implicit with mesh port skipping": {
			svc: resourcetest.Resource(pbcatalog.ServiceType, "api").
				WithData(t, &pbcatalog.Service{
					Ports: []*pbcatalog.ServicePort{
						newPort("mesh", 21001, pbcatalog.Protocol_PROTOCOL_MESH),
						newPort("http", 8080, pbcatalog.Protocol_PROTOCOL_HTTP),
					},
				}).
				Build(),
			failover: resourcetest.Resource(pbcatalog.FailoverPolicyType, "api").
				WithData(t, &pbcatalog.FailoverPolicy{
					Config: &pbcatalog.FailoverConfig{
						Destinations: []*pbcatalog.FailoverDestination{
							{
								Ref: newRef(pbcatalog.ServiceType, "api-backup"),
							},
						},
					},
				}).
				Build(),
			expect: resourcetest.Resource(pbcatalog.FailoverPolicyType, "api").
				WithData(t, &pbcatalog.FailoverPolicy{
					PortConfigs: map[string]*pbcatalog.FailoverConfig{
						"http": {
							Destinations: []*pbcatalog.FailoverDestination{
								{
									Ref:  newRef(pbcatalog.ServiceType, "api-backup"),
									Port: "http", // port defaulted
								},
							},
						},
					},
				}).
				Build(),
		},
		"explicit with port aligned defaulting": {
			svc: resourcetest.Resource(pbcatalog.ServiceType, "api").
				WithData(t, &pbcatalog.Service{
					Ports: []*pbcatalog.ServicePort{
						newPort("mesh", 9999, pbcatalog.Protocol_PROTOCOL_MESH),
						newPort("http", 8080, pbcatalog.Protocol_PROTOCOL_HTTP),
						newPort("rest", 8282, pbcatalog.Protocol_PROTOCOL_HTTP2),
					},
				}).
				Build(),
			failover: resourcetest.Resource(pbcatalog.FailoverPolicyType, "api").
				WithData(t, &pbcatalog.FailoverPolicy{
					PortConfigs: map[string]*pbcatalog.FailoverConfig{
						"http": {
							Destinations: []*pbcatalog.FailoverDestination{
								{
									Ref:  newRef(pbcatalog.ServiceType, "api-backup"),
									Port: "www",
								},
								{
									Ref: newRef(pbcatalog.ServiceType, "api-double-backup"),
								},
							},
						},
					},
				}).
				Build(),
			expect: resourcetest.Resource(pbcatalog.FailoverPolicyType, "api").
				WithData(t, &pbcatalog.FailoverPolicy{
					PortConfigs: map[string]*pbcatalog.FailoverConfig{
						"http": {
							Destinations: []*pbcatalog.FailoverDestination{
								{
									Ref:  newRef(pbcatalog.ServiceType, "api-backup"),
									Port: "www",
								},
								{
									Ref:  newRef(pbcatalog.ServiceType, "api-double-backup"),
									Port: "http", // port defaulted
								},
							},
						},
					},
				}).
				Build(),
		},
		"implicit port explosion": {
			svc: resourcetest.Resource(pbcatalog.ServiceType, "api").
				WithData(t, &pbcatalog.Service{
					Ports: []*pbcatalog.ServicePort{
						newPort("http", 8080, pbcatalog.Protocol_PROTOCOL_HTTP),
						newPort("rest", 8282, pbcatalog.Protocol_PROTOCOL_HTTP2),
					},
				}).
				Build(),
			failover: resourcetest.Resource(pbcatalog.FailoverPolicyType, "api").
				WithData(t, &pbcatalog.FailoverPolicy{
					Config: &pbcatalog.FailoverConfig{
						Destinations: []*pbcatalog.FailoverDestination{
							{
								Ref: newRef(pbcatalog.ServiceType, "api-backup"),
							},
							{
								Ref: newRef(pbcatalog.ServiceType, "api-double-backup"),
							},
						},
					},
				}).
				Build(),
			expect: resourcetest.Resource(pbcatalog.FailoverPolicyType, "api").
				WithData(t, &pbcatalog.FailoverPolicy{
					PortConfigs: map[string]*pbcatalog.FailoverConfig{
						"http": {
							Destinations: []*pbcatalog.FailoverDestination{
								{
									Ref:  newRef(pbcatalog.ServiceType, "api-backup"),
									Port: "http",
								},
								{
									Ref:  newRef(pbcatalog.ServiceType, "api-double-backup"),
									Port: "http",
								},
							},
						},
						"rest": {
							Destinations: []*pbcatalog.FailoverDestination{
								{
									Ref:  newRef(pbcatalog.ServiceType, "api-backup"),
									Port: "rest",
								},
								{
									Ref:  newRef(pbcatalog.ServiceType, "api-double-backup"),
									Port: "rest",
								},
							},
						},
					},
				}).
				Build(),
		},
		"mixed port explosion with skip": {
			svc: resourcetest.Resource(pbcatalog.ServiceType, "api").
				WithData(t, &pbcatalog.Service{
					Ports: []*pbcatalog.ServicePort{
						newPort("http", 8080, pbcatalog.Protocol_PROTOCOL_HTTP),
						newPort("rest", 8282, pbcatalog.Protocol_PROTOCOL_HTTP2),
					},
				}).
				Build(),
			failover: resourcetest.Resource(pbcatalog.FailoverPolicyType, "api").
				WithData(t, &pbcatalog.FailoverPolicy{
					Config: &pbcatalog.FailoverConfig{
						Destinations: []*pbcatalog.FailoverDestination{
							{
								Ref: newRef(pbcatalog.ServiceType, "api-backup"),
							},
							{
								Ref: newRef(pbcatalog.ServiceType, "api-double-backup"),
							},
						},
					},
					PortConfigs: map[string]*pbcatalog.FailoverConfig{
						"rest": {
							// TODO(v2): uncomment when this works
							// Mode:          pbcatalog.FailoverMode_FAILOVER_MODE_ORDER_BY_LOCALITY,
							// Regions:       []string{"us", "eu"},
							// SamenessGroup: "sameweb",
							Destinations: []*pbcatalog.FailoverDestination{
								{
									Ref:  newRef(pbcatalog.ServiceType, "api-backup"),
									Port: "rest",
								},
								{
									Ref:  newRef(pbcatalog.ServiceType, "api-double-backup"),
									Port: "rest",
								},
							},
						},
					},
				}).
				Build(),
			expect: resourcetest.Resource(pbcatalog.FailoverPolicyType, "api").
				WithData(t, &pbcatalog.FailoverPolicy{
					PortConfigs: map[string]*pbcatalog.FailoverConfig{
						"http": {
							Destinations: []*pbcatalog.FailoverDestination{
								{
									Ref:  newRef(pbcatalog.ServiceType, "api-backup"),
									Port: "http",
								},
								{
									Ref:  newRef(pbcatalog.ServiceType, "api-double-backup"),
									Port: "http",
								},
							},
						},
						"rest": {
							// TODO(v2): uncomment when this works
							// Mode:          pbcatalog.FailoverMode_FAILOVER_MODE_ORDER_BY_LOCALITY,
							// Regions:       []string{"us", "eu"},
							// SamenessGroup: "sameweb",
							Destinations: []*pbcatalog.FailoverDestination{
								{
									Ref:  newRef(pbcatalog.ServiceType, "api-backup"),
									Port: "rest",
								},
								{
									Ref:  newRef(pbcatalog.ServiceType, "api-double-backup"),
									Port: "rest",
								},
							},
						},
					},
				}).
				Build(),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestFailoverPolicyACLs(t *testing.T) {
	// Wire up a registry to generically invoke hooks
	registry := resource.NewRegistry()
	Register(registry)

	newFailover := func(t *testing.T, name, tenancyStr string, destRefs []*pbresource.Reference) []*pbresource.Resource {
		var dr []*pbcatalog.FailoverDestination
		for _, destRef := range destRefs {
			dr = append(dr, &pbcatalog.FailoverDestination{Ref: destRef})
		}

		res1 := resourcetest.Resource(pbcatalog.FailoverPolicyType, name).
			WithTenancy(resourcetest.Tenancy(tenancyStr)).
			WithData(t, &pbcatalog.FailoverPolicy{
				Config: &pbcatalog.FailoverConfig{Destinations: dr},
			}).
			Build()
		resourcetest.ValidateAndNormalize(t, registry, res1)

		res2 := resourcetest.Resource(pbcatalog.FailoverPolicyType, name).
			WithTenancy(resourcetest.Tenancy(tenancyStr)).
			WithData(t, &pbcatalog.FailoverPolicy{
				PortConfigs: map[string]*pbcatalog.FailoverConfig{
					"http": {Destinations: dr},
				},
			}).
			Build()
		resourcetest.ValidateAndNormalize(t, registry, res2)

		return []*pbresource.Resource{res1, res2}
	}

	type testcase struct {
		res     *pbresource.Resource
		rules   string
		check   func(t *testing.T, authz acl.Authorizer, res *pbresource.Resource)
		readOK  string
		writeOK string
	}

	const (
		DENY    = resourcetest.DENY
		ALLOW   = resourcetest.ALLOW
		DEFAULT = resourcetest.DEFAULT
	)

	serviceRef := func(tenancy, name string) *pbresource.Reference {
		return newRefWithTenancy(pbcatalog.ServiceType, tenancy, name)
	}

	resOneDest := func(tenancy, destTenancy string) []*pbresource.Resource {
		return newFailover(t, "api", tenancy, []*pbresource.Reference{
			serviceRef(destTenancy, "dest1"),
		})
	}

	resTwoDests := func(tenancy, destTenancy string) []*pbresource.Resource {
		return newFailover(t, "api", tenancy, []*pbresource.Reference{
			serviceRef(destTenancy, "dest1"),
			serviceRef(destTenancy, "dest2"),
		})
	}

	run := func(t *testing.T, name string, tc resourcetest.ACLTestCase) {
		t.Run(name, func(t *testing.T) {
			resourcetest.RunACLTestCase(t, tc, registry)
		})
	}

	isEnterprise := (structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty() == "default")

	serviceRead := func(partition, namespace, name string) string {
		if isEnterprise {
			return fmt.Sprintf(` partition %q { namespace %q { service %q { policy = "read" } } }`, partition, namespace, name)
		}
		return fmt.Sprintf(` service %q { policy = "read" } `, name)
	}
	serviceWrite := func(partition, namespace, name string) string {
		if isEnterprise {
			return fmt.Sprintf(` partition %q { namespace %q { service %q { policy = "write" } } }`, partition, namespace, name)
		}
		return fmt.Sprintf(` service %q { policy = "write" } `, name)
	}

	assert := func(t *testing.T, name string, rules string, resList []*pbresource.Resource, readOK, writeOK string) {
		for i, res := range resList {
			tc := resourcetest.ACLTestCase{
				AuthCtx: resource.AuthorizerContext(res.Id.Tenancy),
				Res:     res,
				Rules:   rules,
				ReadOK:  readOK,
				WriteOK: writeOK,
				ListOK:  DEFAULT,
			}
			run(t, fmt.Sprintf("%s-%d", name, i), tc)
		}
	}

	tenancies := []string{"default.default"}
	if isEnterprise {
		tenancies = append(tenancies, "default.foo", "alpha.default", "alpha.foo")
	}

	for _, policyTenancyStr := range tenancies {
		t.Run("policy tenancy: "+policyTenancyStr, func(t *testing.T) {
			for _, destTenancyStr := range tenancies {
				t.Run("dest tenancy: "+destTenancyStr, func(t *testing.T) {
					for _, aclTenancyStr := range tenancies {
						t.Run("acl tenancy: "+aclTenancyStr, func(t *testing.T) {
							aclTenancy := resourcetest.Tenancy(aclTenancyStr)

							maybe := func(match string, parentOnly bool) string {
								if policyTenancyStr != aclTenancyStr {
									return DENY
								}
								if !parentOnly && destTenancyStr != aclTenancyStr {
									return DENY
								}
								return match
							}

							t.Run("no rules", func(t *testing.T) {
								rules := ``
								assert(t, "1dest", rules, resOneDest(policyTenancyStr, destTenancyStr), DENY, DENY)
								assert(t, "2dests", rules, resTwoDests(policyTenancyStr, destTenancyStr), DENY, DENY)
							})
							t.Run("api:read", func(t *testing.T) {
								rules := serviceRead(aclTenancy.Partition, aclTenancy.Namespace, "api")
								assert(t, "1dest", rules, resOneDest(policyTenancyStr, destTenancyStr), maybe(ALLOW, true), DENY)
								assert(t, "2dests", rules, resTwoDests(policyTenancyStr, destTenancyStr), maybe(ALLOW, true), DENY)
							})
							t.Run("api:write", func(t *testing.T) {
								rules := serviceWrite(aclTenancy.Partition, aclTenancy.Namespace, "api")
								assert(t, "1dest", rules, resOneDest(policyTenancyStr, destTenancyStr), maybe(ALLOW, true), DENY)
								assert(t, "2dests", rules, resTwoDests(policyTenancyStr, destTenancyStr), maybe(ALLOW, true), DENY)
							})
							t.Run("api:write dest1:read", func(t *testing.T) {
								rules := serviceWrite(aclTenancy.Partition, aclTenancy.Namespace, "api") +
									serviceRead(aclTenancy.Partition, aclTenancy.Namespace, "dest1")
								assert(t, "1dest", rules, resOneDest(policyTenancyStr, destTenancyStr), maybe(ALLOW, true), maybe(ALLOW, false))
								assert(t, "2dests", rules, resTwoDests(policyTenancyStr, destTenancyStr), maybe(ALLOW, true), DENY)
							})
						})
					}
				})
			}
		})
	}
}

func newRef(typ *pbresource.Type, name string) *pbresource.Reference {
	return resourcetest.Resource(typ, name).
		WithTenancy(resource.DefaultNamespacedTenancy()).
		Reference("")
}

func newRefWithTenancy(typ *pbresource.Type, tenancyStr, name string) *pbresource.Reference {
	return resourcetest.Resource(typ, name).
		WithTenancy(resourcetest.Tenancy(tenancyStr)).
		Reference("")
}

func newRefWithPeer(typ *pbresource.Type, name string, peer string) *pbresource.Reference {
	ref := newRef(typ, name)
	ref.Tenancy.PeerName = peer
	return ref
}
