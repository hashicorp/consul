package structs

import (
	"strings"
	"testing"

	"github.com/hashicorp/consul/acl"

	"github.com/stretchr/testify/require"
)

func TestStructs_ACLToken_PolicyIDs(t *testing.T) {

	t.Run("Basic", func(t *testing.T) {

		token := &ACLToken{
			Policies: []ACLTokenPolicyLink{
				{
					ID: "one",
				},
				{
					ID: "two",
				},
				{
					ID: "three",
				},
			},
		}

		policyIDs := token.PolicyIDs()
		require.Len(t, policyIDs, 3)
		require.Equal(t, "one", policyIDs[0])
		require.Equal(t, "two", policyIDs[1])
		require.Equal(t, "three", policyIDs[2])
	})

	t.Run("No Policies", func(t *testing.T) {

		token := &ACLToken{}

		policyIDs := token.PolicyIDs()
		require.Len(t, policyIDs, 0)
	})
}

func TestStructs_ACLServiceIdentity_SyntheticPolicy(t *testing.T) {

	cases := []struct {
		serviceName string
		datacenters []string
		expectRules string
	}{
		{"web", nil, aclServiceIdentityRules("web", nil)},
		{"companion-cube-99", []string{"dc1", "dc2"}, aclServiceIdentityRules("companion-cube-99", nil)},
	}

	for _, test := range cases {
		name := test.serviceName
		if len(test.datacenters) > 0 {
			name += " [" + strings.Join(test.datacenters, ", ") + "]"
		}
		t.Run(name, func(t *testing.T) {
			svcid := &ACLServiceIdentity{
				ServiceName: test.serviceName,
				Datacenters: test.datacenters,
			}

			expect := &ACLPolicy{
				Syntax:      acl.SyntaxCurrent,
				Datacenters: test.datacenters,
				Description: "synthetic policy",
				Rules:       test.expectRules,
			}

			got := svcid.SyntheticPolicy(nil)
			require.NotEmpty(t, got.ID)
			require.True(t, strings.HasPrefix(got.Name, "synthetic-policy-"))
			// strip irrelevant fields before equality
			got.ID = ""
			got.Name = ""
			got.Hash = nil
			require.Equal(t, expect, got)
		})
	}
}

func TestStructs_ACLToken_SetHash(t *testing.T) {

	token := ACLToken{
		AccessorID:  "09d1c059-961a-46bd-a2e4-76adebe35fa5",
		SecretID:    "65e98e67-9b29-470c-8ffa-7c5a23cc67c8",
		Description: "test",
		Policies: []ACLTokenPolicyLink{
			{
				ID: "one",
			},
			{
				ID: "two",
			},
			{
				ID: "three",
			},
		},
	}

	t.Run("Nil Hash - Generate", func(t *testing.T) {
		require.Nil(t, token.Hash)
		h := token.SetHash(false)
		require.NotNil(t, h)
		require.NotEqual(t, []byte{}, h)
		require.Equal(t, h, token.Hash)
	})

	t.Run("Hash Set - Dont Generate", func(t *testing.T) {
		original := token.Hash
		h := token.SetHash(false)
		require.Equal(t, original, h)

		token.Description = "changed"
		h = token.SetHash(false)
		require.Equal(t, original, h)
	})

	t.Run("Hash Set - Generate", func(t *testing.T) {
		original := token.Hash
		h := token.SetHash(true)
		require.NotEqual(t, original, h)
	})
}

func TestStructs_ACLToken_EstimateSize(t *testing.T) {

	// estimated size here should
	token := ACLToken{
		AccessorID:  "09d1c059-961a-46bd-a2e4-76adebe35fa5",
		SecretID:    "65e98e67-9b29-470c-8ffa-7c5a23cc67c8",
		Description: "test",
		Policies: []ACLTokenPolicyLink{
			{
				ID: "one",
			},
			{
				ID: "two",
			},
			{
				ID: "three",
			},
		},
	}

	// this test is very contrived. Basically just tests that the
	// math is okay and returns the value.
	require.Equal(t, 128, token.EstimateSize())
}

func TestStructs_ACLToken_Stub(t *testing.T) {
	t.Run("Basic", func(t *testing.T) {

		token := ACLToken{
			AccessorID:  "09d1c059-961a-46bd-a2e4-76adebe35fa5",
			SecretID:    "65e98e67-9b29-470c-8ffa-7c5a23cc67c8",
			Description: "test",
			Policies: []ACLTokenPolicyLink{
				{
					ID: "one",
				},
				{
					ID: "two",
				},
				{
					ID: "three",
				},
			},
		}

		stub := token.Stub()

		require.Equal(t, token.AccessorID, stub.AccessorID)
		require.Equal(t, token.SecretID, stub.SecretID)
		require.Equal(t, token.Description, stub.Description)
		require.Equal(t, token.Policies, stub.Policies)
		require.Equal(t, token.Local, stub.Local)
		require.Equal(t, token.CreateTime, stub.CreateTime)
		require.Equal(t, token.Hash, stub.Hash)
		require.Equal(t, token.CreateIndex, stub.CreateIndex)
		require.Equal(t, token.ModifyIndex, stub.ModifyIndex)
		require.False(t, stub.Legacy)
	})
}

func TestStructs_ACLTokens_Sort(t *testing.T) {

	tokens := ACLTokens{
		&ACLToken{
			AccessorID: "9db509a9-c809-48c1-895d-99f845b7a9d5",
		},
		&ACLToken{
			AccessorID: "6bd01084-1695-43b8-898d-b2dd7874754d",
		},
		&ACLToken{
			AccessorID: "614a4cef-9149-4271-b878-7edb1ad661f8",
		},
		&ACLToken{
			AccessorID: "c9dd9980-8d54-472f-9e5e-74c02143e1f4",
		},
	}

	tokens.Sort()
	require.Equal(t, tokens[0].AccessorID, "614a4cef-9149-4271-b878-7edb1ad661f8")
	require.Equal(t, tokens[1].AccessorID, "6bd01084-1695-43b8-898d-b2dd7874754d")
	require.Equal(t, tokens[2].AccessorID, "9db509a9-c809-48c1-895d-99f845b7a9d5")
	require.Equal(t, tokens[3].AccessorID, "c9dd9980-8d54-472f-9e5e-74c02143e1f4")
}

func TestStructs_ACLTokenListStubs_Sort(t *testing.T) {

	tokens := ACLTokenListStubs{
		&ACLTokenListStub{
			AccessorID: "9db509a9-c809-48c1-895d-99f845b7a9d5",
		},
		&ACLTokenListStub{
			AccessorID: "6bd01084-1695-43b8-898d-b2dd7874754d",
		},
		&ACLTokenListStub{
			AccessorID: "614a4cef-9149-4271-b878-7edb1ad661f8",
		},
		&ACLTokenListStub{
			AccessorID: "c9dd9980-8d54-472f-9e5e-74c02143e1f4",
		},
	}

	tokens.Sort()
	require.Equal(t, tokens[0].AccessorID, "614a4cef-9149-4271-b878-7edb1ad661f8")
	require.Equal(t, tokens[1].AccessorID, "6bd01084-1695-43b8-898d-b2dd7874754d")
	require.Equal(t, tokens[2].AccessorID, "9db509a9-c809-48c1-895d-99f845b7a9d5")
	require.Equal(t, tokens[3].AccessorID, "c9dd9980-8d54-472f-9e5e-74c02143e1f4")
}

func TestStructs_ACLPolicy_Stub(t *testing.T) {

	policy := &ACLPolicy{
		ID:          "09d1c059-961a-46bd-a2e4-76adebe35fa5",
		Name:        "test",
		Description: "test",
		Rules:       `acl = "read"`,
	}

	stub := policy.Stub()

	require.Equal(t, policy.ID, stub.ID)
	require.Equal(t, policy.Name, stub.Name)
	require.Equal(t, policy.Description, stub.Description)
	require.Equal(t, policy.Datacenters, stub.Datacenters)
	require.Equal(t, policy.Hash, stub.Hash)
	require.Equal(t, policy.CreateIndex, stub.CreateIndex)
	require.Equal(t, policy.ModifyIndex, stub.ModifyIndex)
}

func TestStructs_ACLPolicy_SetHash(t *testing.T) {

	policy := &ACLPolicy{
		ID:          "09d1c059-961a-46bd-a2e4-76adebe35fa5",
		Name:        "test",
		Description: "test",
		Rules:       `acl = "read"`,
	}

	t.Run("Nil Hash - Generate", func(t *testing.T) {
		require.Nil(t, policy.Hash)
		h := policy.SetHash(false)
		require.NotNil(t, h)
		require.NotEqual(t, []byte{}, h)
		require.Equal(t, h, policy.Hash)
	})

	t.Run("Hash Set - Dont Generate", func(t *testing.T) {
		original := policy.Hash
		h := policy.SetHash(false)
		require.Equal(t, original, h)

		policy.Description = "changed"
		h = policy.SetHash(false)
		require.Equal(t, original, h)
	})

	t.Run("Hash Set - Generate", func(t *testing.T) {
		original := policy.Hash
		h := policy.SetHash(true)
		require.NotEqual(t, original, h)
	})
}

func TestStructs_ACLPolicy_EstimateSize(t *testing.T) {

	policy := ACLPolicy{
		ID:          "09d1c059-961a-46bd-a2e4-76adebe35fa5",
		Name:        "test",
		Description: "test",
		Rules:       `acl = "read"`,
	}

	// this test is very contrived. Basically just tests that the
	// math is okay and returns the value.
	require.Equal(t, 84, policy.EstimateSize())
	policy.Datacenters = []string{"dc1", "dc2"}
	require.Equal(t, 90, policy.EstimateSize())
}

func TestStructs_ACLPolicies_Sort(t *testing.T) {

	policies := ACLPolicies{
		&ACLPolicy{
			ID: "9db509a9-c809-48c1-895d-99f845b7a9d5",
		},
		&ACLPolicy{
			ID: "6bd01084-1695-43b8-898d-b2dd7874754d",
		},
		&ACLPolicy{
			ID: "614a4cef-9149-4271-b878-7edb1ad661f8",
		},
		&ACLPolicy{
			ID: "c9dd9980-8d54-472f-9e5e-74c02143e1f4",
		},
	}

	policies.Sort()
	require.Equal(t, policies[0].ID, "614a4cef-9149-4271-b878-7edb1ad661f8")
	require.Equal(t, policies[1].ID, "6bd01084-1695-43b8-898d-b2dd7874754d")
	require.Equal(t, policies[2].ID, "9db509a9-c809-48c1-895d-99f845b7a9d5")
	require.Equal(t, policies[3].ID, "c9dd9980-8d54-472f-9e5e-74c02143e1f4")
}

func TestStructs_ACLPolicyListStubs_Sort(t *testing.T) {

	policies := ACLPolicyListStubs{
		&ACLPolicyListStub{
			ID: "9db509a9-c809-48c1-895d-99f845b7a9d5",
		},
		&ACLPolicyListStub{
			ID: "6bd01084-1695-43b8-898d-b2dd7874754d",
		},
		&ACLPolicyListStub{
			ID: "614a4cef-9149-4271-b878-7edb1ad661f8",
		},
		&ACLPolicyListStub{
			ID: "c9dd9980-8d54-472f-9e5e-74c02143e1f4",
		},
	}

	policies.Sort()
	require.Equal(t, policies[0].ID, "614a4cef-9149-4271-b878-7edb1ad661f8")
	require.Equal(t, policies[1].ID, "6bd01084-1695-43b8-898d-b2dd7874754d")
	require.Equal(t, policies[2].ID, "9db509a9-c809-48c1-895d-99f845b7a9d5")
	require.Equal(t, policies[3].ID, "c9dd9980-8d54-472f-9e5e-74c02143e1f4")
}
