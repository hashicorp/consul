// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"fmt"
	"testing"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/version/versiontest"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type computedFailoverTestcase struct {
	failover  *pbcatalog.ComputedFailoverPolicy
	expectErr string
}

func convertToComputedFailverPolicyTestCases(fpCases map[string]failoverTestcase) map[string]computedFailoverTestcase {
	cfpTestcases := map[string]computedFailoverTestcase{}
	for k, v := range fpCases {
		cfpTestcases[k] = computedFailoverTestcase{
			failover: &pbcatalog.ComputedFailoverPolicy{
				Config:      v.failover.Config,
				PortConfigs: v.failover.PortConfigs,
			},
			expectErr: v.expectErr,
		}
	}

	return cfpTestcases
}
func TestValidateComputedFailoverPolicy(t *testing.T) {
	run := func(t *testing.T, tc computedFailoverTestcase) {
		res := resourcetest.Resource(pbcatalog.ComputedFailoverPolicyType, "api").
			WithTenancy(resource.DefaultNamespacedTenancy()).
			WithData(t, tc.failover).
			Build()

		err := ValidateComputedFailoverPolicy(res)

		// Verify that validate didn't actually change the object.
		got := resourcetest.MustDecode[*pbcatalog.ComputedFailoverPolicy](t, res)
		prototest.AssertDeepEqual(t, tc.failover, got.Data)

		if tc.expectErr == "" {
			require.NoError(t, err)
		} else {
			testutil.RequireErrorContains(t, err, tc.expectErr)
		}
	}
	cases := convertToComputedFailverPolicyTestCases(getCommonTestCases())
	cases["plain config: sameness_group"] = computedFailoverTestcase{
		failover: &pbcatalog.ComputedFailoverPolicy{
			Config: &pbcatalog.FailoverConfig{
				SamenessGroup: "test",
			},
		},
		expectErr: `invalid "config" field: computed failover policy cannot have a sameness_group`,
	}
	cases["ported config: sameness_group"] = computedFailoverTestcase{
		failover: &pbcatalog.ComputedFailoverPolicy{
			PortConfigs: map[string]*pbcatalog.FailoverConfig{
				"http": {
					SamenessGroup: "test",
				},
			},
		},
		expectErr: `invalid "config" field: computed failover policy cannot have a sameness_group`,
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestComputedFailoverPolicyACLs(t *testing.T) {
	// Wire up a registry to generically invoke hooks
	testFailOverPolicyAcls(t, true)
}

func testFailOverPolicyAcls(t *testing.T, isComputedFailoverPolicy bool) {
	registry := resource.NewRegistry()
	Register(registry)

	newFailover := func(t *testing.T, name, tenancyStr string, destRefs []*pbresource.Reference) []*pbresource.Resource {
		var dr []*pbcatalog.FailoverDestination
		for _, destRef := range destRefs {
			dr = append(dr, &pbcatalog.FailoverDestination{Ref: destRef})
		}
		var (
			res1 *pbresource.Resource
			res2 *pbresource.Resource
		)
		cfgDests := &pbcatalog.FailoverConfig{Destinations: dr}
		portedCfgDests := map[string]*pbcatalog.FailoverConfig{
			"http": {Destinations: dr},
		}
		var cfgData, portedCfgData protoreflect.ProtoMessage
		var typ *pbresource.Type
		if isComputedFailoverPolicy {
			typ = pbcatalog.ComputedFailoverPolicyType
			cfgData = &pbcatalog.ComputedFailoverPolicy{
				Config: cfgDests,
			}
			portedCfgData = &pbcatalog.ComputedFailoverPolicy{
				PortConfigs: portedCfgDests,
			}
		} else {
			typ = pbcatalog.FailoverPolicyType
			cfgData = &pbcatalog.FailoverPolicy{
				Config: cfgDests,
			}
			portedCfgData = &pbcatalog.FailoverPolicy{
				PortConfigs: portedCfgDests,
			}
		}

		res1 = resourcetest.Resource(typ, name).
			WithTenancy(resourcetest.Tenancy(tenancyStr)).
			WithData(t, cfgData).
			Build()
		resourcetest.ValidateAndNormalize(t, registry, res1)

		res2 = resourcetest.Resource(typ, name).
			WithTenancy(resourcetest.Tenancy(tenancyStr)).
			WithData(t, portedCfgData).
			Build()
		resourcetest.ValidateAndNormalize(t, registry, res2)

		return []*pbresource.Resource{res1, res2}
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
