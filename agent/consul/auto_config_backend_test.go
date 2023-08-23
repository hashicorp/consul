package consul

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/structs"
)

func TestAutoConfigBackend_DatacenterJoinAddresses(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	conf := testClusterConfig{
		Datacenter: "primary",
		Servers:    3,
	}

	nodes := newTestCluster(t, &conf)

	var expected []string
	for _, srv := range nodes.Servers {
		expected = append(expected, fmt.Sprintf("127.0.0.1:%d", srv.config.SerfLANConfig.MemberlistConfig.BindPort))
	}

	backend := autoConfigBackend{Server: nodes.Servers[0]}
	actual, err := backend.DatacenterJoinAddresses("", "")
	require.NoError(t, err)
	require.ElementsMatch(t, expected, actual)
}

func TestAutoConfigBackend_CreateACLToken(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	_, srv, codec := testACLServerWithConfig(t, nil, false)

	waitForLeaderEstablishment(t, srv)

	r1, err := upsertTestRole(codec, TestDefaultInitialManagementToken, "dc1")
	require.NoError(t, err)

	t.Run("predefined-ids", func(t *testing.T) {
		accessor := "554cd3ab-5d4e-4d6e-952e-4e8b6c77bfb3"
		secret := "ef453f31-ad58-4ec8-8bf8-342e99763026"
		in := &structs.ACLToken{
			AccessorID:  accessor,
			SecretID:    secret,
			Description: "test",
			Policies: []structs.ACLTokenPolicyLink{
				{
					ID: structs.ACLPolicyGlobalManagementID,
				},
			},
			NodeIdentities: []*structs.ACLNodeIdentity{
				{
					NodeName:   "foo",
					Datacenter: "bar",
				},
			},
			ServiceIdentities: []*structs.ACLServiceIdentity{
				{
					ServiceName: "web",
				},
			},
			Roles: []structs.ACLTokenRoleLink{
				{
					ID: r1.ID,
				},
			},
		}

		b := autoConfigBackend{Server: srv}
		out, err := b.CreateACLToken(in)
		require.NoError(t, err)
		require.Equal(t, accessor, out.AccessorID)
		require.Equal(t, secret, out.SecretID)
		require.Equal(t, "test", out.Description)
		require.NotZero(t, out.CreateTime)
		require.Len(t, out.Policies, 1)
		require.Len(t, out.Roles, 1)
		require.Len(t, out.NodeIdentities, 1)
		require.Len(t, out.ServiceIdentities, 1)
		require.Equal(t, structs.ACLPolicyGlobalManagementID, out.Policies[0].ID)
		require.Equal(t, "foo", out.NodeIdentities[0].NodeName)
		require.Equal(t, "web", out.ServiceIdentities[0].ServiceName)
		require.Equal(t, r1.ID, out.Roles[0].ID)
	})

	t.Run("autogen-ids", func(t *testing.T) {
		in := &structs.ACLToken{
			Description: "test",
			NodeIdentities: []*structs.ACLNodeIdentity{
				{
					NodeName:   "foo",
					Datacenter: "bar",
				},
			},
		}

		b := autoConfigBackend{Server: srv}
		out, err := b.CreateACLToken(in)
		require.NoError(t, err)
		require.NotEmpty(t, out.AccessorID)
		require.NotEmpty(t, out.SecretID)
		require.Equal(t, "test", out.Description)
		require.NotZero(t, out.CreateTime)
		require.Len(t, out.NodeIdentities, 1)
		require.Equal(t, "foo", out.NodeIdentities[0].NodeName)
	})
}
