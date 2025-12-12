// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/require"
)

func TestDeduplicate(t *testing.T) {
	type testCase struct {
		templatedPolicies ACLTemplatedPolicies
		expectedCount     int
	}
	tcases := map[string]testCase{
		"multiple-of-the-same-template": {
			templatedPolicies: ACLTemplatedPolicies{
				&ACLTemplatedPolicy{
					TemplateName: api.ACLTemplatedPolicyServiceName,
					TemplateVariables: &ACLTemplatedPolicyVariables{
						Name: "api",
					},
				},
				&ACLTemplatedPolicy{
					TemplateName: api.ACLTemplatedPolicyServiceName,
					TemplateVariables: &ACLTemplatedPolicyVariables{
						Name: "api",
					},
				},
			},
			expectedCount: 1,
		},
		"separate-templates-with-matching-variables": {
			templatedPolicies: ACLTemplatedPolicies{
				&ACLTemplatedPolicy{
					TemplateName: api.ACLTemplatedPolicyNodeName,
					TemplateVariables: &ACLTemplatedPolicyVariables{
						Name: "api",
					},
				},
				&ACLTemplatedPolicy{
					TemplateName: api.ACLTemplatedPolicyServiceName,
					TemplateVariables: &ACLTemplatedPolicyVariables{
						Name: "api",
					},
				},
			},
			expectedCount: 2,
		},
		"separate-templates-with-multiple-matching-variables": {
			templatedPolicies: ACLTemplatedPolicies{
				&ACLTemplatedPolicy{
					TemplateName: api.ACLTemplatedPolicyServiceName,
					TemplateVariables: &ACLTemplatedPolicyVariables{
						Name: "api",
					},
				},
				&ACLTemplatedPolicy{
					TemplateName: api.ACLTemplatedPolicyNodeName,
					TemplateVariables: &ACLTemplatedPolicyVariables{
						Name: "api",
					},
				},
				&ACLTemplatedPolicy{
					TemplateName: api.ACLTemplatedPolicyNodeName,
					TemplateVariables: &ACLTemplatedPolicyVariables{
						Name: "web",
					},
				},
				&ACLTemplatedPolicy{
					TemplateName: api.ACLTemplatedPolicyServiceName,
					TemplateVariables: &ACLTemplatedPolicyVariables{
						Name: "api",
					},
				},
				&ACLTemplatedPolicy{
					TemplateName: api.ACLTemplatedPolicyDNSName,
				},
				&ACLTemplatedPolicy{
					TemplateName: api.ACLTemplatedPolicyServiceName,
					TemplateVariables: &ACLTemplatedPolicyVariables{
						Name: "web",
					},
				},
				&ACLTemplatedPolicy{
					TemplateName: api.ACLTemplatedPolicyDNSName,
				},
			},
			expectedCount: 5,
		},
	}

	for name, tcase := range tcases {
		t.Run(name, func(t *testing.T) {
			policies := tcase.templatedPolicies.Deduplicate()

			require.Equal(t, tcase.expectedCount, len(policies))
		})
	}
}
