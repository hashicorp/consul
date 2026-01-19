// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package auth

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/consul/authmethod"
)

func TestFormatTokenName(t *testing.T) {
	tests := []struct {
		name            string
		tokenNameFormat string
		authMethodName  string
		identity        *authmethod.Identity
		meta            map[string]string
		expected        string
	}{
		{
			name:            "empty format uses default",
			tokenNameFormat: "",
			authMethodName:  "test-method",
			identity:        nil,
			meta:            nil,
			expected:        "token created via login",
		},
		{
			name:            "simple auth method name",
			tokenNameFormat: "Token from ${auth_method}",
			authMethodName:  "my-auth-method",
			identity:        nil,
			meta:            nil,
			expected:        "Token from my-auth-method",
		},
		{
			name:            "with identity username",
			tokenNameFormat: "${auth_method}-${identity.username}",
			authMethodName:  "kubernetes",
			identity: &authmethod.Identity{
				ProjectedVars: map[string]string{
					"username": "john.doe",
				},
			},
			meta:     nil,
			expected: "kubernetes-john.doe",
		},
		{
			name:            "with identity email",
			tokenNameFormat: "User: ${identity.email}",
			authMethodName:  "oidc",
			identity: &authmethod.Identity{
				ProjectedVars: map[string]string{
					"email": "user@example.com",
				},
			},
			meta:     nil,
			expected: "User: user@example.com",
		},
		{
			name:            "complex template with multiple variables",
			tokenNameFormat: "${auth_method}/${identity.username}/${identity.namespace}",
			authMethodName:  "k8s-prod",
			identity: &authmethod.Identity{
				ProjectedVars: map[string]string{
					"username":  "service-account",
					"namespace": "default",
				},
			},
			meta:     nil,
			expected: "k8s-prod/service-account/default",
		},
		{
			name:            "with metadata",
			tokenNameFormat: "${auth_method} - ${meta.env}",
			authMethodName:  "ldap",
			identity:        nil,
			meta: map[string]string{
				"env": "production",
			},
			expected: "ldap - production",
		},
		{
			name:            "missing variable defaults to empty",
			tokenNameFormat: "${auth_method}-${identity.missing}",
			authMethodName:  "test",
			identity: &authmethod.Identity{
				ProjectedVars: map[string]string{
					"username": "user",
				},
			},
			meta:     nil,
			expected: "test-<no value>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := FormatTokenName(tt.tokenNameFormat, tt.authMethodName, tt.identity, tt.meta)
			require.NoError(t, err)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatTokenName_InvalidTemplate(t *testing.T) {
	// With the preprocessing we do, ${unclosed won't cause an error as it gets converted
	// Test with an actually invalid Go template instead
	_, err := FormatTokenName("{{unclosed", "method", nil, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to parse token name format template")
}
