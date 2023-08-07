// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package types

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestMutateFailoverPolicy(t *testing.T) {
	type testcase struct {
		failover  *pbcatalog.FailoverPolicy
		expect    *pbcatalog.FailoverPolicy
		expectErr string
	}

	run := func(t *testing.T, tc testcase) {
		res := resourcetest.Resource(FailoverPolicyType, "api").
			WithData(t, tc.failover).
			Build()

		err := MutateFailoverPolicy(res)

		got := resourcetest.MustDecode[pbcatalog.FailoverPolicy, *pbcatalog.FailoverPolicy](t, res)

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
						{Ref: newRef(ServiceType, "a")},
						{Ref: newRef(ServiceType, "b")},
					},
				},
				PortConfigs: map[string]*pbcatalog.FailoverConfig{
					"http": {
						Destinations: []*pbcatalog.FailoverDestination{
							{Ref: newRef(ServiceType, "foo")},
							{Ref: newRef(ServiceType, "bar")},
						},
					},
					"admin": {
						Destinations: []*pbcatalog.FailoverDestination{
							{Ref: newRef(ServiceType, "y")},
							{Ref: newRef(ServiceType, "z")},
						},
					},
				},
			},
			expect: &pbcatalog.FailoverPolicy{
				Config: &pbcatalog.FailoverConfig{
					Mode:    pbcatalog.FailoverMode_FAILOVER_MODE_SEQUENTIAL,
					Regions: []string{"foo", "bar"},
					Destinations: []*pbcatalog.FailoverDestination{
						{Ref: newRef(ServiceType, "a")},
						{Ref: newRef(ServiceType, "b")},
					},
				},
				PortConfigs: map[string]*pbcatalog.FailoverConfig{
					"http": {
						Destinations: []*pbcatalog.FailoverDestination{
							{Ref: newRef(ServiceType, "foo")},
							{Ref: newRef(ServiceType, "bar")},
						},
					},
					"admin": {
						Destinations: []*pbcatalog.FailoverDestination{
							{Ref: newRef(ServiceType, "y")},
							{Ref: newRef(ServiceType, "z")},
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
		res := resourcetest.Resource(FailoverPolicyType, "api").
			WithData(t, tc.failover).
			Build()

		require.NoError(t, MutateFailoverPolicy(res))

		// Verify that mutate didn't actually change the object.
		got := resourcetest.MustDecode[pbcatalog.FailoverPolicy, *pbcatalog.FailoverPolicy](t, res)
		prototest.AssertDeepEqual(t, tc.failover, got.Data)

		err := ValidateFailoverPolicy(res)

		// Verify that validate didn't actually change the object.
		got = resourcetest.MustDecode[pbcatalog.FailoverPolicy, *pbcatalog.FailoverPolicy](t, res)
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
					{Ref: newRef(ServiceType, "api-backup")},
				},
				SamenessGroup: "blah",
			},
			expectErr: `invalid "destinations" field: exactly one of destinations or sameness_group should be set`,
		},
		"dest without sameness": {
			config: &pbcatalog.FailoverConfig{
				Destinations: []*pbcatalog.FailoverDestination{
					{Ref: newRef(ServiceType, "api-backup")},
				},
			},
		},
		"sameness without dest": {
			config: &pbcatalog.FailoverConfig{
				SamenessGroup: "blah",
			},
		},
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
					{Ref: newRef(WorkloadType, "api-backup")},
				},
			},
			expectErr: `invalid element at index 0 of list "destinations": invalid "ref" field: reference must have type catalog.v1alpha1.Service`,
		},
		"dest: ref with section": {
			config: &pbcatalog.FailoverConfig{
				Destinations: []*pbcatalog.FailoverDestination{
					{Ref: resourcetest.Resource(ServiceType, "api").Reference("blah")},
				},
			},
			expectErr: `invalid element at index 0 of list "destinations": invalid "ref" field: invalid "section" field: section not supported for failover policy dest refs`,
		},
		"dest: ref peer and datacenter": {
			config: &pbcatalog.FailoverConfig{
				Destinations: []*pbcatalog.FailoverDestination{
					{Ref: newRefWithPeer(ServiceType, "api", "peer1"), Datacenter: "dc2"},
				},
			},
			expectErr: `invalid element at index 0 of list "destinations": invalid "datacenter" field: ref.tenancy.peer_name and datacenter are mutually exclusive fields`,
		},
		"dest: ref peer without datacenter": {
			config: &pbcatalog.FailoverConfig{
				Destinations: []*pbcatalog.FailoverDestination{
					{Ref: newRefWithPeer(ServiceType, "api", "peer1")},
				},
			},
		},
		"dest: ref datacenter without peer": {
			config: &pbcatalog.FailoverConfig{
				Destinations: []*pbcatalog.FailoverDestination{
					{Ref: newRef(ServiceType, "api"), Datacenter: "dc2"},
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
							{Ref: newRef(ServiceType, "api-backup")},
						},
					},
				},
			},
		},
		"non-empty: some plain config but no port configs": {
			failover: &pbcatalog.FailoverPolicy{
				Config: &pbcatalog.FailoverConfig{
					Destinations: []*pbcatalog.FailoverDestination{
						{Ref: newRef(ServiceType, "api-backup")},
					},
				},
			},
		},
		// plain config
		"plain config: bad dest: invalid port name": {
			failover: &pbcatalog.FailoverPolicy{
				Config: &pbcatalog.FailoverConfig{
					Destinations: []*pbcatalog.FailoverDestination{
						{Ref: newRef(ServiceType, "api-backup"), Port: "web"},
					},
				},
			},
			expectErr: `invalid "config" field: invalid element at index 0 of list "destinations": invalid "port" field: ports cannot be specified explicitly for the general failover section since it relies upon port alignment`,
		},
		// ported config
		"ported config: bad dest: any port name": {
			failover: &pbcatalog.FailoverPolicy{
				PortConfigs: map[string]*pbcatalog.FailoverConfig{
					"http": {
						Destinations: []*pbcatalog.FailoverDestination{
							{Ref: newRef(ServiceType, "api-backup"), Port: "$bad$"},
						},
					},
				},
			},
			expectErr: `invalid value of key "http" within port_configs: invalid element at index 0 of list "destinations": invalid "port" field: value must match regex: ^[a-z0-9]([a-z0-9\-_]*[a-z0-9])?$`,
		},

		// both
		"one": {
			failover: &pbcatalog.FailoverPolicy{
				PortConfigs: map[string]*pbcatalog.FailoverConfig{
					"http": {
						Destinations: []*pbcatalog.FailoverDestination{
							{
								Ref:  newRef(ServiceType, "api-backup"),
								Port: "www",
							},
							{
								Ref: newRef(ServiceType, "api-double-backup"),
							},
						},
					},
				},
			},
			expectErr: "",
		},
		"one-2": {
			failover: &pbcatalog.FailoverPolicy{
				PortConfigs: map[string]*pbcatalog.FailoverConfig{
					"http": {
						Destinations: []*pbcatalog.FailoverDestination{
							{
								Ref:  newRef(ServiceType, "api-backup"),
								Port: "www",
							},
							{
								Ref:  newRef(ServiceType, "api-double-backup"),
								Port: "http", // port defaulted
							},
						},
					},
				},
			},
			expectErr: "",
		},
		"two": {
			failover: &pbcatalog.FailoverPolicy{
				Config: &pbcatalog.FailoverConfig{
					Destinations: []*pbcatalog.FailoverDestination{
						{
							Ref: newRef(ServiceType, "api-backup"),
						},
						{
							Ref: newRef(ServiceType, "api-double-backup"),
						},
					},
				},
			},
			expectErr: "",
		},
		"two-2": {
			failover: &pbcatalog.FailoverPolicy{
				PortConfigs: map[string]*pbcatalog.FailoverConfig{
					"http": {
						Destinations: []*pbcatalog.FailoverDestination{
							{
								Ref:  newRef(ServiceType, "api-backup"),
								Port: "http",
							},
							{
								Ref:  newRef(ServiceType, "api-double-backup"),
								Port: "http",
							},
						},
					},
					"rest": {
						Destinations: []*pbcatalog.FailoverDestination{
							{
								Ref:  newRef(ServiceType, "api-backup"),
								Port: "rest",
							},
							{
								Ref:  newRef(ServiceType, "api-double-backup"),
								Port: "rest",
							},
						},
					},
				},
			},
			expectErr: "",
		},
		"three": {
			failover: &pbcatalog.FailoverPolicy{
				Config: &pbcatalog.FailoverConfig{
					Destinations: []*pbcatalog.FailoverDestination{
						{
							Ref: newRef(ServiceType, "api-backup"),
						},
						{
							Ref: newRef(ServiceType, "api-double-backup"),
						},
					},
				},
				PortConfigs: map[string]*pbcatalog.FailoverConfig{
					"rest": {
						Mode:          pbcatalog.FailoverMode_FAILOVER_MODE_ORDER_BY_LOCALITY,
						Regions:       []string{"us", "eu"},
						SamenessGroup: "sameweb",
					},
				},
			},
			expectErr: "",
		},
		"three-2": {
			failover: &pbcatalog.FailoverPolicy{
				PortConfigs: map[string]*pbcatalog.FailoverConfig{
					"http": {
						Destinations: []*pbcatalog.FailoverDestination{
							{
								Ref:  newRef(ServiceType, "api-backup"),
								Port: "http",
							},
							{
								Ref:  newRef(ServiceType, "api-double-backup"),
								Port: "http",
							},
						},
					},
					"rest": {
						Mode:          pbcatalog.FailoverMode_FAILOVER_MODE_ORDER_BY_LOCALITY,
						Regions:       []string{"us", "eu"},
						SamenessGroup: "sameweb",
					},
				},
			},
			expectErr: "",
		},
	}

	maybeWrap := func(wrapPrefix, base string) string {
		if base != "" {
			return wrapPrefix + base
		}
		return ""
	}

	for name, tc := range configCases {
		cases["XX: plain config: "+name] = testcase{
			failover: &pbcatalog.FailoverPolicy{
				Config: proto.Clone(tc.config).(*pbcatalog.FailoverConfig),
			},
			expectErr: maybeWrap(`invalid "config" field: `, tc.expectErr),
		}

		cases["XX: ported config: "+name] = testcase{
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

		svc := resourcetest.MustDecode[pbcatalog.Service, *pbcatalog.Service](t, tc.svc)
		failover := resourcetest.MustDecode[pbcatalog.FailoverPolicy, *pbcatalog.FailoverPolicy](t, tc.failover)
		expect := resourcetest.MustDecode[pbcatalog.FailoverPolicy, *pbcatalog.FailoverPolicy](t, tc.expect)

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
		"explicit with port aligned defaulting": {
			svc: resourcetest.Resource(ServiceType, "api").
				WithData(t, &pbcatalog.Service{
					Ports: []*pbcatalog.ServicePort{
						newPort("http", 8080, pbcatalog.Protocol_PROTOCOL_HTTP),
						newPort("rest", 8282, pbcatalog.Protocol_PROTOCOL_HTTP2),
					},
				}).
				Build(),
			failover: resourcetest.Resource(FailoverPolicyType, "api").
				WithData(t, &pbcatalog.FailoverPolicy{
					PortConfigs: map[string]*pbcatalog.FailoverConfig{
						"http": {
							Destinations: []*pbcatalog.FailoverDestination{
								{
									Ref:  newRef(ServiceType, "api-backup"),
									Port: "www",
								},
								{
									Ref: newRef(ServiceType, "api-double-backup"),
								},
							},
						},
					},
				}).
				Build(),
			expect: resourcetest.Resource(FailoverPolicyType, "api").
				WithData(t, &pbcatalog.FailoverPolicy{
					PortConfigs: map[string]*pbcatalog.FailoverConfig{
						"http": {
							Destinations: []*pbcatalog.FailoverDestination{
								{
									Ref:  newRef(ServiceType, "api-backup"),
									Port: "www",
								},
								{
									Ref:  newRef(ServiceType, "api-double-backup"),
									Port: "http", // port defaulted
								},
							},
						},
					},
				}).
				Build(),
		},
		"implicit port explosion": {
			svc: resourcetest.Resource(ServiceType, "api").
				WithData(t, &pbcatalog.Service{
					Ports: []*pbcatalog.ServicePort{
						newPort("http", 8080, pbcatalog.Protocol_PROTOCOL_HTTP),
						newPort("rest", 8282, pbcatalog.Protocol_PROTOCOL_HTTP2),
					},
				}).
				Build(),
			failover: resourcetest.Resource(FailoverPolicyType, "api").
				WithData(t, &pbcatalog.FailoverPolicy{
					Config: &pbcatalog.FailoverConfig{
						Destinations: []*pbcatalog.FailoverDestination{
							{
								Ref: newRef(ServiceType, "api-backup"),
							},
							{
								Ref: newRef(ServiceType, "api-double-backup"),
							},
						},
					},
				}).
				Build(),
			expect: resourcetest.Resource(FailoverPolicyType, "api").
				WithData(t, &pbcatalog.FailoverPolicy{
					PortConfigs: map[string]*pbcatalog.FailoverConfig{
						"http": {
							Destinations: []*pbcatalog.FailoverDestination{
								{
									Ref:  newRef(ServiceType, "api-backup"),
									Port: "http",
								},
								{
									Ref:  newRef(ServiceType, "api-double-backup"),
									Port: "http",
								},
							},
						},
						"rest": {
							Destinations: []*pbcatalog.FailoverDestination{
								{
									Ref:  newRef(ServiceType, "api-backup"),
									Port: "rest",
								},
								{
									Ref:  newRef(ServiceType, "api-double-backup"),
									Port: "rest",
								},
							},
						},
					},
				}).
				Build(),
		},
		"mixed port explosion with skip": {
			svc: resourcetest.Resource(ServiceType, "api").
				WithData(t, &pbcatalog.Service{
					Ports: []*pbcatalog.ServicePort{
						newPort("http", 8080, pbcatalog.Protocol_PROTOCOL_HTTP),
						newPort("rest", 8282, pbcatalog.Protocol_PROTOCOL_HTTP2),
					},
				}).
				Build(),
			failover: resourcetest.Resource(FailoverPolicyType, "api").
				WithData(t, &pbcatalog.FailoverPolicy{
					Config: &pbcatalog.FailoverConfig{
						Destinations: []*pbcatalog.FailoverDestination{
							{
								Ref: newRef(ServiceType, "api-backup"),
							},
							{
								Ref: newRef(ServiceType, "api-double-backup"),
							},
						},
					},
					PortConfigs: map[string]*pbcatalog.FailoverConfig{
						"rest": {
							Mode:          pbcatalog.FailoverMode_FAILOVER_MODE_ORDER_BY_LOCALITY,
							Regions:       []string{"us", "eu"},
							SamenessGroup: "sameweb",
						},
					},
				}).
				Build(),
			expect: resourcetest.Resource(FailoverPolicyType, "api").
				WithData(t, &pbcatalog.FailoverPolicy{
					PortConfigs: map[string]*pbcatalog.FailoverConfig{
						"http": {
							Destinations: []*pbcatalog.FailoverDestination{
								{
									Ref:  newRef(ServiceType, "api-backup"),
									Port: "http",
								},
								{
									Ref:  newRef(ServiceType, "api-double-backup"),
									Port: "http",
								},
							},
						},
						"rest": {
							Mode:          pbcatalog.FailoverMode_FAILOVER_MODE_ORDER_BY_LOCALITY,
							Regions:       []string{"us", "eu"},
							SamenessGroup: "sameweb",
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

func newRef(typ *pbresource.Type, name string) *pbresource.Reference {
	return resourcetest.Resource(typ, name).Reference("")
}

func newRefWithPeer(typ *pbresource.Type, name string, peer string) *pbresource.Reference {
	ref := newRef(typ, name)
	ref.Tenancy.PeerName = peer
	return ref
}
