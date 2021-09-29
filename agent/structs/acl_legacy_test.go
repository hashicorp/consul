package structs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStructs_ACL_Convert(t *testing.T) {

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
	require.NotEmpty(t, token.Hash)
}
