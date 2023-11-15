// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/version/versiontest"
)

func TestValidateDestinationPolicy(t *testing.T) {
	type testcase struct {
		policy     *pbmesh.DestinationPolicy
		expectErr  string
		expectErrs []string
	}

	run := func(t *testing.T, tc testcase) {
		res := resourcetest.Resource(pbmesh.DestinationPolicyType, "api").
			WithData(t, tc.policy).
			Build()

		err := ValidateDestinationPolicy(res)

		// Verify that validate didn't actually change the object.
		got := resourcetest.MustDecode[*pbmesh.DestinationPolicy](t, res)
		prototest.AssertDeepEqual(t, tc.policy, got.Data)

		if tc.expectErr != "" && len(tc.expectErrs) > 0 {
			t.Fatalf("cannot test singular and list errors at the same time")
		}

		if tc.expectErr == "" && len(tc.expectErrs) == 0 {
			require.NoError(t, err)
		} else if tc.expectErr != "" {
			testutil.RequireErrorContains(t, err, tc.expectErr)
		} else {
			for _, expectErr := range tc.expectErrs {
				testutil.RequireErrorContains(t, err, expectErr)
			}
		}
	}

	cases := map[string]testcase{
		// emptiness
		"empty": {
			policy:    &pbmesh.DestinationPolicy{},
			expectErr: `invalid "port_configs" field: cannot be empty`,
		},
		"good connect timeout": {
			policy: &pbmesh.DestinationPolicy{
				PortConfigs: map[string]*pbmesh.DestinationConfig{
					"http": {
						ConnectTimeout: durationpb.New(55 * time.Second),
					},
				},
			},
		},
		"bad connect timeout": {
			policy: &pbmesh.DestinationPolicy{
				PortConfigs: map[string]*pbmesh.DestinationConfig{
					"http": {
						ConnectTimeout: durationpb.New(-55 * time.Second),
					},
				},
			},
			expectErr: `invalid value of key "http" within port_configs: invalid "connect_timeout" field: '-55s', must be >= 0`,
		},
		"good request timeout": {
			policy: &pbmesh.DestinationPolicy{
				PortConfigs: map[string]*pbmesh.DestinationConfig{
					"http": {
						RequestTimeout: durationpb.New(55 * time.Second),
					},
				},
			},
		},
		"bad request timeout": {
			policy: &pbmesh.DestinationPolicy{
				PortConfigs: map[string]*pbmesh.DestinationConfig{
					"http": {
						RequestTimeout: durationpb.New(-55 * time.Second),
					},
				},
			},
			expectErr: `invalid value of key "http" within port_configs: invalid "request_timeout" field: '-55s', must be >= 0`,
		},
		// load balancer
		"lbpolicy: missing enum": {
			policy: &pbmesh.DestinationPolicy{
				PortConfigs: map[string]*pbmesh.DestinationConfig{
					"http": {
						ConnectTimeout: durationpb.New(55 * time.Second),
						LoadBalancer:   &pbmesh.LoadBalancer{},
					},
				},
			},
		},
		"lbpolicy: bad enum": {
			policy: &pbmesh.DestinationPolicy{
				PortConfigs: map[string]*pbmesh.DestinationConfig{
					"http": {
						ConnectTimeout: durationpb.New(55 * time.Second),
						LoadBalancer: &pbmesh.LoadBalancer{
							Policy: 99,
						},
					},
				},
			},
			expectErr: `invalid value of key "http" within port_configs: invalid "load_balancer" field: invalid "policy" field: not a supported enum value: 99`,
		},
		"lbpolicy: supported": {
			policy: &pbmesh.DestinationPolicy{
				PortConfigs: map[string]*pbmesh.DestinationConfig{
					"http": {
						ConnectTimeout: durationpb.New(55 * time.Second),
						LoadBalancer: &pbmesh.LoadBalancer{
							Policy: pbmesh.LoadBalancerPolicy_LOAD_BALANCER_POLICY_RANDOM,
						},
					},
				},
			},
		},
		"lbpolicy: bad for least request config": {
			policy: &pbmesh.DestinationPolicy{
				PortConfigs: map[string]*pbmesh.DestinationConfig{
					"http": {
						ConnectTimeout: durationpb.New(55 * time.Second),
						LoadBalancer: &pbmesh.LoadBalancer{
							Policy: pbmesh.LoadBalancerPolicy_LOAD_BALANCER_POLICY_RING_HASH,
							Config: &pbmesh.LoadBalancer_LeastRequestConfig{
								LeastRequestConfig: &pbmesh.LeastRequestConfig{
									ChoiceCount: 10,
								},
							},
						},
					},
				},
			},
			expectErr: `invalid value of key "http" within port_configs: invalid "load_balancer" field: invalid "config" field: least_request_config specified for incompatible load balancing policy "LOAD_BALANCER_POLICY_RING_HASH"`,
		},
		"lbpolicy: bad for ring hash config": {
			policy: &pbmesh.DestinationPolicy{
				PortConfigs: map[string]*pbmesh.DestinationConfig{
					"http": {
						ConnectTimeout: durationpb.New(55 * time.Second),
						LoadBalancer: &pbmesh.LoadBalancer{
							Policy: pbmesh.LoadBalancerPolicy_LOAD_BALANCER_POLICY_LEAST_REQUEST,
							Config: &pbmesh.LoadBalancer_RingHashConfig{
								RingHashConfig: &pbmesh.RingHashConfig{
									MinimumRingSize: 1024,
								},
							},
						},
					},
				},
			},
			expectErr: `invalid value of key "http" within port_configs: invalid "load_balancer" field: invalid "config" field: ring_hash_config specified for incompatible load balancing policy "LOAD_BALANCER_POLICY_LEAST_REQUEST"`,
		},
		"lbpolicy: good for least request config": {
			policy: &pbmesh.DestinationPolicy{
				PortConfigs: map[string]*pbmesh.DestinationConfig{
					"http": {
						ConnectTimeout: durationpb.New(55 * time.Second),
						LoadBalancer: &pbmesh.LoadBalancer{
							Policy: pbmesh.LoadBalancerPolicy_LOAD_BALANCER_POLICY_LEAST_REQUEST,
							Config: &pbmesh.LoadBalancer_LeastRequestConfig{
								LeastRequestConfig: &pbmesh.LeastRequestConfig{
									ChoiceCount: 10,
								},
							},
						},
					},
				},
			},
		},
		"lbpolicy: good for ring hash config": {
			policy: &pbmesh.DestinationPolicy{
				PortConfigs: map[string]*pbmesh.DestinationConfig{
					"http": {
						ConnectTimeout: durationpb.New(55 * time.Second),
						LoadBalancer: &pbmesh.LoadBalancer{
							Policy: pbmesh.LoadBalancerPolicy_LOAD_BALANCER_POLICY_RING_HASH,
							Config: &pbmesh.LoadBalancer_RingHashConfig{
								RingHashConfig: &pbmesh.RingHashConfig{
									MinimumRingSize: 1024,
								},
							},
						},
					},
				},
			},
		},
		"lbpolicy: empty policy with hash policy": {
			policy: &pbmesh.DestinationPolicy{
				PortConfigs: map[string]*pbmesh.DestinationConfig{
					"http": {
						ConnectTimeout: durationpb.New(55 * time.Second),
						LoadBalancer: &pbmesh.LoadBalancer{
							HashPolicies: []*pbmesh.HashPolicy{
								{SourceIp: true},
							},
						},
					},
				},
			},
			expectErr: `invalid value of key "http" within port_configs: invalid "load_balancer" field: invalid "hash_policies" field: hash_policies specified for non-hash-based policy "LOAD_BALANCER_POLICY_UNSPECIFIED"`,
		},
		"lbconfig: cookie config with header policy": {
			policy: &pbmesh.DestinationPolicy{
				PortConfigs: map[string]*pbmesh.DestinationConfig{
					"http": {
						ConnectTimeout: durationpb.New(55 * time.Second),
						LoadBalancer: &pbmesh.LoadBalancer{
							Policy: pbmesh.LoadBalancerPolicy_LOAD_BALANCER_POLICY_MAGLEV,
							HashPolicies: []*pbmesh.HashPolicy{
								{
									Field:      pbmesh.HashPolicyField_HASH_POLICY_FIELD_HEADER,
									FieldValue: "x-user-id",
									CookieConfig: &pbmesh.CookieConfig{
										Ttl:  durationpb.New(10 * time.Second),
										Path: "/root",
									},
								},
							},
						},
					},
				},
			},
			expectErr: `invalid value of key "http" within port_configs: invalid "load_balancer" field: invalid element at index 0 of list "hash_policies": invalid "cookie_config" field: incompatible with field "HASH_POLICY_FIELD_HEADER"`,
		},
		"lbconfig: cannot generate session cookie with ttl": {
			policy: &pbmesh.DestinationPolicy{
				PortConfigs: map[string]*pbmesh.DestinationConfig{
					"http": {
						ConnectTimeout: durationpb.New(55 * time.Second),
						LoadBalancer: &pbmesh.LoadBalancer{
							Policy: pbmesh.LoadBalancerPolicy_LOAD_BALANCER_POLICY_MAGLEV,
							HashPolicies: []*pbmesh.HashPolicy{
								{
									Field:      pbmesh.HashPolicyField_HASH_POLICY_FIELD_COOKIE,
									FieldValue: "good-cookie",
									CookieConfig: &pbmesh.CookieConfig{
										Session: true,
										Ttl:     durationpb.New(10 * time.Second),
									},
								},
							},
						},
					},
				},
			},
			expectErr: `invalid value of key "http" within port_configs: invalid "load_balancer" field: invalid element at index 0 of list "hash_policies": invalid "cookie_config" field: invalid "ttl" field: a session cookie cannot have an associated TTL`,
		},
		"lbconfig: valid cookie policy": {
			policy: &pbmesh.DestinationPolicy{
				PortConfigs: map[string]*pbmesh.DestinationConfig{
					"http": {
						ConnectTimeout: durationpb.New(55 * time.Second),
						LoadBalancer: &pbmesh.LoadBalancer{
							Policy: pbmesh.LoadBalancerPolicy_LOAD_BALANCER_POLICY_MAGLEV,
							HashPolicies: []*pbmesh.HashPolicy{
								{
									Field:      pbmesh.HashPolicyField_HASH_POLICY_FIELD_COOKIE,
									FieldValue: "good-cookie",
									CookieConfig: &pbmesh.CookieConfig{
										Ttl:  durationpb.New(10 * time.Second),
										Path: "/oven",
									},
								},
							},
						},
					},
				},
			},
		},
		"lbconfig: bad match field": {
			policy: &pbmesh.DestinationPolicy{
				PortConfigs: map[string]*pbmesh.DestinationConfig{
					"http": {
						ConnectTimeout: durationpb.New(55 * time.Second),
						LoadBalancer: &pbmesh.LoadBalancer{
							Policy: pbmesh.LoadBalancerPolicy_LOAD_BALANCER_POLICY_MAGLEV,
							HashPolicies: []*pbmesh.HashPolicy{
								{
									Field:      99,
									FieldValue: "X-Consul-Token",
								},
							},
						},
					},
				},
			},
			expectErr: `invalid value of key "http" within port_configs: invalid "load_balancer" field: invalid element at index 0 of list "hash_policies": invalid "field" field: not a supported enum value: 99`,
		},
		"lbconfig: supported match field": {
			policy: &pbmesh.DestinationPolicy{
				PortConfigs: map[string]*pbmesh.DestinationConfig{
					"http": {
						ConnectTimeout: durationpb.New(55 * time.Second),
						LoadBalancer: &pbmesh.LoadBalancer{
							Policy: pbmesh.LoadBalancerPolicy_LOAD_BALANCER_POLICY_MAGLEV,
							HashPolicies: []*pbmesh.HashPolicy{
								{
									Field:      pbmesh.HashPolicyField_HASH_POLICY_FIELD_HEADER,
									FieldValue: "X-Consul-Token",
								},
							},
						},
					},
				},
			},
		},
		"lbconfig: missing match field": {
			policy: &pbmesh.DestinationPolicy{
				PortConfigs: map[string]*pbmesh.DestinationConfig{
					"http": {
						ConnectTimeout: durationpb.New(55 * time.Second),
						LoadBalancer: &pbmesh.LoadBalancer{
							Policy: pbmesh.LoadBalancerPolicy_LOAD_BALANCER_POLICY_MAGLEV,
							HashPolicies: []*pbmesh.HashPolicy{
								{
									FieldValue: "X-Consul-Token",
								},
							},
						},
					},
				},
			},
			expectErrs: []string{
				`invalid value of key "http" within port_configs: invalid "load_balancer" field: invalid element at index 0 of list "hash_policies": invalid "field" field: missing required field`,
				`invalid value of key "http" within port_configs: invalid "load_balancer" field: invalid element at index 0 of list "hash_policies": invalid "field_value" field: requires a field to apply to`,
			},
		},
		"lbconfig: cannot match on source address and custom field": {
			policy: &pbmesh.DestinationPolicy{
				PortConfigs: map[string]*pbmesh.DestinationConfig{
					"http": {
						ConnectTimeout: durationpb.New(55 * time.Second),
						LoadBalancer: &pbmesh.LoadBalancer{
							Policy: pbmesh.LoadBalancerPolicy_LOAD_BALANCER_POLICY_MAGLEV,
							HashPolicies: []*pbmesh.HashPolicy{
								{
									Field:    pbmesh.HashPolicyField_HASH_POLICY_FIELD_HEADER,
									SourceIp: true,
								},
							},
						},
					},
				},
			},
			expectErrs: []string{
				`invalid value of key "http" within port_configs: invalid "load_balancer" field: invalid element at index 0 of list "hash_policies": invalid "field" field: a single hash policy cannot hash both a source address and a "HASH_POLICY_FIELD_HEADER"`,
				`invalid value of key "http" within port_configs: invalid "load_balancer" field: invalid element at index 0 of list "hash_policies": invalid "field_value" field: field "HASH_POLICY_FIELD_HEADER" was specified without a field_value`,
			},
		},
		"lbconfig: matchvalue not compatible with source address": {
			policy: &pbmesh.DestinationPolicy{
				PortConfigs: map[string]*pbmesh.DestinationConfig{
					"http": {
						ConnectTimeout: durationpb.New(55 * time.Second),
						LoadBalancer: &pbmesh.LoadBalancer{
							Policy: pbmesh.LoadBalancerPolicy_LOAD_BALANCER_POLICY_MAGLEV,
							HashPolicies: []*pbmesh.HashPolicy{
								{
									FieldValue: "X-Consul-Token",
									SourceIp:   true,
								},
							},
						},
					},
				},
			},
			expectErrs: []string{
				`invalid value of key "http" within port_configs: invalid "load_balancer" field: invalid element at index 0 of list "hash_policies": invalid "field_value" field: cannot be specified when hashing source_ip`,
				`invalid value of key "http" within port_configs: invalid "load_balancer" field: invalid element at index 0 of list "hash_policies": invalid "field_value" field: requires a field to apply to`,
			},
		},
		"lbconfig: field without match value": {
			policy: &pbmesh.DestinationPolicy{
				PortConfigs: map[string]*pbmesh.DestinationConfig{
					"http": {
						ConnectTimeout: durationpb.New(55 * time.Second),
						LoadBalancer: &pbmesh.LoadBalancer{
							Policy: pbmesh.LoadBalancerPolicy_LOAD_BALANCER_POLICY_MAGLEV,
							HashPolicies: []*pbmesh.HashPolicy{
								{
									Field: pbmesh.HashPolicyField_HASH_POLICY_FIELD_HEADER,
								},
							},
						},
					},
				},
			},
			expectErr: `invalid value of key "http" within port_configs: invalid "load_balancer" field: invalid element at index 0 of list "hash_policies": invalid "field_value" field: field "HASH_POLICY_FIELD_HEADER" was specified without a field_value`,
		},
		"lbconfig: matchvalue without field": {
			policy: &pbmesh.DestinationPolicy{
				PortConfigs: map[string]*pbmesh.DestinationConfig{
					"http": {
						ConnectTimeout: durationpb.New(55 * time.Second),
						LoadBalancer: &pbmesh.LoadBalancer{
							Policy: pbmesh.LoadBalancerPolicy_LOAD_BALANCER_POLICY_MAGLEV,
							HashPolicies: []*pbmesh.HashPolicy{
								{
									FieldValue: "my-cookie",
								},
							},
						},
					},
				},
			},
			expectErr: `invalid value of key "http" within port_configs: invalid "load_balancer" field: invalid element at index 0 of list "hash_policies": invalid "field_value" field: requires a field to apply to`,
		},
		"lbconfig: ring hash kitchen sink": {
			policy: &pbmesh.DestinationPolicy{
				PortConfigs: map[string]*pbmesh.DestinationConfig{
					"http": {
						ConnectTimeout: durationpb.New(55 * time.Second),
						LoadBalancer: &pbmesh.LoadBalancer{
							Policy: pbmesh.LoadBalancerPolicy_LOAD_BALANCER_POLICY_RING_HASH,
							Config: &pbmesh.LoadBalancer_RingHashConfig{
								RingHashConfig: &pbmesh.RingHashConfig{
									MaximumRingSize: 10,
									MinimumRingSize: 2,
								},
							},
							HashPolicies: []*pbmesh.HashPolicy{
								{
									Field:      pbmesh.HashPolicyField_HASH_POLICY_FIELD_COOKIE,
									FieldValue: "my-cookie",
								},
								{
									Field:      pbmesh.HashPolicyField_HASH_POLICY_FIELD_HEADER,
									FieldValue: "alt-header",
									Terminal:   true,
								},
							},
						},
					},
				},
			},
		},
		"lbconfig: least request kitchen sink": {
			policy: &pbmesh.DestinationPolicy{
				PortConfigs: map[string]*pbmesh.DestinationConfig{
					"http": {
						ConnectTimeout: durationpb.New(55 * time.Second),
						LoadBalancer: &pbmesh.LoadBalancer{
							Policy: pbmesh.LoadBalancerPolicy_LOAD_BALANCER_POLICY_LEAST_REQUEST,
							Config: &pbmesh.LoadBalancer_LeastRequestConfig{
								LeastRequestConfig: &pbmesh.LeastRequestConfig{
									ChoiceCount: 10,
								},
							},
						},
					},
				},
			},
		},
		"locality: good mode": {
			policy: &pbmesh.DestinationPolicy{
				PortConfigs: map[string]*pbmesh.DestinationConfig{
					"http": {
						LocalityPrioritization: &pbmesh.LocalityPrioritization{
							Mode: pbmesh.LocalityPrioritizationMode_LOCALITY_PRIORITIZATION_MODE_FAILOVER,
						},
					},
				},
			},
		},
		"locality: unset mode": {
			policy: &pbmesh.DestinationPolicy{
				PortConfigs: map[string]*pbmesh.DestinationConfig{
					"http": {
						LocalityPrioritization: &pbmesh.LocalityPrioritization{
							Mode: pbmesh.LocalityPrioritizationMode_LOCALITY_PRIORITIZATION_MODE_UNSPECIFIED,
						},
					},
				},
			},
		},
		"locality: bad mode": {
			policy: &pbmesh.DestinationPolicy{
				PortConfigs: map[string]*pbmesh.DestinationConfig{
					"http": {
						LocalityPrioritization: &pbmesh.LocalityPrioritization{
							Mode: 99,
						},
					},
				},
			},
			expectErr: `invalid value of key "http" within port_configs: invalid "locality_prioritization" field: invalid "mode" field: not a supported enum value: 99`,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestDestinationPolicyACLs(t *testing.T) {
	// Wire up a registry to generically invoke hooks
	registry := resource.NewRegistry()
	Register(registry)

	newPolicy := func(t *testing.T, tenancyStr string) *pbresource.Resource {
		res := resourcetest.Resource(pbmesh.DestinationPolicyType, "api").
			WithTenancy(resourcetest.Tenancy(tenancyStr)).
			WithData(t, &pbmesh.DestinationPolicy{
				PortConfigs: map[string]*pbmesh.DestinationConfig{
					"http": {
						ConnectTimeout: durationpb.New(55 * time.Second),
					},
				},
			}).
			Build()
		resourcetest.ValidateAndNormalize(t, registry, res)
		return res
	}

	const (
		DENY    = resourcetest.DENY
		ALLOW   = resourcetest.ALLOW
		DEFAULT = resourcetest.DEFAULT
	)

	run := func(t *testing.T, name string, tc resourcetest.ACLTestCase) {
		t.Run(name, func(t *testing.T) {
			resourcetest.RunACLTestCase(t, tc, registry)
		})
	}

	isEnterprise := versiontest.IsEnterprise()

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

	assert := func(t *testing.T, name string, rules string, res *pbresource.Resource, readOK, writeOK string) {
		tc := resourcetest.ACLTestCase{
			AuthCtx: resource.AuthorizerContext(res.Id.Tenancy),
			Rules:   rules,
			Res:     res,
			ReadOK:  readOK,
			WriteOK: writeOK,
			ListOK:  DEFAULT,
		}
		run(t, name, tc)
	}

	tenancies := []string{"default.default"}
	if isEnterprise {
		tenancies = append(tenancies, "default.foo", "alpha.default", "alpha.foo")
	}

	for _, policyTenancyStr := range tenancies {
		t.Run("policy tenancy: "+policyTenancyStr, func(t *testing.T) {
			for _, aclTenancyStr := range tenancies {
				t.Run("acl tenancy: "+aclTenancyStr, func(t *testing.T) {
					aclTenancy := resourcetest.Tenancy(aclTenancyStr)

					maybe := func(match string) string {
						if policyTenancyStr != aclTenancyStr {
							return DENY
						}
						return match
					}

					t.Run("no rules", func(t *testing.T) {
						rules := ``
						assert(t, "any", rules, newPolicy(t, policyTenancyStr), DENY, DENY)
					})
					t.Run("api:read", func(t *testing.T) {
						rules := serviceRead(aclTenancy.Partition, aclTenancy.Namespace, "api")
						assert(t, "any", rules, newPolicy(t, policyTenancyStr), maybe(ALLOW), DENY)
					})
					t.Run("api:write", func(t *testing.T) {
						rules := serviceWrite(aclTenancy.Partition, aclTenancy.Namespace, "api")
						assert(t, "any", rules, newPolicy(t, policyTenancyStr), maybe(ALLOW), maybe(ALLOW))
					})
				})
			}
		})
	}
}
