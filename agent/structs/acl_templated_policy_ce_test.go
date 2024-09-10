// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package structs

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
)

func TestStructs_ACLTemplatedPolicy_SyntheticPolicy(t *testing.T) {
	type testCase struct {
		templatedPolicy *ACLTemplatedPolicy
		expectedPolicy  *ACLPolicy
	}

	testCases := map[string]testCase{
		"service-identity-template": {
			templatedPolicy: &ACLTemplatedPolicy{
				TemplateID:   ACLTemplatedPolicyServiceID,
				TemplateName: api.ACLTemplatedPolicyServiceName,
				TemplateVariables: &ACLTemplatedPolicyVariables{
					Name: "api",
				},
			},
			expectedPolicy: &ACLPolicy{
				Description: "synthetic policy generated from templated policy: builtin/service",
				Rules: `
service "api" {
	policy = "write"
}
service "api-sidecar-proxy" {
	policy = "write"
}
service_prefix "" {
	policy = "read"
}
node_prefix "" {
	policy = "read"
}`,
			},
		},
		"node-identity-template": {
			templatedPolicy: &ACLTemplatedPolicy{
				TemplateID:   ACLTemplatedPolicyNodeID,
				TemplateName: api.ACLTemplatedPolicyNodeName,
				TemplateVariables: &ACLTemplatedPolicyVariables{
					Name: "web",
				},
			},
			expectedPolicy: &ACLPolicy{
				Description: "synthetic policy generated from templated policy: builtin/node",
				Rules: `
node "web" {
	policy = "write"
}
service_prefix "" {
	policy = "read"
}`,
			},
		},
		"dns-template": {
			templatedPolicy: &ACLTemplatedPolicy{
				TemplateID:   ACLTemplatedPolicyDNSID,
				TemplateName: api.ACLTemplatedPolicyDNSName,
			},
			expectedPolicy: &ACLPolicy{
				Description: "synthetic policy generated from templated policy: builtin/dns",
				Rules: `
node_prefix "" {
	policy = "read"
}
service_prefix "" {
	policy = "read"
}
query_prefix "" {
	policy = "read"
}`,
			},
		},
		"api-gateway-template": {
			templatedPolicy: &ACLTemplatedPolicy{
				TemplateID:   ACLTemplatedPolicyAPIGatewayID,
				TemplateName: api.ACLTemplatedPolicyAPIGatewayName,
				TemplateVariables: &ACLTemplatedPolicyVariables{
					Name: "api-gateway",
				},
			},
			expectedPolicy: &ACLPolicy{
				Description: "synthetic policy generated from templated policy: builtin/api-gateway",
				Rules: `mesh = "read"
node_prefix "" {
	policy = "read"
}
service_prefix "" {
	policy = "read"
}
service "api-gateway" {
	policy = "write"
}`,
			},
		},
	}

	for name, tcase := range testCases {
		t.Run(name, func(t *testing.T) {
			policy, err := tcase.templatedPolicy.SyntheticPolicy(nil)

			require.NoError(t, err)
			require.Equal(t, tcase.expectedPolicy.Description, policy.Description)
			require.Equal(t, tcase.expectedPolicy.Rules, policy.Rules)
			require.Contains(t, policy.Name, "synthetic-policy-")
			require.NotEmpty(t, policy.Hash)
			require.NotEmpty(t, policy.ID)
		})
	}
}
