package structs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStructs_ACL_IsSame(t *testing.T) {
	acl := &ACL{
		ID:    "guid",
		Name:  "An ACL for testing",
		Type:  "client",
		Rules: "service \"\" { policy = \"read\" }",
	}
	if !acl.IsSame(acl) {
		t.Fatalf("should be equal to itself")
	}

	other := &ACL{
		ID:    "guid",
		Name:  "An ACL for testing",
		Type:  "client",
		Rules: "service \"\" { policy = \"read\" }",
		RaftIndex: RaftIndex{
			CreateIndex: 1,
			ModifyIndex: 2,
		},
	}
	if !acl.IsSame(other) || !other.IsSame(acl) {
		t.Fatalf("should not care about Raft fields")
	}

	check := func(twiddle, restore func()) {
		if !acl.IsSame(other) || !other.IsSame(acl) {
			t.Fatalf("should be the same")
		}

		twiddle()
		if acl.IsSame(other) || other.IsSame(acl) {
			t.Fatalf("should not be the same")
		}

		restore()
		if !acl.IsSame(other) || !other.IsSame(acl) {
			t.Fatalf("should be the same")
		}
	}

	check(func() { other.ID = "nope" }, func() { other.ID = "guid" })
	check(func() { other.Name = "nope" }, func() { other.Name = "An ACL for testing" })
	check(func() { other.Type = "management" }, func() { other.Type = "client" })
	check(func() { other.Rules = "" }, func() { other.Rules = "service \"\" { policy = \"read\" }" })
}

func TestStructs_ACL_Convert(t *testing.T) {
	t.Parallel()

	acl := &ACL{
		ID:    "guid",
		Name:  "AN ACL for testing",
		Type:  "client",
		Rules: `service "" { policy "read" }`,
	}

	token := acl.Convert()
	require.Equal(t, "", token.AccessorID)
	require.Equal(t, acl.ID, token.SecretID)
	require.Equal(t, acl.Type, token.Type)
	require.Equal(t, acl.Name, token.Description)
	require.Nil(t, token.Policies)
	require.False(t, token.Local)
	require.Equal(t, acl.Rules, token.Rules)
	require.Equal(t, acl.CreateIndex, token.CreateIndex)
	require.Equal(t, acl.ModifyIndex, token.ModifyIndex)
}

func TestStructs_ACLToken_Convert(t *testing.T) {
	t.Parallel()

	t.Run("Management", func(t *testing.T) {
		t.Parallel()
		token := &ACLToken{
			AccessorID:  "6c4eb178-c7f3-4620-b899-91eb8696c265",
			SecretID:    "67c29ecd-cabc-42e0-a20e-771e9a1ab70c",
			Description: "new token",
			Policies: []ACLTokenPolicyLink{
				ACLTokenPolicyLink{
					ID: ACLPolicyGlobalManagementID,
				},
			},
			Type: ACLTokenTypeManagement,
		}

		acl, err := token.Convert()
		require.NoError(t, err)
		require.Equal(t, token.SecretID, acl.ID)
		require.Equal(t, token.Type, acl.Type)
		require.Equal(t, token.Description, acl.Name)
		require.Equal(t, "", acl.Rules)
	})

	t.Run("Client", func(t *testing.T) {
		t.Parallel()
		token := &ACLToken{
			AccessorID:  "6c4eb178-c7f3-4620-b899-91eb8696c265",
			SecretID:    "67c29ecd-cabc-42e0-a20e-771e9a1ab70c",
			Description: "new token",
			Policies:    nil,
			Type:        ACLTokenTypeClient,
			Rules:       `acl = "read"`,
		}

		acl, err := token.Convert()
		require.NoError(t, err)
		require.Equal(t, token.SecretID, acl.ID)
		require.Equal(t, token.Type, acl.Type)
		require.Equal(t, token.Description, acl.Name)
		require.Equal(t, token.Rules, acl.Rules)
	})

	t.Run("Unconvertible", func(t *testing.T) {
		t.Parallel()
		token := &ACLToken{
			AccessorID:  "6c4eb178-c7f3-4620-b899-91eb8696c265",
			SecretID:    "67c29ecd-cabc-42e0-a20e-771e9a1ab70c",
			Description: "new token",
			Policies: []ACLTokenPolicyLink{
				ACLTokenPolicyLink{
					ID: ACLPolicyGlobalManagementID,
				},
			},
		}

		acl, err := token.Convert()
		require.Error(t, err)
		require.Nil(t, acl)
	})

}
