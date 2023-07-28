package consul

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	uuid "github.com/hashicorp/go-uuid"
	"github.com/stretchr/testify/require"
	"gopkg.in/square/go-jose.v2/jwt"

	msgpackrpc "github.com/hashicorp/consul-net-rpc/net-rpc-msgpackrpc"
	"github.com/hashicorp/consul-net-rpc/net/rpc"
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/authmethod/kubeauth"
	"github.com/hashicorp/consul/agent/consul/authmethod/testauth"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/structs/aclfilter"
	"github.com/hashicorp/consul/internal/go-sso/oidcauth/oidcauthtest"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

func TestACLEndpoint_BootstrapTokens(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir, srv, codec := testACLServerWithConfig(t, func(c *Config) {
		// remove this as we are bootstrapping
		c.ACLInitialManagementToken = ""
	}, false)
	waitForLeaderEstablishment(t, srv)

	// Expect an error initially since ACL bootstrap is not initialized.
	arg := structs.DCSpecificRequest{
		Datacenter: "dc1",
	}
	var out structs.ACLToken
	// We can only do some high
	// level checks on the ACL since we don't have control over the UUID or
	// Raft indexes at this level.
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "ACL.BootstrapTokens", &arg, &out))
	require.Equal(t, 36, len(out.AccessorID))
	require.True(t, strings.HasPrefix(out.Description, "Bootstrap Token"))
	require.True(t, out.CreateIndex > 0)
	require.Equal(t, out.CreateIndex, out.ModifyIndex)

	// Finally, make sure that another attempt is rejected.
	err := msgpackrpc.CallWithCodec(codec, "ACL.BootstrapTokens", &arg, &out)
	require.Error(t, err)
	require.True(t, strings.HasPrefix(err.Error(), structs.ACLBootstrapNotAllowedErr.Error()))

	_, resetIdx, err := srv.fsm.State().CanBootstrapACLToken()
	require.NoError(t, err)

	resetPath := filepath.Join(dir, "acl-bootstrap-reset")
	require.NoError(t, ioutil.WriteFile(resetPath, []byte(fmt.Sprintf("%d", resetIdx)), 0600))

	oldID := out.AccessorID
	// Finally, make sure that another attempt is rejected.
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "ACL.BootstrapTokens", &arg, &out))
	require.Equal(t, 36, len(out.AccessorID))
	require.NotEqual(t, oldID, out.AccessorID)
	require.True(t, strings.HasPrefix(out.Description, "Bootstrap Token"))
	require.True(t, out.CreateIndex > 0)
	require.Equal(t, out.CreateIndex, out.ModifyIndex)
}

func TestACLEndpoint_ReplicationStatus(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	_, srv, codec := testACLServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc2"
		c.ACLTokenReplication = true
		c.ACLReplicationRate = 100
		c.ACLReplicationBurst = 100
	}, true)
	waitForLeaderEstablishment(t, srv)

	getR := structs.DCSpecificRequest{
		Datacenter: "dc1",
	}

	retry.Run(t, func(r *retry.R) {
		var status structs.ACLReplicationStatus
		err := msgpackrpc.CallWithCodec(codec, "ACL.ReplicationStatus", &getR, &status)
		require.NoError(r, err)

		require.True(r, status.Enabled)
		require.True(r, status.Running)
		require.Equal(r, "dc2", status.SourceDatacenter)
	})
}

func TestACLEndpoint_TokenRead(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	_, srv, codec := testACLServerWithConfig(t, func(c *Config) {
		c.ACLTokenMinExpirationTTL = 10 * time.Millisecond
		c.ACLTokenMaxExpirationTTL = 5 * time.Second
	}, false)

	waitForLeaderEstablishment(t, srv)

	acl := ACL{srv: srv}

	t.Run("exists and matches what we created", func(t *testing.T) {
		token, err := upsertTestToken(codec, TestDefaultInitialManagementToken, "dc1", nil)
		require.NoError(t, err)

		req := structs.ACLTokenGetRequest{
			Datacenter:   "dc1",
			TokenID:      token.AccessorID,
			TokenIDType:  structs.ACLTokenAccessor,
			QueryOptions: structs.QueryOptions{Token: TestDefaultInitialManagementToken},
		}

		resp := structs.ACLTokenResponse{}

		err = acl.TokenRead(&req, &resp)
		require.NoError(t, err)

		require.Equal(t, token, resp.Token)
	})

	t.Run("expired tokens are filtered", func(t *testing.T) {
		// insert a token that will expire
		token, err := upsertTestToken(codec, TestDefaultInitialManagementToken, "dc1", func(t *structs.ACLToken) {
			t.ExpirationTTL = 200 * time.Millisecond
		})
		require.NoError(t, err)

		t.Run("readable until expiration", func(t *testing.T) {
			req := structs.ACLTokenGetRequest{
				Datacenter:   "dc1",
				TokenID:      token.AccessorID,
				TokenIDType:  structs.ACLTokenAccessor,
				QueryOptions: structs.QueryOptions{Token: TestDefaultInitialManagementToken},
			}

			resp := structs.ACLTokenResponse{}

			require.NoError(t, acl.TokenRead(&req, &resp))
			require.Equal(t, token, resp.Token)
		})

		t.Run("not returned when expired", func(t *testing.T) {
			req := structs.ACLTokenGetRequest{
				Datacenter:   "dc1",
				TokenID:      token.AccessorID,
				TokenIDType:  structs.ACLTokenAccessor,
				QueryOptions: structs.QueryOptions{Token: TestDefaultInitialManagementToken},
			}

			resp := structs.ACLTokenResponse{}

			retry.Run(t, func(r *retry.R) {
				require.NoError(r, acl.TokenRead(&req, &resp))
				require.Nil(r, resp.Token)
			})
		})
	})

	t.Run("nil when token does not exist", func(t *testing.T) {
		fakeID, err := uuid.GenerateUUID()
		require.NoError(t, err)

		req := structs.ACLTokenGetRequest{
			Datacenter:   "dc1",
			TokenID:      fakeID,
			TokenIDType:  structs.ACLTokenAccessor,
			QueryOptions: structs.QueryOptions{Token: TestDefaultInitialManagementToken},
		}

		resp := structs.ACLTokenResponse{}

		err = acl.TokenRead(&req, &resp)
		require.Nil(t, resp.Token)
		require.NoError(t, err)
	})

	t.Run("validates ID format", func(t *testing.T) {
		req := structs.ACLTokenGetRequest{
			Datacenter:   "dc1",
			TokenID:      "definitely-really-certainly-not-a-uuid",
			TokenIDType:  structs.ACLTokenAccessor,
			QueryOptions: structs.QueryOptions{Token: TestDefaultInitialManagementToken},
		}

		resp := structs.ACLTokenResponse{}

		err := acl.TokenRead(&req, &resp)
		require.Nil(t, resp.Token)
		require.EqualError(t, err, "failed acl token lookup: index error: UUID must be 36 characters")
	})

	t.Run("expanded output with role/policy", func(t *testing.T) {
		p1, err := upsertTestPolicy(codec, TestDefaultInitialManagementToken, "dc1")
		require.NoError(t, err)

		p2, err := upsertTestPolicy(codec, TestDefaultInitialManagementToken, "dc1")
		require.NoError(t, err)

		r1, err := upsertTestCustomizedRole(codec, TestDefaultInitialManagementToken, "dc1", func(role *structs.ACLRole) {
			role.Policies = []structs.ACLRolePolicyLink{
				{
					ID: p2.ID,
				},
			}
		})
		require.NoError(t, err)

		setReq := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				Description: "foobar",
				Policies: []structs.ACLTokenPolicyLink{
					{
						ID: p1.ID,
					},
				},
				Roles: []structs.ACLTokenRoleLink{
					{
						ID: r1.ID,
					},
				},
				Local: false,
			},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		setResp := structs.ACLToken{}
		err = msgpackrpc.CallWithCodec(codec, "ACL.TokenSet", &setReq, &setResp)
		require.NoError(t, err)
		require.NotEmpty(t, setResp.AccessorID)

		req := structs.ACLTokenGetRequest{
			Datacenter:   "dc1",
			TokenID:      setResp.AccessorID,
			TokenIDType:  structs.ACLTokenAccessor,
			Expanded:     true,
			QueryOptions: structs.QueryOptions{Token: TestDefaultInitialManagementToken},
		}
		resp := structs.ACLTokenResponse{}

		err = msgpackrpc.CallWithCodec(codec, "ACL.TokenRead", &req, &resp)
		require.NoError(t, err)
		require.NotNil(t, resp.Token)
		require.ElementsMatch(t, []*structs.ACLPolicy{p1, p2}, resp.ExpandedPolicies)
		require.ElementsMatch(t, []*structs.ACLRole{r1}, resp.ExpandedRoles)
	})

	t.Run("expanded output with multiple roles that share a policy", func(t *testing.T) {
		p1, err := upsertTestPolicy(codec, TestDefaultInitialManagementToken, "dc1")
		require.NoError(t, err)

		r1, err := upsertTestCustomizedRole(codec, TestDefaultInitialManagementToken, "dc1", func(role *structs.ACLRole) {
			role.Policies = []structs.ACLRolePolicyLink{
				{
					ID: p1.ID,
				},
			}
		})
		require.NoError(t, err)

		r2, err := upsertTestCustomizedRole(codec, TestDefaultInitialManagementToken, "dc1", func(role *structs.ACLRole) {
			role.Policies = []structs.ACLRolePolicyLink{
				{
					ID: p1.ID,
				},
			}
		})
		require.NoError(t, err)

		setReq := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				Description: "foobar",
				Roles: []structs.ACLTokenRoleLink{
					{
						ID: r1.ID,
					},
					{
						ID: r2.ID,
					},
				},
				Local: false,
			},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		setResp := structs.ACLToken{}
		err = msgpackrpc.CallWithCodec(codec, "ACL.TokenSet", &setReq, &setResp)
		require.NoError(t, err)
		require.NotEmpty(t, setResp.AccessorID)

		req := structs.ACLTokenGetRequest{
			Datacenter:   "dc1",
			TokenID:      setResp.AccessorID,
			TokenIDType:  structs.ACLTokenAccessor,
			Expanded:     true,
			QueryOptions: structs.QueryOptions{Token: TestDefaultInitialManagementToken},
		}
		resp := structs.ACLTokenResponse{}

		err = msgpackrpc.CallWithCodec(codec, "ACL.TokenRead", &req, &resp)
		require.NoError(t, err)
		require.NotNil(t, resp.Token)
		require.ElementsMatch(t, []*structs.ACLPolicy{p1}, resp.ExpandedPolicies)
		require.ElementsMatch(t, []*structs.ACLRole{r1, r2}, resp.ExpandedRoles)
	})

	t.Run("expanded output with node/service identities", func(t *testing.T) {
		setReq := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				Description: "foobar",
				ServiceIdentities: []*structs.ACLServiceIdentity{
					{
						ServiceName: "web",
						Datacenters: []string{"dc1"},
					},
					{
						ServiceName: "db",
						Datacenters: []string{"dc2"},
					},
				},
				NodeIdentities: []*structs.ACLNodeIdentity{
					{
						NodeName:   "foo",
						Datacenter: "dc1",
					},
					{
						NodeName:   "bar",
						Datacenter: "dc1",
					},
				},
				Local: false,
			},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		var expectedPolicies []*structs.ACLPolicy
		entMeta := structs.DefaultEnterpriseMetaInDefaultPartition()
		for _, serviceIdentity := range setReq.ACLToken.ServiceIdentities {
			expectedPolicies = append(expectedPolicies, serviceIdentity.SyntheticPolicy(entMeta))
		}
		for _, serviceIdentity := range setReq.ACLToken.NodeIdentities {
			expectedPolicies = append(expectedPolicies, serviceIdentity.SyntheticPolicy(entMeta))
		}

		setResp := structs.ACLToken{}
		err := msgpackrpc.CallWithCodec(codec, "ACL.TokenSet", &setReq, &setResp)
		require.NoError(t, err)
		require.NotEmpty(t, setResp.AccessorID)

		req := structs.ACLTokenGetRequest{
			Datacenter:   "dc1",
			TokenID:      setResp.AccessorID,
			TokenIDType:  structs.ACLTokenAccessor,
			Expanded:     true,
			QueryOptions: structs.QueryOptions{Token: TestDefaultInitialManagementToken},
		}
		resp := structs.ACLTokenResponse{}

		err = msgpackrpc.CallWithCodec(codec, "ACL.TokenRead", &req, &resp)
		require.NoError(t, err)
		require.NotNil(t, resp.Token)
		require.ElementsMatch(t, expectedPolicies, resp.ExpandedPolicies)
	})
}

func TestACLEndpoint_TokenClone(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	_, srv, codec := testACLServerWithConfig(t, func(c *Config) {
		c.ACLTokenMinExpirationTTL = 10 * time.Millisecond
		c.ACLTokenMaxExpirationTTL = 5 * time.Second
	}, false)
	waitForLeaderEstablishment(t, srv)

	p1, err := upsertTestPolicy(codec, TestDefaultInitialManagementToken, "dc1")
	require.NoError(t, err)

	r1, err := upsertTestRole(codec, TestDefaultInitialManagementToken, "dc1")
	require.NoError(t, err)

	t1, err := upsertTestToken(codec, TestDefaultInitialManagementToken, "dc1", func(t *structs.ACLToken) {
		t.Policies = []structs.ACLTokenPolicyLink{
			{ID: p1.ID},
		}
		t.Roles = []structs.ACLTokenRoleLink{
			{ID: r1.ID},
		}
		t.ServiceIdentities = []*structs.ACLServiceIdentity{
			{ServiceName: "web"},
		}
		t.NodeIdentities = []*structs.ACLNodeIdentity{
			{NodeName: "foo", Datacenter: "bar"},
		}
	})
	require.NoError(t, err)

	endpoint := ACL{srv: srv}

	t.Run("normal", func(t *testing.T) {
		req := structs.ACLTokenSetRequest{
			Datacenter:   "dc1",
			ACLToken:     structs.ACLToken{AccessorID: t1.AccessorID},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		t2 := structs.ACLToken{}

		err = endpoint.TokenClone(&req, &t2)
		require.NoError(t, err)

		require.Equal(t, t1.Description, t2.Description)
		require.Equal(t, t1.Policies, t2.Policies)
		require.Equal(t, t1.Roles, t2.Roles)
		require.Equal(t, t1.ServiceIdentities, t2.ServiceIdentities)
		require.Equal(t, t1.NodeIdentities, t2.NodeIdentities)
		require.Equal(t, t1.Rules, t2.Rules)
		require.Equal(t, t1.Local, t2.Local)
		require.NotEqual(t, t1.AccessorID, t2.AccessorID)
		require.NotEqual(t, t1.SecretID, t2.SecretID)
	})

	t.Run("can't clone expired token", func(t *testing.T) {
		// insert a token that will expire
		t1, err := upsertTestToken(codec, TestDefaultInitialManagementToken, "dc1", func(t *structs.ACLToken) {
			t.ExpirationTTL = 11 * time.Millisecond
		})
		require.NoError(t, err)

		time.Sleep(30 * time.Millisecond)

		req := structs.ACLTokenSetRequest{
			Datacenter:   "dc1",
			ACLToken:     structs.ACLToken{AccessorID: t1.AccessorID},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		t2 := structs.ACLToken{}

		err = endpoint.TokenClone(&req, &t2)
		require.Error(t, err)
		require.Equal(t, acl.ErrNotFound, err)
	})
}

func TestACLEndpoint_TokenSet(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	_, srv, codec := testACLServerWithConfig(t, func(c *Config) {
		c.ACLTokenMinExpirationTTL = 10 * time.Millisecond
		c.ACLTokenMaxExpirationTTL = 5 * time.Second
	}, false)
	waitForLeaderEstablishment(t, srv)

	a := ACL{srv: srv}

	var tokenID string

	t.Run("Create it", func(t *testing.T) {
		req := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				Description: "foobar",
				Policies:    nil,
				Local:       false,
				NodeIdentities: []*structs.ACLNodeIdentity{
					{
						NodeName:   "foo",
						Datacenter: "dc1",
					},
				},
			},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		resp := structs.ACLToken{}

		err := a.TokenSet(&req, &resp)
		require.NoError(t, err)

		// Get the token directly to validate that it exists
		tokenResp, err := retrieveTestToken(codec, TestDefaultInitialManagementToken, "dc1", resp.AccessorID)
		require.NoError(t, err)
		token := tokenResp.Token

		require.NotNil(t, token)
		require.NotNil(t, token.AccessorID)
		require.Equal(t, token.Description, "foobar")
		require.Equal(t, token.AccessorID, resp.AccessorID)
		require.Len(t, token.NodeIdentities, 1)
		require.Equal(t, "foo", token.NodeIdentities[0].NodeName)
		require.Equal(t, "dc1", token.NodeIdentities[0].Datacenter)

		tokenID = token.AccessorID
	})

	t.Run("Update it", func(t *testing.T) {
		req := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				Description: "new-description",
				AccessorID:  tokenID,
			},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		resp := structs.ACLToken{}

		err := a.TokenSet(&req, &resp)
		require.NoError(t, err)

		// Get the token directly to validate that it exists
		tokenResp, err := retrieveTestToken(codec, TestDefaultInitialManagementToken, "dc1", resp.AccessorID)
		require.NoError(t, err)
		token := tokenResp.Token

		require.NotNil(t, token)
		require.NotNil(t, token.AccessorID)
		require.Equal(t, token.Description, "new-description")
		require.Equal(t, token.AccessorID, resp.AccessorID)
		require.Empty(t, token.NodeIdentities)
	})

	t.Run("Create it using Policies linked by id and name", func(t *testing.T) {
		policy1, err := upsertTestPolicy(codec, TestDefaultInitialManagementToken, "dc1")
		require.NoError(t, err)
		policy2, err := upsertTestPolicy(codec, TestDefaultInitialManagementToken, "dc1")
		require.NoError(t, err)

		req := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				Description: "foobar",
				Policies: []structs.ACLTokenPolicyLink{
					{
						ID: policy1.ID,
					},
					{
						Name: policy2.Name,
					},
				},
				Local: false,
			},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		resp := structs.ACLToken{}

		err = a.TokenSet(&req, &resp)
		require.NoError(t, err)

		// Delete both policies to ensure that we skip resolving ID->Name
		// in the returned data.
		require.NoError(t, deleteTestPolicy(codec, TestDefaultInitialManagementToken, "dc1", policy1.ID))
		require.NoError(t, deleteTestPolicy(codec, TestDefaultInitialManagementToken, "dc1", policy2.ID))

		// Get the token directly to validate that it exists
		tokenResp, err := retrieveTestToken(codec, TestDefaultInitialManagementToken, "dc1", resp.AccessorID)
		require.NoError(t, err)
		token := tokenResp.Token

		require.NotNil(t, token)
		require.NotNil(t, token.AccessorID)
		require.Equal(t, token.Description, "foobar")
		require.Equal(t, token.AccessorID, resp.AccessorID)

		require.Len(t, token.Policies, 0)
	})

	t.Run("Create it using Roles linked by id and name", func(t *testing.T) {
		role1, err := upsertTestRole(codec, TestDefaultInitialManagementToken, "dc1")
		require.NoError(t, err)
		role2, err := upsertTestRole(codec, TestDefaultInitialManagementToken, "dc1")
		require.NoError(t, err)

		req := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				Description: "foobar",
				Roles: []structs.ACLTokenRoleLink{
					{
						ID: role1.ID,
					},
					{
						Name: role2.Name,
					},
				},
				Local: false,
			},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		resp := structs.ACLToken{}

		err = a.TokenSet(&req, &resp)
		require.NoError(t, err)

		// Delete both roles to ensure that we skip resolving ID->Name
		// in the returned data.
		require.NoError(t, deleteTestRole(codec, TestDefaultInitialManagementToken, "dc1", role1.ID))
		require.NoError(t, deleteTestRole(codec, TestDefaultInitialManagementToken, "dc1", role2.ID))

		// Get the token directly to validate that it exists
		tokenResp, err := retrieveTestToken(codec, TestDefaultInitialManagementToken, "dc1", resp.AccessorID)
		require.NoError(t, err)
		token := tokenResp.Token

		require.NotNil(t, token)
		require.NotNil(t, token.AccessorID)
		require.Equal(t, token.Description, "foobar")
		require.Equal(t, token.AccessorID, resp.AccessorID)

		require.Len(t, token.Roles, 0)
	})

	t.Run("Create it with AuthMethod set outside of login", func(t *testing.T) {
		req := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				Description: "foobar",
				AuthMethod:  "fakemethod",
			},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		resp := structs.ACLToken{}

		err := a.TokenSet(&req, &resp)
		testutil.RequireErrorContains(t, err, "AuthMethod field is disallowed outside of login")
	})

	t.Run("Update auth method linked token and try to change auth method", func(t *testing.T) {
		acl := ACL{srv: srv}

		testSessionID := testauth.StartSession()
		defer testauth.ResetSession(testSessionID)
		testauth.InstallSessionToken(testSessionID, "fake-token", "default", "demo", "abc123")

		method1, err := upsertTestAuthMethod(codec, TestDefaultInitialManagementToken, "dc1", testSessionID)
		require.NoError(t, err)

		_, err = upsertTestBindingRule(codec, TestDefaultInitialManagementToken, "dc1", method1.Name, "", structs.BindingRuleBindTypeService, "demo")
		require.NoError(t, err)

		// create a token in one method
		methodToken := structs.ACLToken{}
		require.NoError(t, acl.Login(&structs.ACLLoginRequest{
			Auth: &structs.ACLLoginParams{
				AuthMethod:  method1.Name,
				BearerToken: "fake-token",
			},
			Datacenter: "dc1",
		}, &methodToken))

		method2, err := upsertTestAuthMethod(codec, TestDefaultInitialManagementToken, "dc1", "")
		require.NoError(t, err)

		// try to update the token and change the method
		req := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				AccessorID:  methodToken.AccessorID,
				SecretID:    methodToken.SecretID,
				AuthMethod:  method2.Name,
				Description: "updated token",
				Local:       true,
			},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		resp := structs.ACLToken{}

		err = acl.TokenSet(&req, &resp)
		testutil.RequireErrorContains(t, err, "Cannot change AuthMethod")
	})

	t.Run("Update auth method linked token and let the SecretID and AuthMethod be defaulted", func(t *testing.T) {
		acl := ACL{srv: srv}

		testSessionID := testauth.StartSession()
		defer testauth.ResetSession(testSessionID)
		testauth.InstallSessionToken(testSessionID, "fake-token", "default", "demo", "abc123")

		method, err := upsertTestAuthMethod(codec, TestDefaultInitialManagementToken, "dc1", testSessionID)
		require.NoError(t, err)

		_, err = upsertTestBindingRule(codec, TestDefaultInitialManagementToken, "dc1", method.Name, "", structs.BindingRuleBindTypeService, "demo")
		require.NoError(t, err)

		methodToken := structs.ACLToken{}
		require.NoError(t, acl.Login(&structs.ACLLoginRequest{
			Auth: &structs.ACLLoginParams{
				AuthMethod:  method.Name,
				BearerToken: "fake-token",
			},
			Datacenter: "dc1",
		}, &methodToken))

		req := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				AccessorID: methodToken.AccessorID,
				// SecretID:    methodToken.SecretID,
				// AuthMethod:     method.Name,
				Description: "updated token",
				Local:       true,
			},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		resp := structs.ACLToken{}

		require.NoError(t, acl.TokenSet(&req, &resp))

		// Get the token directly to validate that it exists
		tokenResp, err := retrieveTestToken(codec, TestDefaultInitialManagementToken, "dc1", resp.AccessorID)
		require.NoError(t, err)
		token := tokenResp.Token

		require.NotNil(t, token)
		require.Len(t, token.Roles, 0)
		require.Equal(t, "updated token", token.Description)
		require.True(t, token.Local)
		require.Equal(t, methodToken.SecretID, token.SecretID)
		require.Equal(t, methodToken.AuthMethod, token.AuthMethod)
	})

	t.Run("Create it with invalid service identity (empty)", func(t *testing.T) {
		req := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				Description: "foobar",
				Policies:    nil,
				Local:       false,
				ServiceIdentities: []*structs.ACLServiceIdentity{
					{ServiceName: ""},
				},
			},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		resp := structs.ACLToken{}

		err := a.TokenSet(&req, &resp)
		testutil.RequireErrorContains(t, err, "Service identity is missing the service name field")
	})

	t.Run("Create it with invalid service identity (too large)", func(t *testing.T) {
		long := strings.Repeat("x", acl.ServiceIdentityNameMaxLength+1)
		req := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				Description: "foobar",
				Policies:    nil,
				Local:       false,
				ServiceIdentities: []*structs.ACLServiceIdentity{
					{ServiceName: long},
				},
			},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		resp := structs.ACLToken{}

		err := a.TokenSet(&req, &resp)
		require.NotNil(t, err)
	})

	for _, test := range []struct {
		name string
		ok   bool
	}{
		{"-abc", false},
		{"abc-", false},
		{"a-bc", true},
		{"_abc", false},
		{"abc_", false},
		{"a_bc", true},
		{":abc", false},
		{"abc:", false},
		{"a:bc", false},
		{"Abc", false},
		{"aBc", false},
		{"abC", false},
		{"0abc", true},
		{"abc0", true},
		{"a0bc", true},
	} {
		var testName string
		if test.ok {
			testName = "Create it with valid service identity (by regex): " + test.name
		} else {
			testName = "Create it with invalid service identity (by regex): " + test.name
		}
		t.Run(testName, func(t *testing.T) {
			req := structs.ACLTokenSetRequest{
				Datacenter: "dc1",
				ACLToken: structs.ACLToken{
					Description: "foobar",
					Policies:    nil,
					Local:       false,
					ServiceIdentities: []*structs.ACLServiceIdentity{
						{ServiceName: test.name},
					},
				},
				WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
			}

			resp := structs.ACLToken{}

			err := a.TokenSet(&req, &resp)
			if test.ok {
				require.NoError(t, err)

				// Get the token directly to validate that it exists
				tokenResp, err := retrieveTestToken(codec, TestDefaultInitialManagementToken, "dc1", resp.AccessorID)
				require.NoError(t, err)
				token := tokenResp.Token
				require.NotNil(t, token)
				require.ElementsMatch(t, req.ACLToken.ServiceIdentities, token.ServiceIdentities)
			} else {
				require.NotNil(t, err)
			}
		})
	}

	t.Run("Create it with two of the same service identities", func(t *testing.T) {
		req := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				Description: "foobar",
				Policies:    nil,
				Local:       false,
				ServiceIdentities: []*structs.ACLServiceIdentity{
					{ServiceName: "example"},
					{ServiceName: "example"},
				},
			},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		resp := structs.ACLToken{}

		err := a.TokenSet(&req, &resp)
		require.NoError(t, err)

		// Get the token directly to validate that it exists
		tokenResp, err := retrieveTestToken(codec, TestDefaultInitialManagementToken, "dc1", resp.AccessorID)
		require.NoError(t, err)
		token := tokenResp.Token
		require.NotNil(t, token)
		require.Len(t, token.ServiceIdentities, 1)
	})

	t.Run("Create it with two of the same service identities and different DCs", func(t *testing.T) {
		req := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				Description: "foobar",
				Policies:    nil,
				Local:       false,
				ServiceIdentities: []*structs.ACLServiceIdentity{
					{
						ServiceName: "example",
						Datacenters: []string{"dc2", "dc3"},
					},
					{
						ServiceName: "example",
						Datacenters: []string{"dc1", "dc2"},
					},
				},
			},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		resp := structs.ACLToken{}

		err := a.TokenSet(&req, &resp)
		require.NoError(t, err)

		// Get the token directly to validate that it exists
		tokenResp, err := retrieveTestToken(codec, TestDefaultInitialManagementToken, "dc1", resp.AccessorID)
		require.NoError(t, err)
		token := tokenResp.Token
		require.NotNil(t, token)
		require.Len(t, token.ServiceIdentities, 1)
		svcid := token.ServiceIdentities[0]
		require.Equal(t, "example", svcid.ServiceName)
		require.ElementsMatch(t, []string{"dc1", "dc2", "dc3"}, svcid.Datacenters)
	})

	t.Run("Create it with invalid service identity (datacenters set on local token)", func(t *testing.T) {
		req := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				Description: "foobar",
				Policies:    nil,
				Local:       true,
				ServiceIdentities: []*structs.ACLServiceIdentity{
					{ServiceName: "foo", Datacenters: []string{"dc2"}},
				},
			},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		resp := structs.ACLToken{}

		err := a.TokenSet(&req, &resp)
		testutil.RequireErrorContains(t, err, "cannot specify a list of datacenters on a local token")
	})

	for _, test := range []struct {
		name         string
		offset       time.Duration
		errString    string
		errStringTTL string
	}{
		{"before create time", -5 * time.Minute, "ExpirationTime cannot be before CreateTime", ""},
		{"too soon", 1 * time.Millisecond, "ExpirationTime cannot be less than", "ExpirationTime cannot be less than"},
		{"too distant", 25 * time.Hour, "ExpirationTime cannot be more than", "ExpirationTime cannot be more than"},
	} {
		t.Run("Create it with an expiration time that is "+test.name, func(t *testing.T) {
			req := structs.ACLTokenSetRequest{
				Datacenter: "dc1",
				ACLToken: structs.ACLToken{
					Description:    "foobar",
					Policies:       nil,
					Local:          false,
					ExpirationTime: timePointer(time.Now().Add(test.offset)),
				},
				WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
			}

			resp := structs.ACLToken{}

			err := a.TokenSet(&req, &resp)
			if test.errString != "" {
				testutil.RequireErrorContains(t, err, test.errString)
			} else {
				require.NotNil(t, err)
			}
		})

		t.Run("Create it with an expiration TTL that is "+test.name, func(t *testing.T) {
			req := structs.ACLTokenSetRequest{
				Datacenter: "dc1",
				ACLToken: structs.ACLToken{
					Description:   "foobar",
					Policies:      nil,
					Local:         false,
					ExpirationTTL: test.offset,
				},
				WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
			}

			resp := structs.ACLToken{}

			err := a.TokenSet(&req, &resp)
			if test.errString != "" {
				testutil.RequireErrorContains(t, err, test.errStringTTL)
			} else {
				require.NotNil(t, err)
			}
		})
	}

	t.Run("Create it with expiration time AND expiration TTL set (error)", func(t *testing.T) {
		req := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				Description:    "foobar",
				Policies:       nil,
				Local:          false,
				ExpirationTime: timePointer(time.Now().Add(4 * time.Second)),
				ExpirationTTL:  4 * time.Second,
			},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		resp := structs.ACLToken{}

		err := a.TokenSet(&req, &resp)
		testutil.RequireErrorContains(t, err, "Expiration TTL and Expiration Time cannot both be set")
	})

	t.Run("Create it with expiration time using TTLs", func(t *testing.T) {
		req := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				Description:   "foobar",
				Policies:      nil,
				Local:         false,
				ExpirationTTL: 4 * time.Second,
			},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		resp := structs.ACLToken{}

		err := a.TokenSet(&req, &resp)
		require.NoError(t, err)

		// Get the token directly to validate that it exists
		tokenResp, err := retrieveTestToken(codec, TestDefaultInitialManagementToken, "dc1", resp.AccessorID)
		require.NoError(t, err)
		token := tokenResp.Token

		expectExpTime := resp.CreateTime.Add(4 * time.Second)

		require.NotNil(t, token)
		require.NotNil(t, token.AccessorID)
		require.Equal(t, token.Description, "foobar")
		require.Equal(t, token.AccessorID, resp.AccessorID)
		requireTimeEquals(t, &expectExpTime, resp.ExpirationTime)

		tokenID = token.AccessorID
	})

	var expTime time.Time
	t.Run("Create it with expiration time", func(t *testing.T) {
		expTime = time.Now().Add(4 * time.Second)
		req := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				Description:    "foobar",
				Policies:       nil,
				Local:          false,
				ExpirationTime: &expTime,
			},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		resp := structs.ACLToken{}

		err := a.TokenSet(&req, &resp)
		require.NoError(t, err)

		// Get the token directly to validate that it exists
		tokenResp, err := retrieveTestToken(codec, TestDefaultInitialManagementToken, "dc1", resp.AccessorID)
		require.NoError(t, err)
		token := tokenResp.Token

		require.NotNil(t, token)
		require.NotNil(t, token.AccessorID)
		require.Equal(t, token.Description, "foobar")
		require.Equal(t, token.AccessorID, resp.AccessorID)
		requireTimeEquals(t, &expTime, resp.ExpirationTime)

		tokenID = token.AccessorID
	})

	// do not insert another test at this point: these tests need to be serial

	t.Run("Update expiration time is not allowed", func(t *testing.T) {
		req := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				Description:    "new-description",
				AccessorID:     tokenID,
				ExpirationTime: timePointer(expTime.Add(-1 * time.Second)),
			},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		resp := structs.ACLToken{}

		err := a.TokenSet(&req, &resp)
		testutil.RequireErrorContains(t, err, "Cannot change expiration time")
	})

	// do not insert another test at this point: these tests need to be serial

	t.Run("Update anything except expiration time is ok - omit expiration time and let it default", func(t *testing.T) {
		req := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				Description: "new-description-1",
				AccessorID:  tokenID,
			},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		resp := structs.ACLToken{}

		err := a.TokenSet(&req, &resp)
		require.NoError(t, err)

		// Get the token directly to validate that it exists
		tokenResp, err := retrieveTestToken(codec, TestDefaultInitialManagementToken, "dc1", resp.AccessorID)
		require.NoError(t, err)
		token := tokenResp.Token

		require.NotNil(t, token)
		require.NotNil(t, token.AccessorID)
		require.Equal(t, token.Description, "new-description-1")
		require.Equal(t, token.AccessorID, resp.AccessorID)
		requireTimeEquals(t, &expTime, resp.ExpirationTime)
	})

	t.Run("Update anything except expiration time is ok", func(t *testing.T) {
		req := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				Description:    "new-description-2",
				AccessorID:     tokenID,
				ExpirationTime: &expTime,
			},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		resp := structs.ACLToken{}

		err := a.TokenSet(&req, &resp)
		require.NoError(t, err)

		// Get the token directly to validate that it exists
		tokenResp, err := retrieveTestToken(codec, TestDefaultInitialManagementToken, "dc1", resp.AccessorID)
		require.NoError(t, err)
		token := tokenResp.Token

		require.NotNil(t, token)
		require.NotNil(t, token.AccessorID)
		require.Equal(t, token.Description, "new-description-2")
		require.Equal(t, token.AccessorID, resp.AccessorID)
		requireTimeEquals(t, &expTime, resp.ExpirationTime)
	})

	t.Run("cannot update a token that is past its expiration time", func(t *testing.T) {
		// create a token that will expire
		expiringToken, err := upsertTestToken(codec, TestDefaultInitialManagementToken, "dc1", func(token *structs.ACLToken) {
			token.ExpirationTTL = 11 * time.Millisecond
		})
		require.NoError(t, err)

		time.Sleep(20 * time.Millisecond) // now 'expiringToken' is expired

		req := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				Description:   "new-description",
				AccessorID:    expiringToken.AccessorID,
				ExpirationTTL: 4 * time.Second,
			},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		resp := structs.ACLToken{}

		err = a.TokenSet(&req, &resp)
		testutil.RequireErrorContains(t, err, "Cannot find token")
	})

	t.Run("invalid node identity - no name", func(t *testing.T) {
		req := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				NodeIdentities: []*structs.ACLNodeIdentity{
					{
						Datacenter: "dc1",
					},
				},
			},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		resp := structs.ACLToken{}

		err := a.TokenSet(&req, &resp)
		testutil.RequireErrorContains(t, err, "Node identity is missing the node name field on this token")
	})

	t.Run("invalid node identity - invalid name", func(t *testing.T) {
		req := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				NodeIdentities: []*structs.ACLNodeIdentity{
					{
						NodeName:   "foo.bar",
						Datacenter: "dc1",
					},
				},
			},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		resp := structs.ACLToken{}

		err := a.TokenSet(&req, &resp)
		testutil.RequireErrorContains(t, err, "Node identity has an invalid name.")
	})
	t.Run("invalid node identity - no datacenter", func(t *testing.T) {
		req := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				NodeIdentities: []*structs.ACLNodeIdentity{
					{
						NodeName: "foo",
					},
				},
			},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		resp := structs.ACLToken{}

		err := a.TokenSet(&req, &resp)
		testutil.RequireErrorContains(t, err, "Node identity is missing the datacenter field on this token")
	})
}

func TestACLEndpoint_TokenSet_CustomID(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	_, srv, codec := testACLServerWithConfig(t, nil, false)
	waitForLeaderEstablishment(t, srv)

	acl := ACL{srv: srv}

	// No Create Arg
	t.Run("no create arg", func(t *testing.T) {
		req := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				AccessorID:  "5d62a983-bcab-4e0c-9bcd-5dabebe3e273",
				SecretID:    "10a8ad77-2bdf-4939-a9d7-1b7de79d6beb",
				Description: "foobar",
				Policies:    nil,
				Local:       false,
			},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		resp := structs.ACLToken{}

		err := acl.TokenSet(&req, &resp)
		require.Error(t, err)
	})

	// Use the Create Arg
	t.Run("create arg", func(t *testing.T) {
		req := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				AccessorID:  "5d62a983-bcab-4e0c-9bcd-5dabebe3e273",
				SecretID:    "10a8ad77-2bdf-4939-a9d7-1b7de79d6beb",
				Description: "foobar",
				Policies:    nil,
				Local:       false,
			},
			Create:       true,
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		resp := structs.ACLToken{}

		err := acl.TokenSet(&req, &resp)
		require.NoError(t, err)

		// Get the token directly to validate that it exists
		tokenResp, err := retrieveTestToken(codec, TestDefaultInitialManagementToken, "dc1", resp.AccessorID)
		require.NoError(t, err)
		token := tokenResp.Token

		require.NotNil(t, token)
		require.Equal(t, req.ACLToken.AccessorID, token.AccessorID)
		require.Equal(t, req.ACLToken.SecretID, token.SecretID)
		require.Equal(t, token.Description, "foobar")
	})

	// Reserved AccessorID
	t.Run("reserved AccessorID", func(t *testing.T) {
		req := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				AccessorID:  "00000000-0000-0000-0000-000000000073",
				Description: "foobar",
				Policies:    nil,
				Local:       false,
			},
			Create:       true,
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		resp := structs.ACLToken{}

		err := acl.TokenSet(&req, &resp)
		require.Error(t, err)
	})

	// Reserved SecretID
	t.Run("reserved SecretID", func(t *testing.T) {
		req := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				SecretID:    "00000000-0000-0000-0000-000000000073",
				Description: "foobar",
				Policies:    nil,
				Local:       false,
			},
			Create:       true,
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		resp := structs.ACLToken{}

		err := acl.TokenSet(&req, &resp)
		require.Error(t, err)
	})

	// Accessor is dup
	t.Run("accessor Dup", func(t *testing.T) {
		req := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				AccessorID:  "5d62a983-bcab-4e0c-9bcd-5dabebe3e273",
				Description: "foobar",
				Policies:    nil,
				Local:       false,
			},
			Create:       true,
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		resp := structs.ACLToken{}

		err := acl.TokenSet(&req, &resp)
		require.Error(t, err)
	})

	// Accessor is dup of secret
	t.Run("accessor dup of secret", func(t *testing.T) {
		req := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				AccessorID:  "10a8ad77-2bdf-4939-a9d7-1b7de79d6beb",
				Description: "foobar",
				Policies:    nil,
				Local:       false,
			},
			Create:       true,
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		resp := structs.ACLToken{}

		err := acl.TokenSet(&req, &resp)
		require.Error(t, err)
	})

	// Secret is dup of Accessor
	t.Run("secret dup of accessor", func(t *testing.T) {
		req := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				SecretID:    "5d62a983-bcab-4e0c-9bcd-5dabebe3e273",
				Description: "foobar",
				Policies:    nil,
				Local:       false,
			},
			Create:       true,
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		resp := structs.ACLToken{}

		err := acl.TokenSet(&req, &resp)
		require.Error(t, err)
	})

	// Secret is dup
	t.Run("secret dup", func(t *testing.T) {
		req := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				SecretID:    "10a8ad77-2bdf-4939-a9d7-1b7de79d6beb",
				Description: "foobar",
				Policies:    nil,
				Local:       false,
			},
			Create:       true,
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		resp := structs.ACLToken{}

		err := acl.TokenSet(&req, &resp)
		require.Error(t, err)
	})

	// Update Accessor attempt
	t.Run("update accessor", func(t *testing.T) {
		req := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				AccessorID:  "75a0d6a9-6882-4f7a-a053-906db1d55a73",
				SecretID:    "10a8ad77-2bdf-4939-a9d7-1b7de79d6beb",
				Description: "foobar",
				Policies:    nil,
				Local:       false,
			},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		resp := structs.ACLToken{}

		err := acl.TokenSet(&req, &resp)
		require.Error(t, err)
	})

	// Update Accessor attempt - with Create
	t.Run("update accessor create", func(t *testing.T) {
		req := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				AccessorID:  "75a0d6a9-6882-4f7a-a053-906db1d55a73",
				SecretID:    "10a8ad77-2bdf-4939-a9d7-1b7de79d6beb",
				Description: "foobar",
				Policies:    nil,
				Local:       false,
			},
			Create:       true,
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		resp := structs.ACLToken{}

		err := acl.TokenSet(&req, &resp)
		require.Error(t, err)
	})

	// Update Secret attempt
	t.Run("update secret", func(t *testing.T) {
		req := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				AccessorID:  "5d62a983-bcab-4e0c-9bcd-5dabebe3e273",
				SecretID:    "f551f807-b3a7-4483-9ade-97230c974bf3",
				Description: "foobar",
				Policies:    nil,
				Local:       false,
			},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		resp := structs.ACLToken{}

		err := acl.TokenSet(&req, &resp)
		require.Error(t, err)
	})

	// Update Secret attempt - with Create
	t.Run("update secret create", func(t *testing.T) {
		req := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				AccessorID:  "5d62a983-bcab-4e0c-9bcd-5dabebe3e273",
				SecretID:    "f551f807-b3a7-4483-9ade-97230c974bf3",
				Description: "foobar",
				Policies:    nil,
				Local:       false,
			},
			Create:       true,
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		resp := structs.ACLToken{}

		err := acl.TokenSet(&req, &resp)
		require.Error(t, err)
	})
}

func TestACLEndpoint_TokenSet_anon(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	_, srv, codec := testACLServerWithConfig(t, nil, false)
	waitForLeaderEstablishment(t, srv)

	policy, err := upsertTestPolicy(codec, TestDefaultInitialManagementToken, "dc1")
	require.NoError(t, err)

	acl := ACL{srv: srv}

	// Assign the policies to a token
	tokenUpsertReq := structs.ACLTokenSetRequest{
		Datacenter: "dc1",
		ACLToken: structs.ACLToken{
			AccessorID: structs.ACLTokenAnonymousID,
			Policies: []structs.ACLTokenPolicyLink{
				{
					ID: policy.ID,
				},
			},
		},
		WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
	}
	token := structs.ACLToken{}
	err = acl.TokenSet(&tokenUpsertReq, &token)
	require.NoError(t, err)
	require.NotEmpty(t, token.SecretID)

	tokenResp, err := retrieveTestToken(codec, TestDefaultInitialManagementToken, "dc1", structs.ACLTokenAnonymousID)
	require.NoError(t, err)
	require.Equal(t, len(tokenResp.Token.Policies), 1)
	require.Equal(t, tokenResp.Token.Policies[0].ID, policy.ID)

}

func TestACLEndpoint_TokenDelete(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	_, s1, codec := testACLServerWithConfig(t, func(c *Config) {
		c.ACLTokenMinExpirationTTL = 10 * time.Millisecond
		c.ACLTokenMaxExpirationTTL = 5 * time.Second
	}, false)

	_, s2, codec2 := testACLServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc2"
		c.ACLTokenMinExpirationTTL = 10 * time.Millisecond
		c.ACLTokenMaxExpirationTTL = 5 * time.Second
		// token replication is required to test deleting non-local tokens in secondary dc
		c.ACLTokenReplication = true
	}, true)

	waitForLeaderEstablishment(t, s1)
	waitForLeaderEstablishment(t, s2)

	// Try to join
	joinWAN(t, s2, s1)

	// Ensure s2 is authoritative.
	waitForNewACLReplication(t, s2, structs.ACLReplicateTokens, 1, 1, 0)

	acl := ACL{srv: s1}
	acl2 := ACL{srv: s2}

	existingToken, err := upsertTestToken(codec, TestDefaultInitialManagementToken, "dc1", nil)
	require.NoError(t, err)

	t.Run("deletes a token that has an expiration time in the future", func(t *testing.T) {
		// create a token that will expire
		testToken, err := upsertTestToken(codec, TestDefaultInitialManagementToken, "dc1", func(token *structs.ACLToken) {
			token.ExpirationTTL = 4 * time.Second
		})
		require.NoError(t, err)

		// Make sure the token is listable
		tokenResp, err := retrieveTestToken(codec, TestDefaultInitialManagementToken, "dc1", testToken.AccessorID)
		require.NoError(t, err)
		require.NotNil(t, tokenResp.Token)

		// Now try to delete it (this should work).
		req := structs.ACLTokenDeleteRequest{
			Datacenter:   "dc1",
			TokenID:      testToken.AccessorID,
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		var resp string

		err = acl.TokenDelete(&req, &resp)
		require.NoError(t, err)

		// Make sure the token is gone
		tokenResp, err = retrieveTestToken(codec, TestDefaultInitialManagementToken, "dc1", testToken.AccessorID)
		require.NoError(t, err)
		require.Nil(t, tokenResp.Token)
	})

	t.Run("deletes a token that is past its expiration time", func(t *testing.T) {
		// create a token that will expire
		expiringToken, err := upsertTestToken(codec, TestDefaultInitialManagementToken, "dc1", func(token *structs.ACLToken) {
			token.ExpirationTTL = 11 * time.Millisecond
		})
		require.NoError(t, err)

		time.Sleep(20 * time.Millisecond) // now 'expiringToken' is expired

		// Make sure the token is not listable (filtered due to expiry)
		tokenResp, err := retrieveTestToken(codec, TestDefaultInitialManagementToken, "dc1", expiringToken.AccessorID)
		require.NoError(t, err)
		require.Nil(t, tokenResp.Token)

		// Now try to delete it (this should work).
		req := structs.ACLTokenDeleteRequest{
			Datacenter:   "dc1",
			TokenID:      expiringToken.AccessorID,
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		var resp string

		err = acl.TokenDelete(&req, &resp)
		require.NoError(t, err)

		// Make sure the token is still gone (this time it's actually gone)
		tokenResp, err = retrieveTestToken(codec, TestDefaultInitialManagementToken, "dc1", expiringToken.AccessorID)
		require.NoError(t, err)
		require.Nil(t, tokenResp.Token)
	})

	t.Run("deletes a token", func(t *testing.T) {
		req := structs.ACLTokenDeleteRequest{
			Datacenter:   "dc1",
			TokenID:      existingToken.AccessorID,
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		var resp string

		err = acl.TokenDelete(&req, &resp)
		require.NoError(t, err)

		// Make sure the token is gone
		tokenResp, err := retrieveTestToken(codec, TestDefaultInitialManagementToken, "dc1", existingToken.AccessorID)
		require.Nil(t, tokenResp.Token)
		require.NoError(t, err)
	})

	t.Run("can't delete itself", func(t *testing.T) {
		readReq := structs.ACLTokenGetRequest{
			Datacenter:   "dc1",
			TokenID:      TestDefaultInitialManagementToken,
			TokenIDType:  structs.ACLTokenSecret,
			QueryOptions: structs.QueryOptions{Token: TestDefaultInitialManagementToken},
		}

		var out structs.ACLTokenResponse

		err := acl.TokenRead(&readReq, &out)

		require.NoError(t, err)

		req := structs.ACLTokenDeleteRequest{
			Datacenter:   "dc1",
			TokenID:      out.Token.AccessorID,
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		var resp string
		err = acl.TokenDelete(&req, &resp)
		require.EqualError(t, err, "Deletion of the request's authorization token is not permitted")
	})

	t.Run("errors when token doesn't exist", func(t *testing.T) {
		fakeID, err := uuid.GenerateUUID()
		require.NoError(t, err)

		req := structs.ACLTokenDeleteRequest{
			Datacenter:   "dc1",
			TokenID:      fakeID,
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		var resp string

		err = acl.TokenDelete(&req, &resp)
		require.NoError(t, err)

		// token should be nil
		tokenResp, err := retrieveTestToken(codec, TestDefaultInitialManagementToken, "dc1", existingToken.AccessorID)
		require.Nil(t, tokenResp.Token)
		require.NoError(t, err)
	})

	t.Run("don't segfault when attempting to delete non existent token in secondary dc", func(t *testing.T) {
		fakeID, err := uuid.GenerateUUID()
		require.NoError(t, err)

		req := structs.ACLTokenDeleteRequest{
			Datacenter:   "dc2",
			TokenID:      fakeID,
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		var resp string

		err = acl2.TokenDelete(&req, &resp)
		require.NoError(t, err)

		// token should be nil
		tokenResp, err := retrieveTestToken(codec2, TestDefaultInitialManagementToken, "dc1", existingToken.AccessorID)
		require.Nil(t, tokenResp.Token)
		require.NoError(t, err)
	})
}

func TestACLEndpoint_TokenDelete_anon(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	_, srv, codec := testACLServerWithConfig(t, nil, false)
	waitForLeaderEstablishment(t, srv)

	acl := ACL{srv: srv}

	req := structs.ACLTokenDeleteRequest{
		Datacenter:   "dc1",
		TokenID:      structs.ACLTokenAnonymousID,
		WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
	}

	var resp string

	err := acl.TokenDelete(&req, &resp)
	require.EqualError(t, err, "Delete operation not permitted on the anonymous token")

	// Make sure the token is still there
	tokenResp, err := retrieveTestToken(codec, TestDefaultInitialManagementToken, "dc1", structs.ACLTokenAnonymousID)
	require.NoError(t, err)
	require.NotNil(t, tokenResp.Token)
}

func TestACLEndpoint_TokenList(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	_, srv, codec := testACLServerWithConfig(t, func(c *Config) {
		c.ACLTokenMinExpirationTTL = 10 * time.Millisecond
		c.ACLTokenMaxExpirationTTL = 5 * time.Second
	}, false)
	waitForLeaderEstablishment(t, srv)

	acl := ACL{srv: srv}

	t1, err := upsertTestToken(codec, TestDefaultInitialManagementToken, "dc1", nil)
	require.NoError(t, err)

	t2, err := upsertTestToken(codec, TestDefaultInitialManagementToken, "dc1", nil)
	require.NoError(t, err)

	initialManagementTokenAccessorID, err := retrieveTestTokenAccessorForSecret(codec, TestDefaultInitialManagementToken, "dc1", TestDefaultInitialManagementToken)
	require.NoError(t, err)

	t.Run("normal", func(t *testing.T) {
		// this will still be racey even with inserting the token + ttl inside the test function
		// however previously inserting it outside of the subtest func resulted in this being
		// extra flakey due to there being more code that needed to run to setup the subtest
		// between when we inserted the token and when we performed the listing.
		t3, err := upsertTestToken(codec, TestDefaultInitialManagementToken, "dc1", func(token *structs.ACLToken) {
			token.ExpirationTTL = 50 * time.Millisecond
		})
		require.NoError(t, err)

		req := structs.ACLTokenListRequest{
			Datacenter:   "dc1",
			QueryOptions: structs.QueryOptions{Token: TestDefaultInitialManagementToken},
		}

		resp := structs.ACLTokenListResponse{}

		err = acl.TokenList(&req, &resp)
		require.NoError(t, err)

		tokens := []string{
			initialManagementTokenAccessorID,
			structs.ACLTokenAnonymousID,
			t1.AccessorID,
			t2.AccessorID,
			t3.AccessorID,
		}
		require.ElementsMatch(t, gatherIDs(t, resp.Tokens), tokens)
	})

	time.Sleep(50 * time.Millisecond) // now 't3' is expired

	t.Run("filter expired", func(t *testing.T) {
		req := structs.ACLTokenListRequest{
			Datacenter:   "dc1",
			QueryOptions: structs.QueryOptions{Token: TestDefaultInitialManagementToken},
		}

		resp := structs.ACLTokenListResponse{}

		err = acl.TokenList(&req, &resp)
		require.NoError(t, err)

		tokens := []string{
			initialManagementTokenAccessorID,
			structs.ACLTokenAnonymousID,
			t1.AccessorID,
			t2.AccessorID,
		}
		require.ElementsMatch(t, gatherIDs(t, resp.Tokens), tokens)
	})

	t.Run("filter SecretID for acl:read", func(t *testing.T) {
		rules := `
			acl = "read"
		`
		readOnlyToken, err := upsertTestTokenWithPolicyRules(codec, TestDefaultInitialManagementToken, "dc1", rules)
		require.NoError(t, err)

		req := structs.ACLTokenListRequest{
			Datacenter:   "dc1",
			QueryOptions: structs.QueryOptions{Token: readOnlyToken.SecretID},
		}

		resp := structs.ACLTokenListResponse{}

		err = acl.TokenList(&req, &resp)
		require.NoError(t, err)

		tokens := []string{
			initialManagementTokenAccessorID,
			structs.ACLTokenAnonymousID,
			readOnlyToken.AccessorID,
			t1.AccessorID,
			t2.AccessorID,
		}
		require.ElementsMatch(t, gatherIDs(t, resp.Tokens), tokens)
		for _, token := range resp.Tokens {
			require.Equal(t, aclfilter.RedactedToken, token.SecretID)
		}
	})
}

func TestACLEndpoint_TokenBatchRead(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	_, srv, codec := testACLServerWithConfig(t, func(c *Config) {
		c.ACLTokenMinExpirationTTL = 10 * time.Millisecond
		c.ACLTokenMaxExpirationTTL = 5 * time.Second
	}, false)
	waitForLeaderEstablishment(t, srv)

	acl := ACL{srv: srv}

	t1, err := upsertTestToken(codec, TestDefaultInitialManagementToken, "dc1", nil)
	require.NoError(t, err)

	t2, err := upsertTestToken(codec, TestDefaultInitialManagementToken, "dc1", nil)
	require.NoError(t, err)

	t3, err := upsertTestToken(codec, TestDefaultInitialManagementToken, "dc1", func(token *structs.ACLToken) {
		token.ExpirationTTL = 4 * time.Second
	})
	require.NoError(t, err)

	t.Run("normal", func(t *testing.T) {
		tokens := []string{t1.AccessorID, t2.AccessorID, t3.AccessorID}

		req := structs.ACLTokenBatchGetRequest{
			Datacenter:   "dc1",
			AccessorIDs:  tokens,
			QueryOptions: structs.QueryOptions{Token: TestDefaultInitialManagementToken},
		}

		resp := structs.ACLTokenBatchResponse{}

		err = acl.TokenBatchRead(&req, &resp)
		require.NoError(t, err)
		require.ElementsMatch(t, gatherIDs(t, resp.Tokens), tokens)
	})

	time.Sleep(20 * time.Millisecond) // now 't3' is expired

	t.Run("returns expired tokens", func(t *testing.T) {
		tokens := []string{t1.AccessorID, t2.AccessorID, t3.AccessorID}

		req := structs.ACLTokenBatchGetRequest{
			Datacenter:   "dc1",
			AccessorIDs:  tokens,
			QueryOptions: structs.QueryOptions{Token: TestDefaultInitialManagementToken},
		}

		resp := structs.ACLTokenBatchResponse{}

		err = acl.TokenBatchRead(&req, &resp)
		require.NoError(t, err)
		require.ElementsMatch(t, gatherIDs(t, resp.Tokens), tokens)
	})
}

func TestACLEndpoint_PolicyRead(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	_, srv, codec := testACLServerWithConfig(t, nil, false)
	waitForLeaderEstablishment(t, srv)

	policy, err := upsertTestPolicy(codec, TestDefaultInitialManagementToken, "dc1")
	require.NoError(t, err)

	acl := ACL{srv: srv}

	req := structs.ACLPolicyGetRequest{
		Datacenter:   "dc1",
		PolicyID:     policy.ID,
		QueryOptions: structs.QueryOptions{Token: TestDefaultInitialManagementToken},
	}

	resp := structs.ACLPolicyResponse{}

	err = acl.PolicyRead(&req, &resp)
	require.NoError(t, err)
	require.Equal(t, policy, resp.Policy)
}

func TestACLEndpoint_PolicyReadByName(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	_, srv, codec := testACLServerWithConfig(t, nil, false)
	waitForLeaderEstablishment(t, srv)

	policy, err := upsertTestPolicy(codec, TestDefaultInitialManagementToken, "dc1")
	require.NoError(t, err)

	acl := ACL{srv: srv}

	req := structs.ACLPolicyGetRequest{
		Datacenter:   "dc1",
		PolicyName:   policy.Name,
		QueryOptions: structs.QueryOptions{Token: TestDefaultInitialManagementToken},
	}

	resp := structs.ACLPolicyResponse{}

	err = acl.PolicyRead(&req, &resp)
	require.NoError(t, err)
	require.Equal(t, policy, resp.Policy)
}

func TestACLEndpoint_PolicyBatchRead(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	_, srv, codec := testACLServerWithConfig(t, nil, false)
	waitForLeaderEstablishment(t, srv)

	p1, err := upsertTestPolicy(codec, TestDefaultInitialManagementToken, "dc1")
	require.NoError(t, err)

	p2, err := upsertTestPolicy(codec, TestDefaultInitialManagementToken, "dc1")
	require.NoError(t, err)

	acl := ACL{srv: srv}
	policies := []string{p1.ID, p2.ID}

	req := structs.ACLPolicyBatchGetRequest{
		Datacenter:   "dc1",
		PolicyIDs:    policies,
		QueryOptions: structs.QueryOptions{Token: TestDefaultInitialManagementToken},
	}

	resp := structs.ACLPolicyBatchResponse{}

	err = acl.PolicyBatchRead(&req, &resp)
	require.NoError(t, err)
	require.ElementsMatch(t, gatherIDs(t, resp.Policies), []string{p1.ID, p2.ID})
}

func TestACLEndpoint_PolicySet(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	_, srv, codec := testACLServerWithConfig(t, nil, false)
	waitForLeaderEstablishment(t, srv)
	acl := ACL{srv: srv}

	var policyID string

	t.Run("Create it", func(t *testing.T) {
		req := structs.ACLPolicySetRequest{
			Datacenter: "dc1",
			Policy: structs.ACLPolicy{
				Description: "foobar",
				Name:        "baz",
				Rules:       "service \"\" { policy = \"read\" }",
			},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}
		resp := structs.ACLPolicy{}

		err := acl.PolicySet(&req, &resp)
		require.NoError(t, err)
		require.NotNil(t, resp.ID)

		// Get the policy directly to validate that it exists
		policyResp, err := retrieveTestPolicy(codec, TestDefaultInitialManagementToken, "dc1", resp.ID)
		require.NoError(t, err)
		policy := policyResp.Policy

		require.NotNil(t, policy.ID)
		require.Equal(t, policy.Description, "foobar")
		require.Equal(t, policy.Name, "baz")
		require.Equal(t, policy.Rules, "service \"\" { policy = \"read\" }")

		policyID = policy.ID
	})

	t.Run("Name Dup", func(t *testing.T) {
		req := structs.ACLPolicySetRequest{
			Datacenter: "dc1",
			Policy: structs.ACLPolicy{
				Description: "foobar",
				Name:        "baz",
				Rules:       "service \"\" { policy = \"read\" }",
			},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}
		resp := structs.ACLPolicy{}

		err := acl.PolicySet(&req, &resp)
		require.Error(t, err)
	})

	t.Run("Update it", func(t *testing.T) {
		req := structs.ACLPolicySetRequest{
			Datacenter: "dc1",
			Policy: structs.ACLPolicy{
				ID:          policyID,
				Description: "bat",
				Name:        "bar",
				Rules:       "service \"\" { policy = \"write\" }",
			},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}
		resp := structs.ACLPolicy{}

		err := acl.PolicySet(&req, &resp)
		require.NoError(t, err)
		require.NotNil(t, resp.ID)

		// Get the policy directly to validate that it exists
		policyResp, err := retrieveTestPolicy(codec, TestDefaultInitialManagementToken, "dc1", resp.ID)
		require.NoError(t, err)
		policy := policyResp.Policy

		require.NotNil(t, policy.ID)
		require.Equal(t, policy.Description, "bat")
		require.Equal(t, policy.Name, "bar")
		require.Equal(t, policy.Rules, "service \"\" { policy = \"write\" }")
	})
}

func TestACLEndpoint_PolicySet_CustomID(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	_, srv, _ := testACLServerWithConfig(t, nil, false)
	waitForLeaderEstablishment(t, srv)

	acl := ACL{srv: srv}

	// Attempt to create policy with ID
	req := structs.ACLPolicySetRequest{
		Datacenter: "dc1",
		Policy: structs.ACLPolicy{
			ID:          "7ee166a5-b4b7-453c-bdc0-bca8ce50823e",
			Description: "foobar",
			Name:        "baz",
			Rules:       "service \"\" { policy = \"read\" }",
		},
		WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
	}
	resp := structs.ACLPolicy{}

	err := acl.PolicySet(&req, &resp)
	require.Error(t, err)
}

func TestACLEndpoint_PolicySet_builtins(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	_, srv, codec := testACLServerWithConfig(t, nil, false)
	waitForLeaderEstablishment(t, srv)

	acl := ACL{srv: srv}

	for _, builtinPolicy := range structs.ACLBuiltinPolicies {
		name := fmt.Sprintf("foobar-%s", builtinPolicy.Name) // This is required to get past validation

		// Can't change the rules
		{
			req := structs.ACLPolicySetRequest{
				Datacenter: "dc1",
				Policy: structs.ACLPolicy{
					ID:    builtinPolicy.ID,
					Name:  name,
					Rules: "service \"\" { policy = \"write\" }",
				},
				WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
			}
			resp := structs.ACLPolicy{}

			err := acl.PolicySet(&req, &resp)
			require.EqualError(t, err, fmt.Sprintf("Changing the Rules for the builtin %s policy is not permitted", builtinPolicy.Name))
		}

		// Can rename it
		{
			req := structs.ACLPolicySetRequest{
				Datacenter: "dc1",
				Policy: structs.ACLPolicy{
					ID:    builtinPolicy.ID,
					Name:  name,
					Rules: builtinPolicy.Rules,
				},
				WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
			}
			resp := structs.ACLPolicy{}

			err := acl.PolicySet(&req, &resp)
			require.NoError(t, err)

			// Get the policy again
			policyResp, err := retrieveTestPolicy(codec, TestDefaultInitialManagementToken, "dc1", builtinPolicy.ID)
			require.NoError(t, err)
			policy := policyResp.Policy

			require.Equal(t, policy.ID, builtinPolicy.ID)
			require.Equal(t, policy.Name, name)
		}
	}
}

func TestACLEndpoint_PolicyDelete(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	_, srv, codec := testACLServerWithConfig(t, nil, false)
	waitForLeaderEstablishment(t, srv)

	existingPolicy, err := upsertTestPolicy(codec, TestDefaultInitialManagementToken, "dc1")
	require.NoError(t, err)

	acl := ACL{srv: srv}

	req := structs.ACLPolicyDeleteRequest{
		Datacenter:   "dc1",
		PolicyID:     existingPolicy.ID,
		WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
	}

	var resp string

	err = acl.PolicyDelete(&req, &resp)
	require.NoError(t, err)

	// Make sure the policy is gone
	tokenResp, err := retrieveTestPolicy(codec, TestDefaultInitialManagementToken, "dc1", existingPolicy.ID)
	require.NoError(t, err)
	require.Nil(t, tokenResp.Policy)
}

func TestACLEndpoint_PolicyDelete_builtins(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	_, srv, _ := testACLServerWithConfig(t, nil, false)
	waitForLeaderEstablishment(t, srv)
	acl := ACL{srv: srv}

	for _, builtinPolicy := range structs.ACLBuiltinPolicies {
		req := structs.ACLPolicyDeleteRequest{
			Datacenter:   "dc1",
			PolicyID:     builtinPolicy.ID,
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}
		var resp string

		err := acl.PolicyDelete(&req, &resp)
		require.EqualError(t, err, fmt.Sprintf("Delete operation not permitted on the builtin %s policy", builtinPolicy.Name))
	}
}

func TestACLEndpoint_PolicyList(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	_, srv, codec := testACLServerWithConfig(t, nil, false)
	waitForLeaderEstablishment(t, srv)

	p1, err := upsertTestPolicy(codec, TestDefaultInitialManagementToken, "dc1")
	require.NoError(t, err)

	p2, err := upsertTestPolicy(codec, TestDefaultInitialManagementToken, "dc1")
	require.NoError(t, err)

	acl := ACL{srv: srv}

	req := structs.ACLPolicyListRequest{
		Datacenter:   "dc1",
		QueryOptions: structs.QueryOptions{Token: TestDefaultInitialManagementToken},
	}

	resp := structs.ACLPolicyListResponse{}

	err = acl.PolicyList(&req, &resp)
	require.NoError(t, err)

	policies := []string{
		structs.ACLPolicyGlobalManagementID,
		structs.ACLPolicyGlobalReadOnlyID,
		p1.ID,
		p2.ID,
	}
	require.ElementsMatch(t, gatherIDs(t, resp.Policies), policies)
}

func TestACLEndpoint_PolicyResolve(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	_, srv, codec := testACLServerWithConfig(t, nil, false)
	waitForLeaderEstablishment(t, srv)

	p1, err := upsertTestPolicy(codec, TestDefaultInitialManagementToken, "dc1")
	require.NoError(t, err)

	p2, err := upsertTestPolicy(codec, TestDefaultInitialManagementToken, "dc1")
	require.NoError(t, err)

	acl := ACL{srv: srv}

	policies := []string{p1.ID, p2.ID}

	// Assign the policies to a token
	tokenUpsertReq := structs.ACLTokenSetRequest{
		Datacenter: "dc1",
		ACLToken: structs.ACLToken{
			Policies: []structs.ACLTokenPolicyLink{
				{
					ID: p1.ID,
				},
				{
					ID: p2.ID,
				},
			},
		},
		WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
	}
	token := structs.ACLToken{}
	err = acl.TokenSet(&tokenUpsertReq, &token)
	require.NoError(t, err)
	require.NotEmpty(t, token.SecretID)

	resp := structs.ACLPolicyBatchResponse{}
	req := structs.ACLPolicyBatchGetRequest{
		Datacenter:   "dc1",
		PolicyIDs:    []string{p1.ID, p2.ID},
		QueryOptions: structs.QueryOptions{Token: token.SecretID},
	}
	err = acl.PolicyResolve(&req, &resp)
	require.NoError(t, err)
	require.ElementsMatch(t, gatherIDs(t, resp.Policies), policies)
}

func TestACLEndpoint_RoleRead(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	_, srv, codec := testACLServerWithConfig(t, nil, false)
	waitForLeaderEstablishment(t, srv)

	role, err := upsertTestRole(codec, TestDefaultInitialManagementToken, "dc1")
	require.NoError(t, err)

	acl := ACL{srv: srv}

	req := structs.ACLRoleGetRequest{
		Datacenter:   "dc1",
		RoleID:       role.ID,
		QueryOptions: structs.QueryOptions{Token: TestDefaultInitialManagementToken},
	}

	resp := structs.ACLRoleResponse{}

	err = acl.RoleRead(&req, &resp)
	require.NoError(t, err)
	require.Equal(t, role, resp.Role)
}

func TestACLEndpoint_RoleBatchRead(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	_, srv, codec := testACLServerWithConfig(t, nil, false)
	waitForLeaderEstablishment(t, srv)

	r1, err := upsertTestRole(codec, TestDefaultInitialManagementToken, "dc1")
	require.NoError(t, err)

	r2, err := upsertTestRole(codec, TestDefaultInitialManagementToken, "dc1")
	require.NoError(t, err)

	acl := ACL{srv: srv}
	roles := []string{r1.ID, r2.ID}

	req := structs.ACLRoleBatchGetRequest{
		Datacenter:   "dc1",
		RoleIDs:      roles,
		QueryOptions: structs.QueryOptions{Token: TestDefaultInitialManagementToken},
	}

	resp := structs.ACLRoleBatchResponse{}

	err = acl.RoleBatchRead(&req, &resp)
	require.NoError(t, err)
	require.ElementsMatch(t, gatherIDs(t, resp.Roles), roles)
}

func TestACLEndpoint_RoleSet(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	_, srv, codec := testACLServerWithConfig(t, nil, false)
	waitForLeaderEstablishment(t, srv)

	a := ACL{srv: srv}
	var roleID string

	testPolicy1, err := upsertTestPolicy(codec, TestDefaultInitialManagementToken, "dc1")
	require.NoError(t, err)
	testPolicy2, err := upsertTestPolicy(codec, TestDefaultInitialManagementToken, "dc1")
	require.NoError(t, err)

	t.Run("Create it", func(t *testing.T) {
		req := structs.ACLRoleSetRequest{
			Datacenter: "dc1",
			Role: structs.ACLRole{
				Description: "foobar",
				Name:        "baz",
				Policies: []structs.ACLRolePolicyLink{
					{
						ID: testPolicy1.ID,
					},
				},
				NodeIdentities: []*structs.ACLNodeIdentity{
					{
						NodeName:   "foo",
						Datacenter: "bar",
					},
				},
			},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}
		resp := structs.ACLRole{}

		err := a.RoleSet(&req, &resp)
		require.NoError(t, err)
		require.NotNil(t, resp.ID)

		// Get the role directly to validate that it exists
		roleResp, err := retrieveTestRole(codec, TestDefaultInitialManagementToken, "dc1", resp.ID)
		require.NoError(t, err)
		role := roleResp.Role

		require.NotNil(t, role.ID)
		require.Equal(t, role.Description, "foobar")
		require.Equal(t, role.Name, "baz")
		require.Len(t, role.Policies, 1)
		require.Equal(t, testPolicy1.ID, role.Policies[0].ID)
		require.Len(t, role.NodeIdentities, 1)
		require.Equal(t, "foo", role.NodeIdentities[0].NodeName)
		require.Equal(t, "bar", role.NodeIdentities[0].Datacenter)

		roleID = role.ID
	})

	t.Run("Update it", func(t *testing.T) {
		req := structs.ACLRoleSetRequest{
			Datacenter: "dc1",
			Role: structs.ACLRole{
				ID:          roleID,
				Description: "bat",
				Name:        "bar",
				Policies: []structs.ACLRolePolicyLink{
					{
						ID: testPolicy2.ID,
					},
				},
			},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}
		resp := structs.ACLRole{}

		err := a.RoleSet(&req, &resp)
		require.NoError(t, err)
		require.NotNil(t, resp.ID)

		// Get the role directly to validate that it exists
		roleResp, err := retrieveTestRole(codec, TestDefaultInitialManagementToken, "dc1", resp.ID)
		require.NoError(t, err)
		role := roleResp.Role

		require.NotNil(t, role.ID)
		require.Equal(t, role.Description, "bat")
		require.Equal(t, role.Name, "bar")
		require.Len(t, role.Policies, 1)
		require.Equal(t, testPolicy2.ID, role.Policies[0].ID)
		require.Empty(t, role.NodeIdentities)
	})

	t.Run("Create it using Policies linked by id and name", func(t *testing.T) {
		policy1, err := upsertTestPolicy(codec, TestDefaultInitialManagementToken, "dc1")
		require.NoError(t, err)
		policy2, err := upsertTestPolicy(codec, TestDefaultInitialManagementToken, "dc1")
		require.NoError(t, err)

		req := structs.ACLRoleSetRequest{
			Datacenter: "dc1",
			Role: structs.ACLRole{
				Description: "foobar",
				Name:        "baz",
				Policies: []structs.ACLRolePolicyLink{
					{
						ID: policy1.ID,
					},
					{
						Name: policy2.Name,
					},
				},
			},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}
		resp := structs.ACLRole{}

		err = a.RoleSet(&req, &resp)
		require.NoError(t, err)
		require.NotNil(t, resp.ID)

		// Delete both policies to ensure that we skip resolving ID->Name
		// in the returned data.
		require.NoError(t, deleteTestPolicy(codec, TestDefaultInitialManagementToken, "dc1", policy1.ID))
		require.NoError(t, deleteTestPolicy(codec, TestDefaultInitialManagementToken, "dc1", policy2.ID))

		// Get the role directly to validate that it exists
		roleResp, err := retrieveTestRole(codec, TestDefaultInitialManagementToken, "dc1", resp.ID)
		require.NoError(t, err)
		role := roleResp.Role

		require.NotNil(t, role.ID)
		require.Equal(t, role.Description, "foobar")
		require.Equal(t, role.Name, "baz")

		require.Len(t, role.Policies, 0)
	})

	roleNameGen := func(t *testing.T) string {
		t.Helper()
		name, err := uuid.GenerateUUID()
		require.NoError(t, err)
		return name
	}

	t.Run("Create it with invalid service identity (empty)", func(t *testing.T) {
		req := structs.ACLRoleSetRequest{
			Datacenter: "dc1",
			Role: structs.ACLRole{
				Description: "foobar",
				Name:        roleNameGen(t),
				ServiceIdentities: []*structs.ACLServiceIdentity{
					{ServiceName: ""},
				},
			},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}
		resp := structs.ACLRole{}

		err := a.RoleSet(&req, &resp)
		testutil.RequireErrorContains(t, err, "Service identity is missing the service name field")
	})

	t.Run("Create it with invalid service identity (too large)", func(t *testing.T) {
		long := strings.Repeat("x", acl.ServiceIdentityNameMaxLength+1)
		req := structs.ACLRoleSetRequest{
			Datacenter: "dc1",
			Role: structs.ACLRole{
				Description: "foobar",
				Name:        roleNameGen(t),
				ServiceIdentities: []*structs.ACLServiceIdentity{
					{ServiceName: long},
				},
			},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}
		resp := structs.ACLRole{}

		err := a.RoleSet(&req, &resp)
		require.NotNil(t, err)
	})

	for _, test := range []struct {
		name string
		ok   bool
	}{
		{"-abc", false},
		{"abc-", false},
		{"a-bc", true},
		{"_abc", false},
		{"abc_", false},
		{"a_bc", true},
		{":abc", false},
		{"abc:", false},
		{"a:bc", false},
		{"Abc", false},
		{"aBc", false},
		{"abC", false},
		{"0abc", true},
		{"abc0", true},
		{"a0bc", true},
	} {
		var testName string
		if test.ok {
			testName = "Create it with valid service identity (by regex): " + test.name
		} else {
			testName = "Create it with invalid service identity (by regex): " + test.name
		}
		t.Run(testName, func(t *testing.T) {
			req := structs.ACLRoleSetRequest{
				Datacenter: "dc1",
				Role: structs.ACLRole{
					Description: "foobar",
					Name:        roleNameGen(t),
					ServiceIdentities: []*structs.ACLServiceIdentity{
						{ServiceName: test.name},
					},
				},
				WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
			}

			resp := structs.ACLRole{}

			err := a.RoleSet(&req, &resp)
			if test.ok {
				require.NoError(t, err)

				// Get the token directly to validate that it exists
				roleResp, err := retrieveTestRole(codec, TestDefaultInitialManagementToken, "dc1", resp.ID)
				require.NoError(t, err)
				role := roleResp.Role
				require.ElementsMatch(t, req.Role.ServiceIdentities, role.ServiceIdentities)
			} else {
				require.NotNil(t, err)
			}
		})
	}

	t.Run("Create it with two of the same service identities", func(t *testing.T) {
		req := structs.ACLRoleSetRequest{
			Datacenter: "dc1",
			Role: structs.ACLRole{
				Description: "foobar",
				Name:        roleNameGen(t),
				ServiceIdentities: []*structs.ACLServiceIdentity{
					{ServiceName: "example"},
					{ServiceName: "example"},
				},
			},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		resp := structs.ACLRole{}

		err := a.RoleSet(&req, &resp)
		require.NoError(t, err)

		// Get the role directly to validate that it exists
		roleResp, err := retrieveTestRole(codec, TestDefaultInitialManagementToken, "dc1", resp.ID)
		require.NoError(t, err)
		role := roleResp.Role
		require.Len(t, role.ServiceIdentities, 1)
	})

	t.Run("Create it with two of the same service identities and different DCs", func(t *testing.T) {
		req := structs.ACLRoleSetRequest{
			Datacenter: "dc1",
			Role: structs.ACLRole{
				Description: "foobar",
				Name:        roleNameGen(t),
				ServiceIdentities: []*structs.ACLServiceIdentity{
					{
						ServiceName: "example",
						Datacenters: []string{"dc2", "dc3"},
					},
					{
						ServiceName: "example",
						Datacenters: []string{"dc1", "dc2"},
					},
				},
			},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		resp := structs.ACLRole{}

		err := a.RoleSet(&req, &resp)
		require.NoError(t, err)

		// Get the role directly to validate that it exists
		roleResp, err := retrieveTestRole(codec, TestDefaultInitialManagementToken, "dc1", resp.ID)
		require.NoError(t, err)
		role := roleResp.Role
		require.Len(t, role.ServiceIdentities, 1)
		svcid := role.ServiceIdentities[0]
		require.Equal(t, "example", svcid.ServiceName)
		require.ElementsMatch(t, []string{"dc1", "dc2", "dc3"}, svcid.Datacenters)
	})

	t.Run("invalid node identity - no name", func(t *testing.T) {
		req := structs.ACLRoleSetRequest{
			Datacenter: "dc1",
			Role: structs.ACLRole{
				Name: roleNameGen(t),
				NodeIdentities: []*structs.ACLNodeIdentity{
					{
						Datacenter: "dc1",
					},
				},
			},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		resp := structs.ACLRole{}

		err := a.RoleSet(&req, &resp)
		testutil.RequireErrorContains(t, err, "Node identity is missing the node name field on this role")
	})

	t.Run("invalid node identity - invalid name", func(t *testing.T) {
		req := structs.ACLRoleSetRequest{
			Datacenter: "dc1",
			Role: structs.ACLRole{
				Name: roleNameGen(t),
				NodeIdentities: []*structs.ACLNodeIdentity{
					{
						NodeName:   "foo.bar",
						Datacenter: "dc1",
					},
				},
			},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		resp := structs.ACLRole{}

		err := a.RoleSet(&req, &resp)
		testutil.RequireErrorContains(t, err, "Node identity has an invalid name.")
	})
	t.Run("invalid node identity - no datacenter", func(t *testing.T) {
		req := structs.ACLRoleSetRequest{
			Datacenter: "dc1",
			Role: structs.ACLRole{
				Name: roleNameGen(t),
				NodeIdentities: []*structs.ACLNodeIdentity{
					{
						NodeName: "foo",
					},
				},
			},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		resp := structs.ACLRole{}

		err := a.RoleSet(&req, &resp)
		testutil.RequireErrorContains(t, err, "Node identity is missing the datacenter field on this role")
	})
}

func TestACLEndpoint_RoleSet_names(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	_, srv, codec := testACLServerWithConfig(t, nil, false)
	waitForLeaderEstablishment(t, srv)

	acl := ACL{srv: srv}
	testPolicy1, err := upsertTestPolicy(codec, TestDefaultInitialManagementToken, "dc1")

	require.NoError(t, err)

	for _, test := range []struct {
		name string
		ok   bool
	}{
		{"", false},
		{"-bad", true},
		{"bad-", true},
		{"bad?bad", false},
		{strings.Repeat("x", 257), false},
		{strings.Repeat("x", 256), true},
		{"-abc", true},
		{"abc-", true},
		{"a-bc", true},
		{"_abc", true},
		{"abc_", true},
		{"a_bc", true},
		{":abc", false},
		{"abc:", false},
		{"a:bc", false},
		{"Abc", true},
		{"aBc", true},
		{"abC", true},
		{"0abc", true},
		{"abc0", true},
		{"a0bc", true},
	} {
		var testName string
		if test.ok {
			testName = "create with valid name: " + test.name
		} else {
			testName = "create with invalid name: " + test.name
		}

		t.Run(testName, func(t *testing.T) {
			// cleanup from a prior insertion that may have succeeded
			require.NoError(t, deleteTestRoleByName(codec, TestDefaultInitialManagementToken, "dc1", test.name))

			req := structs.ACLRoleSetRequest{
				Datacenter: "dc1",
				Role: structs.ACLRole{
					Name:        test.name,
					Description: "foobar",
					Policies: []structs.ACLRolePolicyLink{
						{
							ID: testPolicy1.ID,
						},
					},
				},
				WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
			}
			resp := structs.ACLRole{}

			err := acl.RoleSet(&req, &resp)
			if test.ok {
				require.NoError(t, err)

				roleResp, err := retrieveTestRole(codec, TestDefaultInitialManagementToken, "dc1", resp.ID)
				require.NoError(t, err)
				role := roleResp.Role
				require.Equal(t, test.name, role.Name)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestACLEndpoint_RoleDelete(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	_, srv, codec := testACLServerWithConfig(t, nil, false)
	waitForLeaderEstablishment(t, srv)
	existingRole, err := upsertTestRole(codec, TestDefaultInitialManagementToken, "dc1")

	require.NoError(t, err)

	acl := ACL{srv: srv}

	req := structs.ACLRoleDeleteRequest{
		Datacenter:   "dc1",
		RoleID:       existingRole.ID,
		WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
	}

	var resp string

	err = acl.RoleDelete(&req, &resp)
	require.NoError(t, err)

	// Make sure the role is gone
	roleResp, err := retrieveTestRole(codec, TestDefaultInitialManagementToken, "dc1", existingRole.ID)
	require.NoError(t, err)
	require.Nil(t, roleResp.Role)
}

func TestACLEndpoint_RoleList(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	_, srv, codec := testACLServerWithConfig(t, nil, false)
	waitForLeaderEstablishment(t, srv)

	r1, err := upsertTestRole(codec, TestDefaultInitialManagementToken, "dc1")
	require.NoError(t, err)

	r2, err := upsertTestRole(codec, TestDefaultInitialManagementToken, "dc1")
	require.NoError(t, err)

	acl := ACL{srv: srv}

	req := structs.ACLRoleListRequest{
		Datacenter:   "dc1",
		QueryOptions: structs.QueryOptions{Token: TestDefaultInitialManagementToken},
	}

	resp := structs.ACLRoleListResponse{}

	err = acl.RoleList(&req, &resp)
	require.NoError(t, err)
	require.ElementsMatch(t, gatherIDs(t, resp.Roles), []string{r1.ID, r2.ID})
}

func TestACLEndpoint_RoleResolve(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	_, srv, codec := testACLServerWithConfig(t, nil, false)
	waitForLeaderEstablishment(t, srv)

	t.Run("Normal", func(t *testing.T) {
		r1, err := upsertTestRole(codec, TestDefaultInitialManagementToken, "dc1")
		require.NoError(t, err)

		r2, err := upsertTestRole(codec, TestDefaultInitialManagementToken, "dc1")
		require.NoError(t, err)

		acl := ACL{srv: srv}

		// Assign the roles to a token
		tokenUpsertReq := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				Roles: []structs.ACLTokenRoleLink{
					{
						ID: r1.ID,
					},
					{
						ID: r2.ID,
					},
				},
			},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}
		token := structs.ACLToken{}
		err = acl.TokenSet(&tokenUpsertReq, &token)
		require.NoError(t, err)
		require.NotEmpty(t, token.SecretID)

		resp := structs.ACLRoleBatchResponse{}
		req := structs.ACLRoleBatchGetRequest{
			Datacenter:   "dc1",
			RoleIDs:      []string{r1.ID, r2.ID},
			QueryOptions: structs.QueryOptions{Token: token.SecretID},
		}
		err = acl.RoleResolve(&req, &resp)
		require.NoError(t, err)
		require.ElementsMatch(t, gatherIDs(t, resp.Roles), []string{r1.ID, r2.ID})
	})
}

func TestACLEndpoint_AuthMethodSet(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	tempDir, err := ioutil.TempDir("", "consul")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(tempDir) })
	_, srv, codec := testACLServerWithConfig(t, nil, false)

	waitForLeaderEstablishment(t, srv)

	acl := ACL{srv: srv}

	newAuthMethod := func(name string) structs.ACLAuthMethod {
		return structs.ACLAuthMethod{
			Name:        name,
			Description: "test",
			Type:        "testing",
		}
	}

	t.Run("Create", func(t *testing.T) {
		reqMethod := newAuthMethod("test")

		req := structs.ACLAuthMethodSetRequest{
			Datacenter:   "dc1",
			AuthMethod:   reqMethod,
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}
		resp := structs.ACLAuthMethod{}

		err := acl.AuthMethodSet(&req, &resp)
		require.NoError(t, err)

		// Get the method directly to validate that it exists
		methodResp, err := retrieveTestAuthMethod(codec, TestDefaultInitialManagementToken, "dc1", resp.Name)
		require.NoError(t, err)
		method := methodResp.AuthMethod

		require.Equal(t, method.Name, "test")
		require.Equal(t, method.Description, "test")
		require.Equal(t, method.Type, "testing")
	})

	t.Run("Update fails; not allowed to change types", func(t *testing.T) {
		reqMethod := newAuthMethod("test")
		reqMethod.Type = "invalid"

		req := structs.ACLAuthMethodSetRequest{
			Datacenter:   "dc1",
			AuthMethod:   reqMethod,
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}
		resp := structs.ACLAuthMethod{}

		err := acl.AuthMethodSet(&req, &resp)
		require.Error(t, err)
	})

	t.Run("Update - allow type to default", func(t *testing.T) {
		reqMethod := newAuthMethod("test")
		reqMethod.DisplayName = "updated display name 1"
		reqMethod.Description = "test modified 1"
		reqMethod.Type = "" // unset

		req := structs.ACLAuthMethodSetRequest{
			Datacenter:   "dc1",
			AuthMethod:   reqMethod,
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}
		resp := structs.ACLAuthMethod{}

		err := acl.AuthMethodSet(&req, &resp)
		require.NoError(t, err)

		// Get the method directly to validate that it exists
		methodResp, err := retrieveTestAuthMethod(codec, TestDefaultInitialManagementToken, "dc1", resp.Name)
		require.NoError(t, err)
		method := methodResp.AuthMethod

		require.Equal(t, method.Name, "test")
		require.Equal(t, method.DisplayName, "updated display name 1")
		require.Equal(t, method.Description, "test modified 1")
		require.Equal(t, method.Type, "testing")
	})

	t.Run("Update - specify type", func(t *testing.T) {
		reqMethod := newAuthMethod("test")
		reqMethod.DisplayName = "updated display name 2"
		reqMethod.Description = "test modified 2"

		req := structs.ACLAuthMethodSetRequest{
			Datacenter:   "dc1",
			AuthMethod:   reqMethod,
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}
		resp := structs.ACLAuthMethod{}

		err := acl.AuthMethodSet(&req, &resp)
		require.NoError(t, err)

		// Get the method directly to validate that it exists
		methodResp, err := retrieveTestAuthMethod(codec, TestDefaultInitialManagementToken, "dc1", resp.Name)
		require.NoError(t, err)
		method := methodResp.AuthMethod

		require.Equal(t, method.Name, "test")
		require.Equal(t, method.DisplayName, "updated display name 2")
		require.Equal(t, method.Description, "test modified 2")
		require.Equal(t, method.Type, "testing")
	})

	t.Run("Create with no name", func(t *testing.T) {
		req := structs.ACLAuthMethodSetRequest{
			Datacenter:   "dc1",
			AuthMethod:   newAuthMethod(""),
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}
		resp := structs.ACLAuthMethod{}

		err := acl.AuthMethodSet(&req, &resp)
		require.Error(t, err)
	})

	t.Run("Create with invalid type", func(t *testing.T) {
		req := structs.ACLAuthMethodSetRequest{
			Datacenter: "dc1",
			AuthMethod: structs.ACLAuthMethod{
				Name:        "invalid",
				Description: "invalid test",
				Type:        "invalid",
			},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}
		resp := structs.ACLAuthMethod{}

		err := acl.AuthMethodSet(&req, &resp)
		require.Error(t, err)
	})

	for _, test := range []struct {
		name string
		ok   bool
	}{
		{strings.Repeat("x", 129), false},
		{strings.Repeat("x", 128), true},
		{"-abc", true},
		{"abc-", true},
		{"a-bc", true},
		{"_abc", true},
		{"abc_", true},
		{"a_bc", true},
		{":abc", false},
		{"abc:", false},
		{"a:bc", false},
		{"Abc", true},
		{"aBc", true},
		{"abC", true},
		{"0abc", true},
		{"abc0", true},
		{"a0bc", true},
	} {
		var testName string
		if test.ok {
			testName = "Create with valid name (by regex): " + test.name
		} else {
			testName = "Create with invalid name (by regex): " + test.name
		}
		t.Run(testName, func(t *testing.T) {
			req := structs.ACLAuthMethodSetRequest{
				Datacenter:   "dc1",
				AuthMethod:   newAuthMethod(test.name),
				WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
			}
			resp := structs.ACLAuthMethod{}

			err := acl.AuthMethodSet(&req, &resp)

			if test.ok {
				require.NoError(t, err)

				// Get the method directly to validate that it exists
				methodResp, err := retrieveTestAuthMethod(codec, TestDefaultInitialManagementToken, "dc1", resp.Name)
				require.NoError(t, err)
				method := methodResp.AuthMethod

				require.Equal(t, method.Name, test.name)
				require.Equal(t, method.Type, "testing")
			} else {
				require.Error(t, err)
			}
		})
	}

	t.Run("Create with MaxTokenTTL", func(t *testing.T) {
		reqMethod := newAuthMethod("test")
		reqMethod.MaxTokenTTL = 5 * time.Minute

		req := structs.ACLAuthMethodSetRequest{
			Datacenter:   "dc1",
			AuthMethod:   reqMethod,
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}
		resp := structs.ACLAuthMethod{}

		err := acl.AuthMethodSet(&req, &resp)
		require.NoError(t, err)

		// Get the method directly to validate that it exists
		methodResp, err := retrieveTestAuthMethod(codec, TestDefaultInitialManagementToken, "dc1", resp.Name)
		require.NoError(t, err)
		method := methodResp.AuthMethod

		require.Equal(t, method.Name, "test")
		require.Equal(t, method.Description, "test")
		require.Equal(t, method.Type, "testing")
		require.Equal(t, method.MaxTokenTTL, 5*time.Minute)
	})

	t.Run("Update - change MaxTokenTTL", func(t *testing.T) {
		reqMethod := newAuthMethod("test")
		reqMethod.DisplayName = "updated display name 2"
		reqMethod.Description = "test modified 2"
		reqMethod.MaxTokenTTL = 8 * time.Minute

		req := structs.ACLAuthMethodSetRequest{
			Datacenter:   "dc1",
			AuthMethod:   reqMethod,
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}
		resp := structs.ACLAuthMethod{}

		err := acl.AuthMethodSet(&req, &resp)
		require.NoError(t, err)

		// Get the method directly to validate that it exists
		methodResp, err := retrieveTestAuthMethod(codec, TestDefaultInitialManagementToken, "dc1", resp.Name)
		require.NoError(t, err)
		method := methodResp.AuthMethod

		require.Equal(t, method.Name, "test")
		require.Equal(t, method.DisplayName, "updated display name 2")
		require.Equal(t, method.Description, "test modified 2")
		require.Equal(t, method.Type, "testing")
		require.Equal(t, method.MaxTokenTTL, 8*time.Minute)
	})

	t.Run("Create with MaxTokenTTL too small", func(t *testing.T) {
		reqMethod := newAuthMethod("test")
		reqMethod.MaxTokenTTL = 1 * time.Millisecond

		req := structs.ACLAuthMethodSetRequest{
			Datacenter:   "dc1",
			AuthMethod:   reqMethod,
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}
		resp := structs.ACLAuthMethod{}

		err := acl.AuthMethodSet(&req, &resp)
		testutil.RequireErrorContains(t, err, "MaxTokenTTL 1ms cannot be less than")
	})

	t.Run("Create with MaxTokenTTL too big", func(t *testing.T) {
		reqMethod := newAuthMethod("test")
		reqMethod.MaxTokenTTL = 25 * time.Hour

		req := structs.ACLAuthMethodSetRequest{
			Datacenter:   "dc1",
			AuthMethod:   reqMethod,
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}
		resp := structs.ACLAuthMethod{}

		err := acl.AuthMethodSet(&req, &resp)
		testutil.RequireErrorContains(t, err, "MaxTokenTTL 25h0m0s cannot be more than")
	})
}

func TestACLEndpoint_AuthMethodDelete(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	_, srv, codec := testACLServerWithConfig(t, nil, false)
	waitForLeaderEstablishment(t, srv)

	testSessionID := testauth.StartSession()
	defer testauth.ResetSession(testSessionID)

	existingMethod, err := upsertTestAuthMethod(codec, TestDefaultInitialManagementToken, "dc1", testSessionID)
	require.NoError(t, err)

	acl := ACL{srv: srv}

	t.Run("normal", func(t *testing.T) {
		req := structs.ACLAuthMethodDeleteRequest{
			Datacenter:     "dc1",
			AuthMethodName: existingMethod.Name,
			WriteRequest:   structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		var ignored bool
		err = acl.AuthMethodDelete(&req, &ignored)
		require.NoError(t, err)

		// Make sure the method is gone
		methodResp, err := retrieveTestAuthMethod(codec, TestDefaultInitialManagementToken, "dc1", existingMethod.Name)
		require.NoError(t, err)
		require.Nil(t, methodResp.AuthMethod)
	})

	t.Run("delete something that doesn't exist", func(t *testing.T) {
		req := structs.ACLAuthMethodDeleteRequest{
			Datacenter:     "dc1",
			AuthMethodName: "missing",
			WriteRequest:   structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		var ignored bool
		err = acl.AuthMethodDelete(&req, &ignored)
		require.NoError(t, err)
	})
}

// Deleting an auth method atomically deletes all rules and tokens as well.
func TestACLEndpoint_AuthMethodDelete_RuleAndTokenCascade(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	_, srv, codec := testACLServerWithConfig(t, nil, false)
	waitForLeaderEstablishment(t, srv)

	testSessionID1 := testauth.StartSession()
	defer testauth.ResetSession(testSessionID1)
	testauth.InstallSessionToken(testSessionID1, "fake-token1", "default", "abc", "abc123")

	testSessionID2 := testauth.StartSession()
	defer testauth.ResetSession(testSessionID2)
	testauth.InstallSessionToken(testSessionID2, "fake-token2", "default", "abc", "abc123")

	createToken := func(methodName, bearerToken string) *structs.ACLToken {
		acl := ACL{srv: srv}

		resp := structs.ACLToken{}

		require.NoError(t, acl.Login(&structs.ACLLoginRequest{
			Auth: &structs.ACLLoginParams{
				AuthMethod:  methodName,
				BearerToken: bearerToken,
			},
			Datacenter: "dc1",
		}, &resp))

		return &resp
	}

	method1, err := upsertTestAuthMethod(codec, TestDefaultInitialManagementToken, "dc1", testSessionID1)
	require.NoError(t, err)
	i1_r1, err := upsertTestBindingRule(
		codec, TestDefaultInitialManagementToken, "dc1",
		method1.Name,
		"serviceaccount.name==abc",
		structs.BindingRuleBindTypeService,
		"abc",
	)
	require.NoError(t, err)
	i1_r2, err := upsertTestBindingRule(
		codec, TestDefaultInitialManagementToken, "dc1",
		method1.Name,
		"serviceaccount.name==def",
		structs.BindingRuleBindTypeService,
		"def",
	)
	require.NoError(t, err)
	i1_t1 := createToken(method1.Name, "fake-token1")
	i1_t2 := createToken(method1.Name, "fake-token1")

	method2, err := upsertTestAuthMethod(codec, TestDefaultInitialManagementToken, "dc1", testSessionID2)
	require.NoError(t, err)
	i2_r1, err := upsertTestBindingRule(
		codec, TestDefaultInitialManagementToken, "dc1",
		method2.Name,
		"serviceaccount.name==abc",
		structs.BindingRuleBindTypeService,
		"abc",
	)
	require.NoError(t, err)
	i2_r2, err := upsertTestBindingRule(
		codec, TestDefaultInitialManagementToken, "dc1",
		method2.Name,
		"serviceaccount.name==def",
		structs.BindingRuleBindTypeService,
		"def",
	)
	require.NoError(t, err)
	i2_t1 := createToken(method2.Name, "fake-token2")
	i2_t2 := createToken(method2.Name, "fake-token2")

	acl := ACL{srv: srv}

	req := structs.ACLAuthMethodDeleteRequest{
		Datacenter:     "dc1",
		AuthMethodName: method1.Name,
		WriteRequest:   structs.WriteRequest{Token: TestDefaultInitialManagementToken},
	}

	var ignored bool
	err = acl.AuthMethodDelete(&req, &ignored)
	require.NoError(t, err)

	// Make sure the method is gone.
	methodResp, err := retrieveTestAuthMethod(codec, TestDefaultInitialManagementToken, "dc1", method1.Name)
	require.NoError(t, err)
	require.Nil(t, methodResp.AuthMethod)

	// Make sure the rules and tokens are gone.
	for _, id := range []string{i1_r1.ID, i1_r2.ID} {
		ruleResp, err := retrieveTestBindingRule(codec, TestDefaultInitialManagementToken, "dc1", id)
		require.NoError(t, err)
		require.Nil(t, ruleResp.BindingRule)
	}
	for _, id := range []string{i1_t1.AccessorID, i1_t2.AccessorID} {
		tokResp, err := retrieveTestToken(codec, TestDefaultInitialManagementToken, "dc1", id)
		require.NoError(t, err)
		require.Nil(t, tokResp.Token)
	}

	// Make sure the rules and tokens for the untouched auth method are still there.
	for _, id := range []string{i2_r1.ID, i2_r2.ID} {
		ruleResp, err := retrieveTestBindingRule(codec, TestDefaultInitialManagementToken, "dc1", id)
		require.NoError(t, err)
		require.NotNil(t, ruleResp.BindingRule)
	}
	for _, id := range []string{i2_t1.AccessorID, i2_t2.AccessorID} {
		tokResp, err := retrieveTestToken(codec, TestDefaultInitialManagementToken, "dc1", id)
		require.NoError(t, err)
		require.NotNil(t, tokResp.Token)
	}
}

func TestACLEndpoint_AuthMethodList(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	_, srv, codec := testACLServerWithConfig(t, nil, false)
	waitForLeaderEstablishment(t, srv)

	i1, err := upsertTestAuthMethod(codec, TestDefaultInitialManagementToken, "dc1", "")
	require.NoError(t, err)

	i2, err := upsertTestAuthMethod(codec, TestDefaultInitialManagementToken, "dc1", "")
	require.NoError(t, err)

	acl := ACL{srv: srv}

	req := structs.ACLAuthMethodListRequest{
		Datacenter:   "dc1",
		QueryOptions: structs.QueryOptions{Token: TestDefaultInitialManagementToken},
	}

	resp := structs.ACLAuthMethodListResponse{}

	err = acl.AuthMethodList(&req, &resp)
	require.NoError(t, err)
	require.ElementsMatch(t, gatherIDs(t, resp.AuthMethods), []string{i1.Name, i2.Name})
}

func TestACLEndpoint_BindingRuleSet(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	_, srv, codec := testACLServerWithConfig(t, nil, false)
	waitForLeaderEstablishment(t, srv)
	acl := ACL{srv: srv}

	var ruleID string

	testAuthMethod, err := upsertTestAuthMethod(codec, TestDefaultInitialManagementToken, "dc1", "")
	require.NoError(t, err)

	otherTestAuthMethod, err := upsertTestAuthMethod(codec, TestDefaultInitialManagementToken, "dc1", "")
	require.NoError(t, err)

	newRule := func() structs.ACLBindingRule {
		return structs.ACLBindingRule{
			Description: "foobar",
			AuthMethod:  testAuthMethod.Name,
			Selector:    "serviceaccount.name==abc",
			BindType:    structs.BindingRuleBindTypeService,
			BindName:    "abc",
		}
	}

	requireSetErrors := func(t *testing.T, reqRule structs.ACLBindingRule) {
		req := structs.ACLBindingRuleSetRequest{
			Datacenter:   "dc1",
			BindingRule:  reqRule,
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}
		resp := structs.ACLBindingRule{}

		err := acl.BindingRuleSet(&req, &resp)
		require.Error(t, err)
	}

	requireOK := func(t *testing.T, reqRule structs.ACLBindingRule) *structs.ACLBindingRule {
		req := structs.ACLBindingRuleSetRequest{
			Datacenter:   "dc1",
			BindingRule:  reqRule,
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}
		resp := structs.ACLBindingRule{}

		err := acl.BindingRuleSet(&req, &resp)
		require.NoError(t, err)
		require.NotEmpty(t, resp.ID)
		return &resp
	}

	t.Run("Create it", func(t *testing.T) {
		reqRule := newRule()

		req := structs.ACLBindingRuleSetRequest{
			Datacenter:   "dc1",
			BindingRule:  reqRule,
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}
		resp := structs.ACLBindingRule{}

		err := acl.BindingRuleSet(&req, &resp)
		require.NoError(t, err)
		require.NotNil(t, resp.ID)

		// Get the rule directly to validate that it exists
		ruleResp, err := retrieveTestBindingRule(codec, TestDefaultInitialManagementToken, "dc1", resp.ID)
		require.NoError(t, err)
		rule := ruleResp.BindingRule

		require.NotEmpty(t, rule.ID)
		require.Equal(t, rule.Description, "foobar")
		require.Equal(t, rule.AuthMethod, testAuthMethod.Name)
		require.Equal(t, "serviceaccount.name==abc", rule.Selector)
		require.Equal(t, structs.BindingRuleBindTypeService, rule.BindType)
		require.Equal(t, "abc", rule.BindName)

		ruleID = rule.ID
	})

	t.Run("Bind Node", func(t *testing.T) {
		req := structs.ACLBindingRuleSetRequest{
			Datacenter: "dc1",
			BindingRule: structs.ACLBindingRule{
				Description: "foobar",
				AuthMethod:  testAuthMethod.Name,
				Selector:    "serviceaccount.name==abc",
				BindType:    structs.BindingRuleBindTypeNode,
				BindName:    "test-node",
			},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}
		var resp structs.ACLBindingRule

		err := acl.BindingRuleSet(&req, &resp)
		require.NoError(t, err)
		require.NotNil(t, resp.ID)

		// Get the rule directly to validate that it exists
		ruleResp, err := retrieveTestBindingRule(codec, TestDefaultInitialManagementToken, "dc1", resp.ID)
		require.NoError(t, err)
		rule := ruleResp.BindingRule

		require.NotEmpty(t, rule.ID)
		require.Equal(t, rule.Description, "foobar")
		require.Equal(t, rule.AuthMethod, testAuthMethod.Name)
		require.Equal(t, "serviceaccount.name==abc", rule.Selector)
		require.Equal(t, structs.BindingRuleBindTypeNode, rule.BindType)
		require.Equal(t, "test-node", rule.BindName)
	})

	t.Run("Update fails; cannot change method name", func(t *testing.T) {
		reqRule := newRule()
		reqRule.ID = ruleID
		reqRule.AuthMethod = otherTestAuthMethod.Name
		requireSetErrors(t, reqRule)
	})

	t.Run("Update it - omit method name", func(t *testing.T) {
		reqRule := newRule()
		reqRule.ID = ruleID
		reqRule.Description = "foobar modified 1"
		reqRule.Selector = "serviceaccount.namespace==def"
		reqRule.BindType = structs.BindingRuleBindTypeRole
		reqRule.BindName = "def"
		reqRule.AuthMethod = "" // clear

		req := structs.ACLBindingRuleSetRequest{
			Datacenter:   "dc1",
			BindingRule:  reqRule,
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}
		resp := structs.ACLBindingRule{}

		err := acl.BindingRuleSet(&req, &resp)
		require.NoError(t, err)
		require.NotNil(t, resp.ID)

		// Get the rule directly to validate that it exists
		ruleResp, err := retrieveTestBindingRule(codec, TestDefaultInitialManagementToken, "dc1", resp.ID)
		require.NoError(t, err)
		rule := ruleResp.BindingRule

		require.NotEmpty(t, rule.ID)
		require.Equal(t, rule.Description, "foobar modified 1")
		require.Equal(t, rule.AuthMethod, testAuthMethod.Name)
		require.Equal(t, "serviceaccount.namespace==def", rule.Selector)
		require.Equal(t, structs.BindingRuleBindTypeRole, rule.BindType)
		require.Equal(t, "def", rule.BindName)
	})

	t.Run("Update it - specify method name", func(t *testing.T) {
		reqRule := newRule()
		reqRule.ID = ruleID
		reqRule.Description = "foobar modified 2"
		reqRule.Selector = "serviceaccount.namespace==def"
		reqRule.BindType = structs.BindingRuleBindTypeRole
		reqRule.BindName = "def"

		req := structs.ACLBindingRuleSetRequest{
			Datacenter:   "dc1",
			BindingRule:  reqRule,
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}
		resp := structs.ACLBindingRule{}

		err := acl.BindingRuleSet(&req, &resp)
		require.NoError(t, err)
		require.NotNil(t, resp.ID)

		// Get the rule directly to validate that it exists
		ruleResp, err := retrieveTestBindingRule(codec, TestDefaultInitialManagementToken, "dc1", resp.ID)
		require.NoError(t, err)
		rule := ruleResp.BindingRule

		require.NotEmpty(t, rule.ID)
		require.Equal(t, rule.Description, "foobar modified 2")
		require.Equal(t, rule.AuthMethod, testAuthMethod.Name)
		require.Equal(t, "serviceaccount.namespace==def", rule.Selector)
		require.Equal(t, structs.BindingRuleBindTypeRole, rule.BindType)
		require.Equal(t, "def", rule.BindName)
	})

	t.Run("Create fails; empty method name", func(t *testing.T) {
		reqRule := newRule()
		reqRule.AuthMethod = ""
		requireSetErrors(t, reqRule)
	})

	t.Run("Create fails; unknown method name", func(t *testing.T) {
		reqRule := newRule()
		reqRule.AuthMethod = "unknown"
		requireSetErrors(t, reqRule)
	})

	t.Run("Create with no explicit selector", func(t *testing.T) {
		reqRule := newRule()
		reqRule.Selector = ""

		rule := requireOK(t, reqRule)
		require.Empty(t, rule.Selector, 0)
	})

	t.Run("Create fails; match selector with unknown vars", func(t *testing.T) {
		reqRule := newRule()
		reqRule.Selector = "serviceaccount.name==a and serviceaccount.bizarroname==b"
		requireSetErrors(t, reqRule)
	})

	t.Run("Create fails; match selector invalid", func(t *testing.T) {
		reqRule := newRule()
		reqRule.Selector = "serviceaccount.name"
		requireSetErrors(t, reqRule)
	})

	t.Run("Create fails; empty bind type", func(t *testing.T) {
		reqRule := newRule()
		reqRule.BindType = ""
		requireSetErrors(t, reqRule)
	})

	t.Run("Create fails; empty bind name", func(t *testing.T) {
		reqRule := newRule()
		reqRule.BindName = ""
		requireSetErrors(t, reqRule)
	})

	t.Run("Create fails; invalid bind type", func(t *testing.T) {
		reqRule := newRule()
		reqRule.BindType = "invalid"
		requireSetErrors(t, reqRule)
	})

	t.Run("Create fails; bind name with unknown vars", func(t *testing.T) {
		reqRule := newRule()
		reqRule.BindName = "method-${serviceaccount.bizarroname}"
		requireSetErrors(t, reqRule)
	})

	t.Run("Create fails; invalid bind name no template", func(t *testing.T) {
		reqRule := newRule()
		reqRule.BindName = "-abc:"
		requireSetErrors(t, reqRule)
	})

	t.Run("Create fails; invalid bind name with template", func(t *testing.T) {
		reqRule := newRule()
		reqRule.BindName = "method-${serviceaccount.name"
		requireSetErrors(t, reqRule)
	})
	t.Run("Create fails; invalid bind name after template computed", func(t *testing.T) {
		reqRule := newRule()
		reqRule.BindName = "method-${serviceaccount.name}:blah-"
		requireSetErrors(t, reqRule)
	})
}

func TestACLEndpoint_BindingRuleDelete(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	_, srv, codec := testACLServerWithConfig(t, nil, false)
	waitForLeaderEstablishment(t, srv)

	testAuthMethod, err := upsertTestAuthMethod(codec, TestDefaultInitialManagementToken, "dc1", "")
	require.NoError(t, err)

	existingRule, err := upsertTestBindingRule(
		codec, TestDefaultInitialManagementToken, "dc1",
		testAuthMethod.Name,
		"serviceaccount.name==abc",
		structs.BindingRuleBindTypeService,
		"abc",
	)
	require.NoError(t, err)

	acl := ACL{srv: srv}

	t.Run("normal", func(t *testing.T) {
		req := structs.ACLBindingRuleDeleteRequest{
			Datacenter:    "dc1",
			BindingRuleID: existingRule.ID,
			WriteRequest:  structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		var ignored bool
		err = acl.BindingRuleDelete(&req, &ignored)
		require.NoError(t, err)

		// Make sure the rule is gone
		ruleResp, err := retrieveTestBindingRule(codec, TestDefaultInitialManagementToken, "dc1", existingRule.ID)
		require.NoError(t, err)
		require.Nil(t, ruleResp.BindingRule)
	})

	t.Run("delete something that doesn't exist", func(t *testing.T) {
		fakeID, err := uuid.GenerateUUID()
		require.NoError(t, err)

		req := structs.ACLBindingRuleDeleteRequest{
			Datacenter:    "dc1",
			BindingRuleID: fakeID,
			WriteRequest:  structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		var ignored bool
		err = acl.BindingRuleDelete(&req, &ignored)
		require.NoError(t, err)
	})
}

func TestACLEndpoint_BindingRuleList(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	_, srv, codec := testACLServerWithConfig(t, nil, false)
	waitForLeaderEstablishment(t, srv)

	testAuthMethod, err := upsertTestAuthMethod(codec, TestDefaultInitialManagementToken, "dc1", "")
	require.NoError(t, err)

	r1, err := upsertTestBindingRule(
		codec, TestDefaultInitialManagementToken, "dc1",
		testAuthMethod.Name,
		"serviceaccount.name==abc",
		structs.BindingRuleBindTypeService,
		"abc",
	)
	require.NoError(t, err)

	r2, err := upsertTestBindingRule(
		codec, TestDefaultInitialManagementToken, "dc1",
		testAuthMethod.Name,
		"serviceaccount.name==def",
		structs.BindingRuleBindTypeService,
		"def",
	)
	require.NoError(t, err)

	acl := ACL{srv: srv}

	req := structs.ACLBindingRuleListRequest{
		Datacenter:   "dc1",
		QueryOptions: structs.QueryOptions{Token: TestDefaultInitialManagementToken},
	}

	resp := structs.ACLBindingRuleListResponse{}

	err = acl.BindingRuleList(&req, &resp)
	require.NoError(t, err)
	require.ElementsMatch(t, gatherIDs(t, resp.BindingRules), []string{r1.ID, r2.ID})
}

func TestACLEndpoint_SecureIntroEndpoints_LocalTokensDisabled(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	_, s1, _ := testACLServerWithConfig(t, func(c *Config) {
		c.ACLTokenMinExpirationTTL = 10 * time.Millisecond
		c.ACLTokenMaxExpirationTTL = 5 * time.Second
	}, false)
	waitForLeaderEstablishment(t, s1)

	_, s2, _ := testACLServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc2"
		c.ACLTokenMinExpirationTTL = 10 * time.Millisecond
		c.ACLTokenMaxExpirationTTL = 5 * time.Second
		// disable local tokens
		c.ACLTokenReplication = false
	}, true)
	waitForLeaderEstablishment(t, s2)

	// Try to join
	joinWAN(t, s2, s1)

	acl2 := ACL{srv: s2}
	var ignored bool

	errString := errAuthMethodsRequireTokenReplication.Error()

	t.Run("AuthMethodRead", func(t *testing.T) {
		testutil.RequireErrorContains(t,
			acl2.AuthMethodRead(&structs.ACLAuthMethodGetRequest{Datacenter: "dc2"},
				&structs.ACLAuthMethodResponse{}),
			errString,
		)
	})
	t.Run("AuthMethodSet", func(t *testing.T) {
		testutil.RequireErrorContains(t,
			acl2.AuthMethodSet(&structs.ACLAuthMethodSetRequest{Datacenter: "dc2"},
				&structs.ACLAuthMethod{}),
			errString,
		)
	})
	t.Run("AuthMethodDelete", func(t *testing.T) {
		testutil.RequireErrorContains(t,
			acl2.AuthMethodDelete(&structs.ACLAuthMethodDeleteRequest{Datacenter: "dc2"}, &ignored),
			errString,
		)
	})
	t.Run("AuthMethodList", func(t *testing.T) {
		testutil.RequireErrorContains(t,
			acl2.AuthMethodList(&structs.ACLAuthMethodListRequest{Datacenter: "dc2"},
				&structs.ACLAuthMethodListResponse{}),
			errString,
		)
	})

	t.Run("BindingRuleRead", func(t *testing.T) {
		testutil.RequireErrorContains(t,
			acl2.BindingRuleRead(&structs.ACLBindingRuleGetRequest{Datacenter: "dc2"},
				&structs.ACLBindingRuleResponse{}),
			errString,
		)
	})
	t.Run("BindingRuleSet", func(t *testing.T) {
		testutil.RequireErrorContains(t,
			acl2.BindingRuleSet(&structs.ACLBindingRuleSetRequest{Datacenter: "dc2"},
				&structs.ACLBindingRule{}),
			errString,
		)
	})
	t.Run("BindingRuleDelete", func(t *testing.T) {
		testutil.RequireErrorContains(t,
			acl2.BindingRuleDelete(&structs.ACLBindingRuleDeleteRequest{Datacenter: "dc2"}, &ignored),
			errString,
		)
	})
	t.Run("BindingRuleList", func(t *testing.T) {
		testutil.RequireErrorContains(t,
			acl2.BindingRuleList(&structs.ACLBindingRuleListRequest{Datacenter: "dc2"},
				&structs.ACLBindingRuleListResponse{}),
			errString,
		)
	})

	t.Run("Login", func(t *testing.T) {
		testutil.RequireErrorContains(t,
			acl2.Login(&structs.ACLLoginRequest{Datacenter: "dc2"},
				&structs.ACLToken{}),
			errString,
		)
	})
	t.Run("Logout", func(t *testing.T) {
		testutil.RequireErrorContains(t,
			acl2.Logout(&structs.ACLLogoutRequest{Datacenter: "dc2"}, &ignored),
			errString,
		)
	})
}

func TestACLEndpoint_SecureIntroEndpoints_OnlyCreateLocalData(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	_, s1, codec1 := testACLServerWithConfig(t, func(c *Config) {
		c.ACLTokenMinExpirationTTL = 10 * time.Millisecond
		c.ACLTokenMaxExpirationTTL = 5 * time.Second
	}, false)
	waitForLeaderEstablishment(t, s1)

	_, s2, codec2 := testACLServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc2"
		c.ACLTokenMinExpirationTTL = 10 * time.Millisecond
		c.ACLTokenMaxExpirationTTL = 5 * time.Second
		// enable token replication so secure intro works
		c.ACLTokenReplication = true
	}, true)
	waitForLeaderEstablishment(t, s2)

	// Try to join
	joinWAN(t, s2, s1)

	// Ensure s2 is authoritative.
	waitForNewACLReplication(t, s2, structs.ACLReplicateTokens, 1, 1, 0)

	acl := ACL{srv: s1}
	acl2 := ACL{srv: s2}

	//
	// this order is specific so that we can do it in one pass
	//

	testSessionID_1 := testauth.StartSession()
	defer testauth.ResetSession(testSessionID_1)

	testSessionID_2 := testauth.StartSession()
	defer testauth.ResetSession(testSessionID_2)

	testauth.InstallSessionToken(
		testSessionID_1,
		"fake-web1-token",
		"default", "web1", "abc123",
	)
	testauth.InstallSessionToken(
		testSessionID_2,
		"fake-web2-token",
		"default", "web2", "def456",
	)

	t.Run("create auth method", func(t *testing.T) {
		req := structs.ACLAuthMethodSetRequest{
			Datacenter: "dc2",
			AuthMethod: structs.ACLAuthMethod{
				Name:        "testmethod",
				Description: "test original",
				Type:        "testing",
				Config: map[string]interface{}{
					"SessionID": testSessionID_2,
				},
			},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}
		resp := structs.ACLAuthMethod{}

		require.NoError(t, acl2.AuthMethodSet(&req, &resp))

		// present in dc2
		resp2, err := retrieveTestAuthMethod(codec2, TestDefaultInitialManagementToken, "dc2", "testmethod")
		require.NoError(t, err)
		require.NotNil(t, resp2.AuthMethod)
		require.Equal(t, "test original", resp2.AuthMethod.Description)
		// absent in dc1
		resp2, err = retrieveTestAuthMethod(codec1, TestDefaultInitialManagementToken, "dc1", "testmethod")
		require.NoError(t, err)
		require.Nil(t, resp2.AuthMethod)
	})

	t.Run("update auth method", func(t *testing.T) {
		req := structs.ACLAuthMethodSetRequest{
			Datacenter: "dc2",
			AuthMethod: structs.ACLAuthMethod{
				Name:        "testmethod",
				Description: "test updated",
				Config: map[string]interface{}{
					"SessionID": testSessionID_2,
				},
			},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}
		resp := structs.ACLAuthMethod{}

		require.NoError(t, acl2.AuthMethodSet(&req, &resp))

		// present in dc2
		resp2, err := retrieveTestAuthMethod(codec2, TestDefaultInitialManagementToken, "dc2", "testmethod")
		require.NoError(t, err)
		require.NotNil(t, resp2.AuthMethod)
		require.Equal(t, "test updated", resp2.AuthMethod.Description)
		// absent in dc1
		resp2, err = retrieveTestAuthMethod(codec1, TestDefaultInitialManagementToken, "dc1", "testmethod")
		require.NoError(t, err)
		require.Nil(t, resp2.AuthMethod)
	})

	t.Run("read auth method", func(t *testing.T) {
		// present in dc2
		req := structs.ACLAuthMethodGetRequest{
			Datacenter:     "dc2",
			AuthMethodName: "testmethod",
			QueryOptions:   structs.QueryOptions{Token: TestDefaultInitialManagementToken},
		}
		resp := structs.ACLAuthMethodResponse{}
		require.NoError(t, acl2.AuthMethodRead(&req, &resp))
		require.NotNil(t, resp.AuthMethod)
		require.Equal(t, "test updated", resp.AuthMethod.Description)

		// absent in dc1
		req = structs.ACLAuthMethodGetRequest{
			Datacenter:     "dc1",
			AuthMethodName: "testmethod",
			QueryOptions:   structs.QueryOptions{Token: TestDefaultInitialManagementToken},
		}
		resp = structs.ACLAuthMethodResponse{}
		require.NoError(t, acl.AuthMethodRead(&req, &resp))
		require.Nil(t, resp.AuthMethod)
	})

	t.Run("list auth method", func(t *testing.T) {
		// present in dc2
		req := structs.ACLAuthMethodListRequest{
			Datacenter:   "dc2",
			QueryOptions: structs.QueryOptions{Token: TestDefaultInitialManagementToken},
		}
		resp := structs.ACLAuthMethodListResponse{}
		require.NoError(t, acl2.AuthMethodList(&req, &resp))
		require.Len(t, resp.AuthMethods, 1)

		// absent in dc1
		req = structs.ACLAuthMethodListRequest{
			Datacenter:   "dc1",
			QueryOptions: structs.QueryOptions{Token: TestDefaultInitialManagementToken},
		}
		resp = structs.ACLAuthMethodListResponse{}
		require.NoError(t, acl.AuthMethodList(&req, &resp))
		require.Len(t, resp.AuthMethods, 0)
	})

	var ruleID string
	t.Run("create binding rule", func(t *testing.T) {
		req := structs.ACLBindingRuleSetRequest{
			Datacenter: "dc2",
			BindingRule: structs.ACLBindingRule{
				Description: "test original",
				AuthMethod:  "testmethod",
				BindType:    structs.BindingRuleBindTypeService,
				BindName:    "${serviceaccount.name}",
			},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		resp := structs.ACLBindingRule{}

		require.NoError(t, acl2.BindingRuleSet(&req, &resp))
		ruleID = resp.ID

		// present in dc2
		resp2, err := retrieveTestBindingRule(codec2, TestDefaultInitialManagementToken, "dc2", ruleID)
		require.NoError(t, err)
		require.NotNil(t, resp2.BindingRule)
		require.Equal(t, "test original", resp2.BindingRule.Description)
		// absent in dc1
		resp2, err = retrieveTestBindingRule(codec1, TestDefaultInitialManagementToken, "dc1", ruleID)
		require.NoError(t, err)
		require.Nil(t, resp2.BindingRule)
	})

	t.Run("update binding rule", func(t *testing.T) {
		req := structs.ACLBindingRuleSetRequest{
			Datacenter: "dc2",
			BindingRule: structs.ACLBindingRule{
				ID:          ruleID,
				Description: "test updated",
				AuthMethod:  "testmethod",
				BindType:    structs.BindingRuleBindTypeService,
				BindName:    "${serviceaccount.name}",
			},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		resp := structs.ACLBindingRule{}

		require.NoError(t, acl2.BindingRuleSet(&req, &resp))
		ruleID = resp.ID

		// present in dc2
		resp2, err := retrieveTestBindingRule(codec2, TestDefaultInitialManagementToken, "dc2", ruleID)
		require.NoError(t, err)
		require.NotNil(t, resp2.BindingRule)
		require.Equal(t, "test updated", resp2.BindingRule.Description)
		// absent in dc1
		resp2, err = retrieveTestBindingRule(codec1, TestDefaultInitialManagementToken, "dc1", ruleID)
		require.NoError(t, err)
		require.Nil(t, resp2.BindingRule)
	})

	t.Run("read binding rule", func(t *testing.T) {
		// present in dc2
		req := structs.ACLBindingRuleGetRequest{
			Datacenter:    "dc2",
			BindingRuleID: ruleID,
			QueryOptions:  structs.QueryOptions{Token: TestDefaultInitialManagementToken},
		}
		resp := structs.ACLBindingRuleResponse{}
		require.NoError(t, acl2.BindingRuleRead(&req, &resp))
		require.NotNil(t, resp.BindingRule)
		require.Equal(t, "test updated", resp.BindingRule.Description)

		// absent in dc1
		req = structs.ACLBindingRuleGetRequest{
			Datacenter:    "dc1",
			BindingRuleID: ruleID,
			QueryOptions:  structs.QueryOptions{Token: TestDefaultInitialManagementToken},
		}
		resp = structs.ACLBindingRuleResponse{}
		require.NoError(t, acl.BindingRuleRead(&req, &resp))
		require.Nil(t, resp.BindingRule)
	})

	t.Run("list binding rule", func(t *testing.T) {
		// present in dc2
		req := structs.ACLBindingRuleListRequest{
			Datacenter:   "dc2",
			QueryOptions: structs.QueryOptions{Token: TestDefaultInitialManagementToken},
		}
		resp := structs.ACLBindingRuleListResponse{}
		require.NoError(t, acl2.BindingRuleList(&req, &resp))
		require.Len(t, resp.BindingRules, 1)

		// absent in dc1
		req = structs.ACLBindingRuleListRequest{
			Datacenter:   "dc1",
			QueryOptions: structs.QueryOptions{Token: TestDefaultInitialManagementToken},
		}
		resp = structs.ACLBindingRuleListResponse{}
		require.NoError(t, acl.BindingRuleList(&req, &resp))
		require.Len(t, resp.BindingRules, 0)
	})

	var remoteToken *structs.ACLToken
	t.Run("login in remote", func(t *testing.T) {
		req := structs.ACLLoginRequest{
			Datacenter: "dc2",
			Auth: &structs.ACLLoginParams{
				AuthMethod:  "testmethod",
				BearerToken: "fake-web2-token",
			},
		}
		resp := structs.ACLToken{}

		require.NoError(t, acl2.Login(&req, &resp))
		remoteToken = &resp

		// present in dc2
		resp2, err := retrieveTestToken(codec2, TestDefaultInitialManagementToken, "dc2", remoteToken.AccessorID)
		require.NoError(t, err)
		require.NotNil(t, resp2.Token)
		require.Len(t, resp2.Token.ServiceIdentities, 1)
		require.Equal(t, "web2", resp2.Token.ServiceIdentities[0].ServiceName)
		// absent in dc1
		resp2, err = retrieveTestToken(codec1, TestDefaultInitialManagementToken, "dc1", remoteToken.AccessorID)
		require.NoError(t, err)
		require.Nil(t, resp2.Token)
	})

	// We delay until now to setup an auth method and binding rule in the
	// primary so our earlier listing tests were reasonable. We need to be able to
	// use auth methods in both datacenters in order to verify Logout is
	// properly scoped.
	t.Run("initialize primary so we can test logout", func(t *testing.T) {
		reqAM := structs.ACLAuthMethodSetRequest{
			Datacenter: "dc1",
			AuthMethod: structs.ACLAuthMethod{
				Name: "primarymethod",
				Type: "testing",
				Config: map[string]interface{}{
					"SessionID": testSessionID_1,
				},
			},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}
		respAM := structs.ACLAuthMethod{}
		require.NoError(t, acl.AuthMethodSet(&reqAM, &respAM))

		reqBR := structs.ACLBindingRuleSetRequest{
			Datacenter: "dc1",
			BindingRule: structs.ACLBindingRule{
				AuthMethod: "primarymethod",
				BindType:   structs.BindingRuleBindTypeService,
				BindName:   "${serviceaccount.name}",
			},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		respBR := structs.ACLBindingRule{}
		require.NoError(t, acl.BindingRuleSet(&reqBR, &respBR))
	})

	var primaryToken *structs.ACLToken
	t.Run("login in primary", func(t *testing.T) {
		req := structs.ACLLoginRequest{
			Datacenter: "dc1",
			Auth: &structs.ACLLoginParams{
				AuthMethod:  "primarymethod",
				BearerToken: "fake-web1-token",
			},
		}
		resp := structs.ACLToken{}

		require.NoError(t, acl.Login(&req, &resp))
		primaryToken = &resp

		// present in dc1
		resp2, err := retrieveTestToken(codec1, TestDefaultInitialManagementToken, "dc1", primaryToken.AccessorID)
		require.NoError(t, err)
		require.NotNil(t, resp2.Token)
		require.Len(t, resp2.Token.ServiceIdentities, 1)
		require.Equal(t, "web1", resp2.Token.ServiceIdentities[0].ServiceName)
		// absent in dc2
		resp2, err = retrieveTestToken(codec2, TestDefaultInitialManagementToken, "dc2", primaryToken.AccessorID)
		require.NoError(t, err)
		require.Nil(t, resp2.Token)
	})

	t.Run("logout of remote token in remote dc", func(t *testing.T) {
		// if the other test fails this one will to but will now not segfault
		require.NotNil(t, remoteToken)

		req := structs.ACLLogoutRequest{
			Datacenter:   "dc2",
			WriteRequest: structs.WriteRequest{Token: remoteToken.SecretID},
		}

		var ignored bool
		require.NoError(t, acl.Logout(&req, &ignored))

		// absent in dc2
		resp2, err := retrieveTestToken(codec2, TestDefaultInitialManagementToken, "dc2", remoteToken.AccessorID)
		require.NoError(t, err)
		require.Nil(t, resp2.Token)
		// absent in dc1
		resp2, err = retrieveTestToken(codec1, TestDefaultInitialManagementToken, "dc1", remoteToken.AccessorID)
		require.NoError(t, err)
		require.Nil(t, resp2.Token)
	})

	t.Run("logout of primary token in remote dc should not work", func(t *testing.T) {
		req := structs.ACLLogoutRequest{
			Datacenter:   "dc2",
			WriteRequest: structs.WriteRequest{Token: primaryToken.SecretID},
		}

		var ignored bool
		testutil.RequireErrorContains(t, acl.Logout(&req, &ignored), "ACL not found")

		// present in dc1
		resp2, err := retrieveTestToken(codec1, TestDefaultInitialManagementToken, "dc1", primaryToken.AccessorID)
		require.NoError(t, err)
		require.NotNil(t, resp2.Token)
		require.Len(t, resp2.Token.ServiceIdentities, 1)
		require.Equal(t, "web1", resp2.Token.ServiceIdentities[0].ServiceName)
		// absent in dc2
		resp2, err = retrieveTestToken(codec2, TestDefaultInitialManagementToken, "dc2", primaryToken.AccessorID)
		require.NoError(t, err)
		require.Nil(t, resp2.Token)
	})

	// Don't trigger the auth method delete cascade so we know the individual
	// endpoints follow the rules.

	t.Run("delete binding rule", func(t *testing.T) {
		req := structs.ACLBindingRuleDeleteRequest{
			Datacenter:    "dc2",
			BindingRuleID: ruleID,
			WriteRequest:  structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		var ignored bool
		require.NoError(t, acl2.BindingRuleDelete(&req, &ignored))

		// absent in dc2
		resp2, err := retrieveTestBindingRule(codec2, TestDefaultInitialManagementToken, "dc2", ruleID)
		require.NoError(t, err)
		require.Nil(t, resp2.BindingRule)
		// absent in dc1
		resp2, err = retrieveTestBindingRule(codec1, TestDefaultInitialManagementToken, "dc1", ruleID)
		require.NoError(t, err)
		require.Nil(t, resp2.BindingRule)
	})

	t.Run("delete auth method", func(t *testing.T) {
		req := structs.ACLAuthMethodDeleteRequest{
			Datacenter:     "dc2",
			AuthMethodName: "testmethod",
			WriteRequest:   structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		var ignored bool
		require.NoError(t, acl2.AuthMethodDelete(&req, &ignored))

		// absent in dc2
		resp2, err := retrieveTestAuthMethod(codec2, TestDefaultInitialManagementToken, "dc2", "testmethod")
		require.NoError(t, err)
		require.Nil(t, resp2.AuthMethod)
		// absent in dc1
		resp2, err = retrieveTestAuthMethod(codec1, TestDefaultInitialManagementToken, "dc1", "testmethod")
		require.NoError(t, err)
		require.Nil(t, resp2.AuthMethod)
	})
}

func TestACLEndpoint_Login(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	_, srv, codec := testACLServerWithConfig(t, nil, false)
	waitForLeaderEstablishment(t, srv)

	acl := ACL{srv: srv}

	testSessionID := testauth.StartSession()
	defer testauth.ResetSession(testSessionID)

	testauth.InstallSessionToken(
		testSessionID,
		"fake-web", // no rules
		"default", "web", "abc123",
	)
	testauth.InstallSessionToken(
		testSessionID,
		"fake-db", // 1 rule (service)
		"default", "db", "def456",
	)
	testauth.InstallSessionToken(
		testSessionID,
		"fake-vault", // 1 rule (role)
		"default", "vault", "jkl012",
	)
	testauth.InstallSessionToken(
		testSessionID,
		"fake-monolith", // 2 rules (one of each)
		"default", "monolith", "ghi789",
	)
	testauth.InstallSessionToken(
		testSessionID,
		"fake-node",
		"default", "mynode", "jkl101",
	)

	method, err := upsertTestAuthMethod(codec, TestDefaultInitialManagementToken, "dc1", testSessionID)
	require.NoError(t, err)

	// 'fake-db' rules
	ruleDB, err := upsertTestBindingRule(
		codec, TestDefaultInitialManagementToken, "dc1", method.Name,
		"serviceaccount.namespace==default and serviceaccount.name==db",
		structs.BindingRuleBindTypeService,
		"method-${serviceaccount.name}",
	)
	require.NoError(t, err)

	// 'fake-vault' rules
	_, err = upsertTestBindingRule(
		codec, TestDefaultInitialManagementToken, "dc1", method.Name,
		"serviceaccount.namespace==default and serviceaccount.name==vault",
		structs.BindingRuleBindTypeRole,
		"method-${serviceaccount.name}",
	)
	require.NoError(t, err)

	// 'fake-monolith' rules
	_, err = upsertTestBindingRule(
		codec, TestDefaultInitialManagementToken, "dc1", method.Name,
		"serviceaccount.namespace==default and serviceaccount.name==monolith",
		structs.BindingRuleBindTypeService,
		"method-${serviceaccount.name}",
	)
	require.NoError(t, err)
	_, err = upsertTestBindingRule(
		codec, TestDefaultInitialManagementToken, "dc1", method.Name,
		"serviceaccount.namespace==default and serviceaccount.name==monolith",
		structs.BindingRuleBindTypeRole,
		"method-${serviceaccount.name}",
	)
	require.NoError(t, err)

	// node identity rule
	_, err = upsertTestBindingRule(
		codec, TestDefaultInitialManagementToken, "dc1", method.Name,
		"serviceaccount.namespace==default and serviceaccount.name==mynode",
		structs.BindingRuleBindTypeNode,
		"${serviceaccount.name}",
	)
	require.NoError(t, err)

	t.Run("do not provide a token", func(t *testing.T) {
		req := structs.ACLLoginRequest{
			Auth: &structs.ACLLoginParams{
				AuthMethod:  method.Name,
				BearerToken: "fake-web",
				Meta:        map[string]string{"pod": "pod1"},
			},
			Datacenter: "dc1",
		}
		req.Token = "nope"
		resp := structs.ACLToken{}

		testutil.RequireErrorContains(t, acl.Login(&req, &resp), "do not provide a token")
	})

	t.Run("unknown method", func(t *testing.T) {
		req := structs.ACLLoginRequest{
			Auth: &structs.ACLLoginParams{
				AuthMethod:  method.Name + "-notexist",
				BearerToken: "fake-web",
				Meta:        map[string]string{"pod": "pod1"},
			},
			Datacenter: "dc1",
		}
		resp := structs.ACLToken{}

		testutil.RequireErrorContains(t, acl.Login(&req, &resp), fmt.Sprintf("auth method %q not found", method.Name+"-notexist"))
	})

	t.Run("invalid method token", func(t *testing.T) {
		req := structs.ACLLoginRequest{
			Auth: &structs.ACLLoginParams{
				AuthMethod:  method.Name,
				BearerToken: "invalid",
				Meta:        map[string]string{"pod": "pod1"},
			},
			Datacenter: "dc1",
		}
		resp := structs.ACLToken{}

		require.Error(t, acl.Login(&req, &resp))
	})

	t.Run("valid method token no bindings", func(t *testing.T) {
		req := structs.ACLLoginRequest{
			Auth: &structs.ACLLoginParams{
				AuthMethod:  method.Name,
				BearerToken: "fake-web",
				Meta:        map[string]string{"pod": "pod1"},
			},
			Datacenter: "dc1",
		}
		resp := structs.ACLToken{}

		testutil.RequireErrorContains(t, acl.Login(&req, &resp), "Permission denied")
	})

	t.Run("valid method token 1 role binding and role does not exist", func(t *testing.T) {
		req := structs.ACLLoginRequest{
			Auth: &structs.ACLLoginParams{
				AuthMethod:  method.Name,
				BearerToken: "fake-vault",
				Meta:        map[string]string{"pod": "pod1"},
			},
			Datacenter: "dc1",
		}
		resp := structs.ACLToken{}

		testutil.RequireErrorContains(t, acl.Login(&req, &resp), "Permission denied")
	})

	// create the role so that the bindtype=role login works
	var vaultRoleID string
	{
		arg := structs.ACLRoleSetRequest{
			Datacenter: "dc1",
			Role: structs.ACLRole{
				Name: "method-vault",
			},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		var out structs.ACLRole
		require.NoError(t, acl.RoleSet(&arg, &out))

		vaultRoleID = out.ID
	}

	t.Run("valid method token 1 role binding and role now exists", func(t *testing.T) {
		req := structs.ACLLoginRequest{
			Auth: &structs.ACLLoginParams{
				AuthMethod:  method.Name,
				BearerToken: "fake-vault",
				Meta:        map[string]string{"pod": "pod1"},
			},
			Datacenter: "dc1",
		}
		resp := structs.ACLToken{}

		require.NoError(t, acl.Login(&req, &resp))

		require.Equal(t, method.Name, resp.AuthMethod)
		require.Equal(t, `token created via login: {"pod":"pod1"}`, resp.Description)
		require.True(t, resp.Local)
		require.Len(t, resp.ServiceIdentities, 0)
		require.Len(t, resp.Roles, 1)
		role := resp.Roles[0]
		require.Equal(t, vaultRoleID, role.ID)
		require.Equal(t, "method-vault", role.Name)
	})

	t.Run("valid method token 1 service binding 1 role binding and role does not exist", func(t *testing.T) {
		req := structs.ACLLoginRequest{
			Auth: &structs.ACLLoginParams{
				AuthMethod:  method.Name,
				BearerToken: "fake-monolith",
				Meta:        map[string]string{"pod": "pod1"},
			},
			Datacenter: "dc1",
		}
		resp := structs.ACLToken{}

		require.NoError(t, acl.Login(&req, &resp))

		require.Equal(t, method.Name, resp.AuthMethod)
		require.Equal(t, `token created via login: {"pod":"pod1"}`, resp.Description)
		require.True(t, resp.Local)
		require.Len(t, resp.ServiceIdentities, 1)
		require.Len(t, resp.Roles, 0)
		svcid := resp.ServiceIdentities[0]
		require.Len(t, svcid.Datacenters, 0)
		require.Equal(t, "method-monolith", svcid.ServiceName)
	})

	// create the role so that the bindtype=role login works
	var monolithRoleID string
	{
		arg := structs.ACLRoleSetRequest{
			Datacenter: "dc1",
			Role: structs.ACLRole{
				Name: "method-monolith",
			},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		var out structs.ACLRole
		require.NoError(t, acl.RoleSet(&arg, &out))

		monolithRoleID = out.ID
	}

	t.Run("valid method token 1 service binding 1 role binding and role now exists", func(t *testing.T) {
		req := structs.ACLLoginRequest{
			Auth: &structs.ACLLoginParams{
				AuthMethod:  method.Name,
				BearerToken: "fake-monolith",
				Meta:        map[string]string{"pod": "pod1"},
			},
			Datacenter: "dc1",
		}
		resp := structs.ACLToken{}

		require.NoError(t, acl.Login(&req, &resp))

		require.Equal(t, method.Name, resp.AuthMethod)
		require.Equal(t, `token created via login: {"pod":"pod1"}`, resp.Description)
		require.True(t, resp.Local)
		require.Len(t, resp.ServiceIdentities, 1)
		require.Len(t, resp.Roles, 1)
		role := resp.Roles[0]
		require.Equal(t, monolithRoleID, role.ID)
		require.Equal(t, "method-monolith", role.Name)
		svcid := resp.ServiceIdentities[0]
		require.Len(t, svcid.Datacenters, 0)
		require.Equal(t, "method-monolith", svcid.ServiceName)
	})

	t.Run("valid bearer token 1 service binding", func(t *testing.T) {
		req := structs.ACLLoginRequest{
			Auth: &structs.ACLLoginParams{
				AuthMethod:  method.Name,
				BearerToken: "fake-db",
				Meta:        map[string]string{"pod": "pod1"},
			},
			Datacenter: "dc1",
		}
		resp := structs.ACLToken{}

		require.NoError(t, acl.Login(&req, &resp))

		require.Equal(t, method.Name, resp.AuthMethod)
		require.Equal(t, `token created via login: {"pod":"pod1"}`, resp.Description)
		require.True(t, resp.Local)
		require.Len(t, resp.Roles, 0)
		require.Len(t, resp.ServiceIdentities, 1)
		svcid := resp.ServiceIdentities[0]
		require.Len(t, svcid.Datacenters, 0)
		require.Equal(t, "method-db", svcid.ServiceName)
	})

	t.Run("valid bearer token 1 node binding", func(t *testing.T) {
		req := structs.ACLLoginRequest{
			Auth: &structs.ACLLoginParams{
				AuthMethod:  method.Name,
				BearerToken: "fake-node",
				Meta:        map[string]string{"node": "true"},
			},
			Datacenter: "dc1",
		}
		resp := structs.ACLToken{}

		require.NoError(t, acl.Login(&req, &resp))

		require.Equal(t, method.Name, resp.AuthMethod)
		require.Equal(t, `token created via login: {"node":"true"}`, resp.Description)
		require.True(t, resp.Local)
		require.Empty(t, resp.Roles)
		require.Empty(t, resp.ServiceIdentities)
		require.Len(t, resp.NodeIdentities, 1)
		nodeid := resp.NodeIdentities[0]
		require.Equal(t, "mynode", nodeid.NodeName)
		require.Equal(t, "dc1", nodeid.Datacenter)
	})

	{
		req := structs.ACLBindingRuleSetRequest{
			Datacenter: "dc1",
			BindingRule: structs.ACLBindingRule{
				AuthMethod: ruleDB.AuthMethod,
				BindType:   structs.BindingRuleBindTypeService,
				BindName:   ruleDB.BindName,
				Selector:   "",
			},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		var out structs.ACLBindingRule
		require.NoError(t, acl.BindingRuleSet(&req, &out))
	}

	t.Run("valid bearer token 1 binding (no selectors this time)", func(t *testing.T) {
		req := structs.ACLLoginRequest{
			Auth: &structs.ACLLoginParams{
				AuthMethod:  method.Name,
				BearerToken: "fake-db",
				Meta:        map[string]string{"pod": "pod1"},
			},
			Datacenter: "dc1",
		}
		resp := structs.ACLToken{}

		require.NoError(t, acl.Login(&req, &resp))

		require.Equal(t, method.Name, resp.AuthMethod)
		require.Equal(t, `token created via login: {"pod":"pod1"}`, resp.Description)
		require.True(t, resp.Local)
		require.Len(t, resp.Roles, 0)
		require.Len(t, resp.ServiceIdentities, 1)
		svcid := resp.ServiceIdentities[0]
		require.Len(t, svcid.Datacenters, 0)
		require.Equal(t, "method-db", svcid.ServiceName)
	})

	testSessionID_2 := testauth.StartSession()
	defer testauth.ResetSession(testSessionID_2)
	{
		// Update the method to force the cache to invalidate for the next
		// subtest.
		updated := *method
		updated.Description = "updated for the test"
		updated.Config = map[string]interface{}{
			"SessionID": testSessionID_2,
		}

		req := structs.ACLAuthMethodSetRequest{
			Datacenter:   "dc1",
			AuthMethod:   updated,
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}

		var ignored structs.ACLAuthMethod
		require.NoError(t, acl.AuthMethodSet(&req, &ignored))
	}

	t.Run("updating the method invalidates the cache", func(t *testing.T) {
		// We'll try to login with the 'fake-db' cred which DOES exist in the
		// old fake validator, but no longer exists in the new fake validator.
		req := structs.ACLLoginRequest{
			Auth: &structs.ACLLoginParams{
				AuthMethod:  method.Name,
				BearerToken: "fake-db",
				Meta:        map[string]string{"pod": "pod1"},
			},
			Datacenter: "dc1",
		}
		resp := structs.ACLToken{}

		testutil.RequireErrorContains(t, acl.Login(&req, &resp), "ACL not found")
	})
}

func TestACLEndpoint_Login_with_MaxTokenTTL(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	_, srv, codec := testACLServerWithConfig(t, nil, false)
	waitForLeaderEstablishment(t, srv)

	acl := ACL{srv: srv}

	testSessionID := testauth.StartSession()
	defer testauth.ResetSession(testSessionID)

	testauth.InstallSessionToken(
		testSessionID,
		"fake-web", // no rules
		"default", "web", "abc123",
	)

	method, err := upsertTestCustomizedAuthMethod(codec, TestDefaultInitialManagementToken, "dc1", func(method *structs.ACLAuthMethod) {
		method.MaxTokenTTL = 5 * time.Minute
		method.Config = map[string]interface{}{
			"SessionID": testSessionID,
		}
	})
	require.NoError(t, err)

	_, err = upsertTestBindingRule(
		codec, TestDefaultInitialManagementToken, "dc1", method.Name,
		"",
		structs.BindingRuleBindTypeService,
		"web",
	)
	require.NoError(t, err)

	// Create a token.
	req := structs.ACLLoginRequest{
		Auth: &structs.ACLLoginParams{
			AuthMethod:  method.Name,
			BearerToken: "fake-web",
			Meta:        map[string]string{"pod": "pod1"},
		},
		Datacenter: "dc1",
	}

	resp := structs.ACLToken{}
	require.NoError(t, acl.Login(&req, &resp))

	got := &resp
	got.CreateIndex = 0
	got.ModifyIndex = 0
	got.AccessorID = ""
	got.SecretID = ""
	got.Hash = nil

	defaultEntMeta := structs.DefaultEnterpriseMetaInDefaultPartition()
	expect := &structs.ACLToken{
		AuthMethod:     method.Name,
		Description:    `token created via login: {"pod":"pod1"}`,
		Local:          true,
		CreateTime:     got.CreateTime,
		ExpirationTime: timePointer(got.CreateTime.Add(method.MaxTokenTTL)),
		ServiceIdentities: []*structs.ACLServiceIdentity{
			{ServiceName: "web"},
		},
		EnterpriseMeta: *defaultEntMeta,
	}
	expect.ACLAuthMethodEnterpriseMeta.FillWithEnterpriseMeta(defaultEntMeta)
	require.Equal(t, got, expect)
}

func TestACLEndpoint_Login_with_TokenLocality(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	_, s1, codec := testACLServerWithConfig(t, func(c *Config) {
		c.ACLTokenMinExpirationTTL = 10 * time.Millisecond
		c.ACLTokenMaxExpirationTTL = 5 * time.Second
	}, false)

	_, s2, _ := testACLServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc2"
		c.ACLTokenMinExpirationTTL = 10 * time.Millisecond
		c.ACLTokenMaxExpirationTTL = 5 * time.Second
		// token replication is required to test deleting non-local tokens in secondary dc
		c.ACLTokenReplication = true
	}, true)

	waitForLeaderEstablishment(t, s1)
	waitForLeaderEstablishment(t, s2)

	joinWAN(t, s2, s1)

	// Ensure s2 is authoritative.
	waitForNewACLReplication(t, s2, structs.ACLReplicateTokens, 1, 1, 0)

	acl := ACL{srv: s1}
	acl2 := ACL{srv: s2}

	testSessionID := testauth.StartSession()
	defer testauth.ResetSession(testSessionID)

	testauth.InstallSessionToken(
		testSessionID,
		"fake-web", // no rules
		"default", "web", "abc123",
	)

	cases := map[string]struct {
		tokenLocality     string
		expectLocal       bool
		deleteFromReplica bool
	}{
		"empty":             {tokenLocality: "", expectLocal: true},
		"local":             {tokenLocality: "local", expectLocal: true},
		"global dc1 delete": {tokenLocality: "global", expectLocal: false, deleteFromReplica: false},
		"global dc2 delete": {tokenLocality: "global", expectLocal: false, deleteFromReplica: true},
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			method, err := upsertTestCustomizedAuthMethod(codec, TestDefaultInitialManagementToken, "dc1", func(method *structs.ACLAuthMethod) {
				method.TokenLocality = tc.tokenLocality
				method.Config = map[string]interface{}{
					"SessionID": testSessionID,
				}
			})
			require.NoError(t, err)

			_, err = upsertTestBindingRule(
				codec, TestDefaultInitialManagementToken, "dc1", method.Name,
				"",
				structs.BindingRuleBindTypeService,
				"web",
			)
			require.NoError(t, err)

			// Create a token.
			req := structs.ACLLoginRequest{
				Auth: &structs.ACLLoginParams{
					AuthMethod:  method.Name,
					BearerToken: "fake-web",
					Meta:        map[string]string{"pod": "pod1"},
				},
				Datacenter: "dc1",
			}

			resp := structs.ACLToken{}
			require.NoError(t, acl.Login(&req, &resp))

			secretID := resp.SecretID

			got := resp.Clone()
			got.CreateIndex = 0
			got.ModifyIndex = 0
			got.AccessorID = ""
			got.SecretID = ""
			got.Hash = nil

			defaultEntMeta := structs.DefaultEnterpriseMetaInDefaultPartition()
			expect := &structs.ACLToken{
				AuthMethod:  method.Name,
				Description: `token created via login: {"pod":"pod1"}`,
				Local:       tc.expectLocal,
				CreateTime:  got.CreateTime,
				ServiceIdentities: []*structs.ACLServiceIdentity{
					{ServiceName: "web"},
				},
				EnterpriseMeta: *defaultEntMeta,
			}
			expect.ACLAuthMethodEnterpriseMeta.FillWithEnterpriseMeta(defaultEntMeta)
			require.Equal(t, got, expect)

			// Now turn around and nuke it.
			var (
				logoutACL ACL
				logoutDC  string
			)
			if tc.deleteFromReplica {
				// wait for it to show up
				retry.Run(t, func(r *retry.R) {
					var resp structs.ACLTokenResponse
					require.NoError(r, acl2.TokenRead(&structs.ACLTokenGetRequest{
						TokenID:        secretID,
						TokenIDType:    structs.ACLTokenSecret,
						Datacenter:     "dc2",
						EnterpriseMeta: *defaultEntMeta,
						QueryOptions:   structs.QueryOptions{Token: TestDefaultInitialManagementToken},
					}, &resp))
					require.NotNil(r, resp.Token, "cannot lookup token with secretID %q", secretID)
				})

				logoutACL = acl2
				logoutDC = "dc2"
			} else {
				logoutACL = acl
				logoutDC = "dc1"
			}

			logoutReq := structs.ACLLogoutRequest{
				Datacenter:   logoutDC,
				WriteRequest: structs.WriteRequest{Token: secretID},
			}

			var ignored bool
			require.NoError(t, logoutACL.Logout(&logoutReq, &ignored))
		})
	}
}

func TestACLEndpoint_Login_k8s(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	_, srv, codec := testACLServerWithConfig(t, nil, false)
	waitForLeaderEstablishment(t, srv)

	acl := ACL{srv: srv}

	// spin up a fake api server
	testSrv := kubeauth.StartTestAPIServer(t)
	defer testSrv.Stop()

	testSrv.AuthorizeJWT(goodJWT_A)
	testSrv.SetAllowedServiceAccount(
		"default",
		"demo",
		"76091af4-4b56-11e9-ac4b-708b11801cbe",
		"",
		goodJWT_B,
	)

	method, err := upsertTestKubernetesAuthMethod(
		codec, TestDefaultInitialManagementToken, "dc1",
		testSrv.CACert(),
		testSrv.Addr(),
		goodJWT_A,
	)
	require.NoError(t, err)

	t.Run("invalid bearer token", func(t *testing.T) {
		req := structs.ACLLoginRequest{
			Auth: &structs.ACLLoginParams{
				AuthMethod:  method.Name,
				BearerToken: "invalid",
				Meta:        map[string]string{"pod": "pod1"},
			},
			Datacenter: "dc1",
		}
		resp := structs.ACLToken{}

		require.Error(t, acl.Login(&req, &resp))
	})

	t.Run("valid bearer token no bindings", func(t *testing.T) {
		req := structs.ACLLoginRequest{
			Auth: &structs.ACLLoginParams{
				AuthMethod:  method.Name,
				BearerToken: goodJWT_B,
				Meta:        map[string]string{"pod": "pod1"},
			},
			Datacenter: "dc1",
		}
		resp := structs.ACLToken{}

		testutil.RequireErrorContains(t, acl.Login(&req, &resp), "Permission denied")
	})

	_, err = upsertTestBindingRule(
		codec, TestDefaultInitialManagementToken, "dc1", method.Name,
		"serviceaccount.namespace==default",
		structs.BindingRuleBindTypeService,
		"${serviceaccount.name}",
	)
	require.NoError(t, err)

	t.Run("valid bearer token 1 service binding", func(t *testing.T) {
		req := structs.ACLLoginRequest{
			Auth: &structs.ACLLoginParams{
				AuthMethod:  method.Name,
				BearerToken: goodJWT_B,
				Meta:        map[string]string{"pod": "pod1"},
			},
			Datacenter: "dc1",
		}
		resp := structs.ACLToken{}

		require.NoError(t, acl.Login(&req, &resp))

		require.Equal(t, method.Name, resp.AuthMethod)
		require.Equal(t, `token created via login: {"pod":"pod1"}`, resp.Description)
		require.True(t, resp.Local)
		require.Len(t, resp.Roles, 0)
		require.Len(t, resp.ServiceIdentities, 1)
		svcid := resp.ServiceIdentities[0]
		require.Len(t, svcid.Datacenters, 0)
		require.Equal(t, "demo", svcid.ServiceName)
	})

	// annotate the account
	testSrv.SetAllowedServiceAccount(
		"default",
		"demo",
		"76091af4-4b56-11e9-ac4b-708b11801cbe",
		"alternate-name",
		goodJWT_B,
	)

	t.Run("valid bearer token 1 service binding - with annotation", func(t *testing.T) {
		req := structs.ACLLoginRequest{
			Auth: &structs.ACLLoginParams{
				AuthMethod:  method.Name,
				BearerToken: goodJWT_B,
				Meta:        map[string]string{"pod": "pod1"},
			},
			Datacenter: "dc1",
		}
		resp := structs.ACLToken{}

		require.NoError(t, acl.Login(&req, &resp))

		require.Equal(t, method.Name, resp.AuthMethod)
		require.Equal(t, `token created via login: {"pod":"pod1"}`, resp.Description)
		require.True(t, resp.Local)
		require.Len(t, resp.Roles, 0)
		require.Len(t, resp.ServiceIdentities, 1)
		svcid := resp.ServiceIdentities[0]
		require.Len(t, svcid.Datacenters, 0)
		require.Equal(t, "alternate-name", svcid.ServiceName)
	})
}

func TestACLEndpoint_Login_jwt(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	_, srv, codec := testACLServerWithConfig(t, nil, false)
	waitForLeaderEstablishment(t, srv)

	acl := ACL{srv: srv}

	// spin up a fake oidc server
	oidcServer := oidcauthtest.Start(t)
	pubKey, privKey := oidcServer.SigningKeys()

	type mConfig = map[string]interface{}
	cases := map[string]struct {
		f         func(config mConfig)
		issuer    string
		expectErr string
	}{
		"success - jwt static keys": {func(config mConfig) {
			config["BoundIssuer"] = "https://legit.issuer.internal/"
			config["JWTValidationPubKeys"] = []string{pubKey}
		},
			"https://legit.issuer.internal/",
			""},
		"success - jwt jwks": {func(config mConfig) {
			config["JWKSURL"] = oidcServer.Addr() + "/certs"
			config["JWKSCACert"] = oidcServer.CACert()
		},
			"https://legit.issuer.internal/",
			""},
		"success - jwt oidc discovery": {func(config mConfig) {
			config["OIDCDiscoveryURL"] = oidcServer.Addr()
			config["OIDCDiscoveryCACert"] = oidcServer.CACert()
		},
			oidcServer.Addr(),
			""},
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			method, err := upsertTestCustomizedAuthMethod(codec, TestDefaultInitialManagementToken, "dc1", func(method *structs.ACLAuthMethod) {
				method.Type = "jwt"
				method.Config = map[string]interface{}{
					"JWTSupportedAlgs": []string{"ES256"},
					"ClaimMappings": map[string]string{
						"first_name":   "name",
						"/org/primary": "primary_org",
					},
					"ListClaimMappings": map[string]string{
						"https://consul.test/groups": "groups",
					},
					"BoundAudiences": []string{"https://consul.test"},
				}
				if tc.f != nil {
					tc.f(method.Config)
				}
			})
			require.NoError(t, err)

			t.Run("invalid bearer token", func(t *testing.T) {
				req := structs.ACLLoginRequest{
					Auth: &structs.ACLLoginParams{
						AuthMethod:  method.Name,
						BearerToken: "invalid",
					},
					Datacenter: "dc1",
				}
				resp := structs.ACLToken{}

				require.Error(t, acl.Login(&req, &resp))
			})

			cl := jwt.Claims{
				Subject:   "r3qXcK2bix9eFECzsU3Sbmh0K16fatW6@clients",
				Audience:  jwt.Audience{"https://consul.test"},
				Issuer:    tc.issuer,
				NotBefore: jwt.NewNumericDate(time.Now().Add(-5 * time.Second)),
				Expiry:    jwt.NewNumericDate(time.Now().Add(5 * time.Second)),
			}

			type orgs struct {
				Primary string `json:"primary"`
			}

			privateCl := struct {
				FirstName string   `json:"first_name"`
				Org       orgs     `json:"org"`
				Groups    []string `json:"https://consul.test/groups"`
			}{
				FirstName: "jeff2",
				Org:       orgs{"engineering"},
				Groups:    []string{"foo", "bar"},
			}

			jwtData, err := oidcauthtest.SignJWT(privKey, cl, privateCl)
			require.NoError(t, err)

			t.Run("valid bearer token no bindings", func(t *testing.T) {
				req := structs.ACLLoginRequest{
					Auth: &structs.ACLLoginParams{
						AuthMethod:  method.Name,
						BearerToken: jwtData,
					},
					Datacenter: "dc1",
				}
				resp := structs.ACLToken{}

				testutil.RequireErrorContains(t, acl.Login(&req, &resp), "Permission denied")
			})

			_, err = upsertTestBindingRule(
				codec, TestDefaultInitialManagementToken, "dc1", method.Name,
				"value.name == jeff2 and value.primary_org == engineering and foo in list.groups",
				structs.BindingRuleBindTypeService,
				"test--${value.name}--${value.primary_org}",
			)
			require.NoError(t, err)

			t.Run("valid bearer token 1 service binding", func(t *testing.T) {
				req := structs.ACLLoginRequest{
					Auth: &structs.ACLLoginParams{
						AuthMethod:  method.Name,
						BearerToken: jwtData,
					},
					Datacenter: "dc1",
				}
				resp := structs.ACLToken{}

				require.NoError(t, acl.Login(&req, &resp))

				require.Equal(t, method.Name, resp.AuthMethod)
				require.Equal(t, `token created via login`, resp.Description)
				require.True(t, resp.Local)
				require.Len(t, resp.Roles, 0)
				require.Len(t, resp.ServiceIdentities, 1)
				svcid := resp.ServiceIdentities[0]
				require.Len(t, svcid.Datacenters, 0)
				require.Equal(t, "test--jeff2--engineering", svcid.ServiceName)
			})
		})
	}
}

func TestACLEndpoint_Logout(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	_, srv, codec := testACLServerWithConfig(t, nil, false)
	waitForLeaderEstablishment(t, srv)

	acl := ACL{srv: srv}

	testSessionID := testauth.StartSession()
	defer testauth.ResetSession(testSessionID)
	testauth.InstallSessionToken(
		testSessionID,
		"fake-db", // 1 rule
		"default", "db", "def456",
	)

	method, err := upsertTestAuthMethod(codec, TestDefaultInitialManagementToken, "dc1", testSessionID)
	require.NoError(t, err)

	_, err = upsertTestBindingRule(
		codec, TestDefaultInitialManagementToken, "dc1", method.Name,
		"",
		structs.BindingRuleBindTypeService,
		"method-${serviceaccount.name}",
	)
	require.NoError(t, err)

	t.Run("you must provide a token", func(t *testing.T) {
		req := structs.ACLLogoutRequest{
			Datacenter:   "dc1",
			WriteRequest: structs.WriteRequest{Token: ""},
		}
		req.Token = ""
		var ignored bool

		testutil.RequireErrorContains(t, acl.Logout(&req, &ignored), "ACL not found")
	})

	t.Run("logout from deleted token", func(t *testing.T) {
		req := structs.ACLLogoutRequest{
			Datacenter:   "dc1",
			WriteRequest: structs.WriteRequest{Token: "not-found"},
		}
		var ignored bool
		testutil.RequireErrorContains(t, acl.Logout(&req, &ignored), "ACL not found")
	})

	t.Run("logout from non-auth method-linked token should fail", func(t *testing.T) {
		req := structs.ACLLogoutRequest{
			Datacenter:   "dc1",
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}
		var ignored bool
		testutil.RequireErrorContains(t, acl.Logout(&req, &ignored), "Permission denied")
	})

	t.Run("login then logout", func(t *testing.T) {
		// Create a totally legit Login token.
		loginReq := structs.ACLLoginRequest{
			Auth: &structs.ACLLoginParams{
				AuthMethod:  method.Name,
				BearerToken: "fake-db",
			},
			Datacenter: "dc1",
		}
		loginToken := structs.ACLToken{}

		require.NoError(t, acl.Login(&loginReq, &loginToken))
		require.NotEmpty(t, loginToken.SecretID)

		// Now turn around and nuke it.
		req := structs.ACLLogoutRequest{
			Datacenter:   "dc1",
			WriteRequest: structs.WriteRequest{Token: loginToken.SecretID},
		}

		var ignored bool
		require.NoError(t, acl.Logout(&req, &ignored))
	})
}

func gatherIDs(t *testing.T, v interface{}) []string {
	t.Helper()

	var out []string
	switch x := v.(type) {
	case []*structs.ACLRole:
		for _, r := range x {
			out = append(out, r.ID)
		}
	case structs.ACLRoles:
		for _, r := range x {
			out = append(out, r.ID)
		}
	case []*structs.ACLPolicy:
		for _, p := range x {
			out = append(out, p.ID)
		}
	case structs.ACLPolicyListStubs:
		for _, p := range x {
			out = append(out, p.ID)
		}
	case []*structs.ACLToken:
		for _, p := range x {
			out = append(out, p.AccessorID)
		}
	case structs.ACLTokenListStubs:
		for _, p := range x {
			out = append(out, p.AccessorID)
		}
	case []*structs.ACLAuthMethod:
		for _, p := range x {
			out = append(out, p.Name)
		}
	case structs.ACLAuthMethodListStubs:
		for _, p := range x {
			out = append(out, p.Name)
		}
	case []*structs.ACLBindingRule:
		for _, p := range x {
			out = append(out, p.ID)
		}
	case structs.ACLBindingRules:
		for _, p := range x {
			out = append(out, p.ID)
		}
	default:
		t.Fatalf("unknown type: %T", x)
	}
	return out
}

// upsertTestToken creates a token for testing purposes
func upsertTestTokenInEntMeta(codec rpc.ClientCodec, initialManagementToken string, datacenter string,
	tokenModificationFn func(token *structs.ACLToken), entMeta *acl.EnterpriseMeta) (*structs.ACLToken, error) {
	if entMeta == nil {
		entMeta = structs.DefaultEnterpriseMetaInDefaultPartition()
	}
	arg := structs.ACLTokenSetRequest{
		Datacenter: datacenter,
		ACLToken: structs.ACLToken{
			Description:    "User token",
			Local:          false,
			Policies:       nil,
			EnterpriseMeta: *entMeta,
		},
		WriteRequest: structs.WriteRequest{Token: initialManagementToken},
	}

	if tokenModificationFn != nil {
		tokenModificationFn(&arg.ACLToken)
	}

	var out structs.ACLToken

	err := msgpackrpc.CallWithCodec(codec, "ACL.TokenSet", &arg, &out)

	if err != nil {
		return nil, err
	}

	if out.AccessorID == "" {
		return nil, fmt.Errorf("AccessorID is nil: %v", out)
	}

	return &out, nil
}

func upsertTestToken(codec rpc.ClientCodec, initialManagementToken string, datacenter string,
	tokenModificationFn func(token *structs.ACLToken)) (*structs.ACLToken, error) {
	return upsertTestTokenInEntMeta(codec, initialManagementToken, datacenter,
		tokenModificationFn, structs.DefaultEnterpriseMetaInDefaultPartition())
}

func upsertTestTokenWithPolicyRulesInEntMeta(codec rpc.ClientCodec, initialManagementToken string, datacenter string, rules string, entMeta *acl.EnterpriseMeta) (*structs.ACLToken, error) {
	policy, err := upsertTestPolicyWithRulesInEntMeta(codec, initialManagementToken, datacenter, rules, entMeta)
	if err != nil {
		return nil, err
	}

	token, err := upsertTestTokenInEntMeta(codec, initialManagementToken, datacenter, func(token *structs.ACLToken) {
		token.Policies = []structs.ACLTokenPolicyLink{{ID: policy.ID}}
	}, entMeta)
	if err != nil {
		return nil, err
	}

	return token, nil
}

func upsertTestTokenWithPolicyRules(codec rpc.ClientCodec, initialManagementToken string, datacenter string, rules string) (*structs.ACLToken, error) {
	return upsertTestTokenWithPolicyRulesInEntMeta(codec, initialManagementToken, datacenter, rules, nil)
}

func retrieveTestTokenAccessorForSecret(codec rpc.ClientCodec, initialManagementToken string, datacenter string, id string) (string, error) {
	arg := structs.ACLTokenGetRequest{
		TokenID:      id,
		TokenIDType:  structs.ACLTokenSecret,
		Datacenter:   datacenter,
		QueryOptions: structs.QueryOptions{Token: initialManagementToken},
	}

	var out structs.ACLTokenResponse

	err := msgpackrpc.CallWithCodec(codec, "ACL.TokenRead", &arg, &out)

	if err != nil {
		return "", err
	}

	if out.Token == nil {
		return "", nil
	}

	return out.Token.AccessorID, nil
}

// retrieveTestToken returns a policy for testing purposes
func retrieveTestToken(codec rpc.ClientCodec, initialManagementToken string, datacenter string, id string) (*structs.ACLTokenResponse, error) {
	arg := structs.ACLTokenGetRequest{
		Datacenter:   datacenter,
		TokenID:      id,
		TokenIDType:  structs.ACLTokenAccessor,
		QueryOptions: structs.QueryOptions{Token: initialManagementToken},
	}

	var out structs.ACLTokenResponse

	err := msgpackrpc.CallWithCodec(codec, "ACL.TokenRead", &arg, &out)

	if err != nil {
		return nil, err
	}

	return &out, nil
}

func deleteTestToken(codec rpc.ClientCodec, initialManagementToken string, datacenter string, tokenAccessor string) error {
	arg := structs.ACLTokenDeleteRequest{
		Datacenter:   datacenter,
		TokenID:      tokenAccessor,
		WriteRequest: structs.WriteRequest{Token: initialManagementToken},
	}

	var ignored string
	err := msgpackrpc.CallWithCodec(codec, "ACL.TokenDelete", &arg, &ignored)
	return err
}

func deleteTestPolicy(codec rpc.ClientCodec, initialManagementToken string, datacenter string, policyID string) error {
	arg := structs.ACLPolicyDeleteRequest{
		Datacenter:   datacenter,
		PolicyID:     policyID,
		WriteRequest: structs.WriteRequest{Token: initialManagementToken},
	}

	var ignored string
	err := msgpackrpc.CallWithCodec(codec, "ACL.PolicyDelete", &arg, &ignored)
	return err
}

func upsertTestCustomizedPolicy(codec rpc.ClientCodec, initialManagementToken string, datacenter string, policyModificationFn func(policy *structs.ACLPolicy)) (*structs.ACLPolicy, error) {
	// Make sure test policies can't collide
	policyUnq, err := uuid.GenerateUUID()
	if err != nil {
		return nil, err
	}

	arg := structs.ACLPolicySetRequest{
		Datacenter: datacenter,
		Policy: structs.ACLPolicy{
			Name: fmt.Sprintf("test-policy-%s", policyUnq),
		},
		WriteRequest: structs.WriteRequest{Token: initialManagementToken},
	}

	if policyModificationFn != nil {
		policyModificationFn(&arg.Policy)
	}

	var out structs.ACLPolicy

	err = msgpackrpc.CallWithCodec(codec, "ACL.PolicySet", &arg, &out)

	if err != nil {
		return nil, err
	}

	if out.ID == "" {
		return nil, fmt.Errorf("ID is nil: %v", out)
	}

	return &out, nil
}

// upsertTestPolicy creates a policy for testing purposes
func upsertTestPolicy(codec rpc.ClientCodec, initialManagementToken string, datacenter string) (*structs.ACLPolicy, error) {
	return upsertTestPolicyWithRules(codec, initialManagementToken, datacenter, "")
}

func upsertTestPolicyWithRules(codec rpc.ClientCodec, initialManagementToken string, datacenter string, rules string) (*structs.ACLPolicy, error) {
	return upsertTestPolicyWithRulesInEntMeta(codec, initialManagementToken, datacenter, rules, structs.DefaultEnterpriseMetaInDefaultPartition())
}

func upsertTestPolicyWithRulesInEntMeta(codec rpc.ClientCodec, initialManagementToken string, datacenter string, rules string, entMeta *acl.EnterpriseMeta) (*structs.ACLPolicy, error) {
	return upsertTestCustomizedPolicy(codec, initialManagementToken, datacenter, func(policy *structs.ACLPolicy) {
		if entMeta == nil {
			entMeta = structs.DefaultEnterpriseMetaInDefaultPartition()
		}
		policy.Rules = rules
		policy.EnterpriseMeta = *entMeta
	})
}

// retrieveTestPolicy returns a policy for testing purposes
func retrieveTestPolicy(codec rpc.ClientCodec, initialManagementToken string, datacenter string, id string) (*structs.ACLPolicyResponse, error) {
	arg := structs.ACLPolicyGetRequest{
		Datacenter:   datacenter,
		PolicyID:     id,
		QueryOptions: structs.QueryOptions{Token: initialManagementToken},
	}

	var out structs.ACLPolicyResponse

	err := msgpackrpc.CallWithCodec(codec, "ACL.PolicyRead", &arg, &out)

	if err != nil {
		return nil, err
	}

	return &out, nil
}

func deleteTestRole(codec rpc.ClientCodec, initialManagementToken string, datacenter string, roleID string) error {
	arg := structs.ACLRoleDeleteRequest{
		Datacenter:   datacenter,
		RoleID:       roleID,
		WriteRequest: structs.WriteRequest{Token: initialManagementToken},
	}

	var ignored string
	err := msgpackrpc.CallWithCodec(codec, "ACL.RoleDelete", &arg, &ignored)
	return err
}

func deleteTestRoleByName(codec rpc.ClientCodec, initialManagementToken string, datacenter string, roleName string) error {
	resp, err := retrieveTestRoleByName(codec, initialManagementToken, datacenter, roleName)
	if err != nil {
		return err
	}
	if resp.Role == nil {
		return nil
	}

	return deleteTestRole(codec, initialManagementToken, datacenter, resp.Role.ID)
}

// upsertTestRole creates a role for testing purposes
func upsertTestRole(codec rpc.ClientCodec, initialManagementToken string, datacenter string) (*structs.ACLRole, error) {
	return upsertTestCustomizedRole(codec, initialManagementToken, datacenter, nil)
}

func upsertTestCustomizedRole(codec rpc.ClientCodec, initialManagementToken string, datacenter string, modify func(role *structs.ACLRole)) (*structs.ACLRole, error) {
	// Make sure test roles can't collide
	roleUnq, err := uuid.GenerateUUID()
	if err != nil {
		return nil, err
	}

	arg := structs.ACLRoleSetRequest{
		Datacenter: datacenter,
		Role: structs.ACLRole{
			Name: fmt.Sprintf("test-role-%s", roleUnq),
		},
		WriteRequest: structs.WriteRequest{Token: initialManagementToken},
	}

	if modify != nil {
		modify(&arg.Role)
	}

	var out structs.ACLRole

	err = msgpackrpc.CallWithCodec(codec, "ACL.RoleSet", &arg, &out)

	if err != nil {
		return nil, err
	}

	if out.ID == "" {
		return nil, fmt.Errorf("ID is nil: %v", out)
	}

	return &out, nil
}

func retrieveTestRole(codec rpc.ClientCodec, initialManagementToken string, datacenter string, id string) (*structs.ACLRoleResponse, error) {
	arg := structs.ACLRoleGetRequest{
		Datacenter:   datacenter,
		RoleID:       id,
		QueryOptions: structs.QueryOptions{Token: initialManagementToken},
	}

	var out structs.ACLRoleResponse

	err := msgpackrpc.CallWithCodec(codec, "ACL.RoleRead", &arg, &out)

	if err != nil {
		return nil, err
	}

	return &out, nil
}

func retrieveTestRoleByName(codec rpc.ClientCodec, initialManagementToken string, datacenter string, name string) (*structs.ACLRoleResponse, error) {
	arg := structs.ACLRoleGetRequest{
		Datacenter:   datacenter,
		RoleName:     name,
		QueryOptions: structs.QueryOptions{Token: initialManagementToken},
	}

	var out structs.ACLRoleResponse

	err := msgpackrpc.CallWithCodec(codec, "ACL.RoleRead", &arg, &out)

	if err != nil {
		return nil, err
	}

	return &out, nil
}

func deleteTestAuthMethod(codec rpc.ClientCodec, initialManagementToken string, datacenter string, methodName string) error {
	arg := structs.ACLAuthMethodDeleteRequest{
		Datacenter:     datacenter,
		AuthMethodName: methodName,
		WriteRequest:   structs.WriteRequest{Token: initialManagementToken},
	}

	var ignored string
	err := msgpackrpc.CallWithCodec(codec, "ACL.AuthMethodDelete", &arg, &ignored)
	return err
}

func upsertTestAuthMethod(
	codec rpc.ClientCodec, initialManagementToken string, datacenter string,
	sessionID string,
) (*structs.ACLAuthMethod, error) {
	return upsertTestCustomizedAuthMethod(codec, initialManagementToken, datacenter, func(method *structs.ACLAuthMethod) {
		method.Config = map[string]interface{}{
			"SessionID": sessionID,
		}
	})
}

func upsertTestCustomizedAuthMethod(
	codec rpc.ClientCodec, initialManagementToken string, datacenter string,
	modify func(method *structs.ACLAuthMethod),
) (*structs.ACLAuthMethod, error) {
	name, err := uuid.GenerateUUID()
	if err != nil {
		return nil, err
	}

	req := structs.ACLAuthMethodSetRequest{
		Datacenter: datacenter,
		AuthMethod: structs.ACLAuthMethod{
			Name: "test-method-" + name,
			Type: "testing",
		},
		WriteRequest: structs.WriteRequest{Token: initialManagementToken},
	}

	if modify != nil {
		modify(&req.AuthMethod)
	}

	var out structs.ACLAuthMethod

	err = msgpackrpc.CallWithCodec(codec, "ACL.AuthMethodSet", &req, &out)
	if err != nil {
		return nil, err
	}

	return &out, nil
}

func upsertTestKubernetesAuthMethod(
	codec rpc.ClientCodec, initialManagementToken string, datacenter string,
	caCert, kubeHost, kubeJWT string,
) (*structs.ACLAuthMethod, error) {
	name, err := uuid.GenerateUUID()
	if err != nil {
		return nil, err
	}

	if kubeHost == "" {
		kubeHost = "https://abc:8443"
	}
	if kubeJWT == "" {
		kubeJWT = goodJWT_A
	}

	req := structs.ACLAuthMethodSetRequest{
		Datacenter: datacenter,
		AuthMethod: structs.ACLAuthMethod{
			Name: "test-method-" + name,
			Type: "kubernetes",
			Config: map[string]interface{}{
				"Host":              kubeHost,
				"CACert":            caCert,
				"ServiceAccountJWT": kubeJWT,
			},
		},
		WriteRequest: structs.WriteRequest{Token: initialManagementToken},
	}

	var out structs.ACLAuthMethod

	err = msgpackrpc.CallWithCodec(codec, "ACL.AuthMethodSet", &req, &out)
	if err != nil {
		return nil, err
	}

	return &out, nil
}

func retrieveTestAuthMethod(codec rpc.ClientCodec, initialManagementToken string, datacenter string, name string) (*structs.ACLAuthMethodResponse, error) {
	arg := structs.ACLAuthMethodGetRequest{
		Datacenter:     datacenter,
		AuthMethodName: name,
		QueryOptions:   structs.QueryOptions{Token: initialManagementToken},
	}

	var out structs.ACLAuthMethodResponse

	err := msgpackrpc.CallWithCodec(codec, "ACL.AuthMethodRead", &arg, &out)

	if err != nil {
		return nil, err
	}

	return &out, nil
}

func deleteTestBindingRule(codec rpc.ClientCodec, initialManagementToken string, datacenter string, ruleID string) error {
	arg := structs.ACLBindingRuleDeleteRequest{
		Datacenter:    datacenter,
		BindingRuleID: ruleID,
		WriteRequest:  structs.WriteRequest{Token: initialManagementToken},
	}

	var ignored string
	err := msgpackrpc.CallWithCodec(codec, "ACL.BindingRuleDelete", &arg, &ignored)
	return err
}

func upsertTestBindingRule(
	codec rpc.ClientCodec,
	initialManagementToken string,
	datacenter string,
	methodName string,
	selector string,
	bindType string,
	bindName string,
) (*structs.ACLBindingRule, error) {
	return upsertTestCustomizedBindingRule(codec, initialManagementToken, datacenter, func(rule *structs.ACLBindingRule) {
		rule.AuthMethod = methodName
		rule.BindType = bindType
		rule.BindName = bindName
		rule.Selector = selector
	})
}

func upsertTestCustomizedBindingRule(codec rpc.ClientCodec, initialManagementToken string, datacenter string, modify func(rule *structs.ACLBindingRule)) (*structs.ACLBindingRule, error) {
	req := structs.ACLBindingRuleSetRequest{
		Datacenter:   datacenter,
		BindingRule:  structs.ACLBindingRule{},
		WriteRequest: structs.WriteRequest{Token: initialManagementToken},
	}

	if modify != nil {
		modify(&req.BindingRule)
	}

	var out structs.ACLBindingRule

	err := msgpackrpc.CallWithCodec(codec, "ACL.BindingRuleSet", &req, &out)
	if err != nil {
		return nil, err
	}

	return &out, nil
}

func retrieveTestBindingRule(codec rpc.ClientCodec, initialManagementToken string, datacenter string, ruleID string) (*structs.ACLBindingRuleResponse, error) {
	arg := structs.ACLBindingRuleGetRequest{
		Datacenter:    datacenter,
		BindingRuleID: ruleID,
		QueryOptions:  structs.QueryOptions{Token: initialManagementToken},
	}

	var out structs.ACLBindingRuleResponse

	err := msgpackrpc.CallWithCodec(codec, "ACL.BindingRuleRead", &arg, &out)

	if err != nil {
		return nil, err
	}

	return &out, nil
}

func requireTimeEquals(t *testing.T, expect, got *time.Time) {
	t.Helper()
	if expect == nil && got == nil {
		return
	} else if expect == nil && got != nil {
		t.Fatalf("expected=NIL != got=%q", *got)
	} else if expect != nil && got == nil {
		t.Fatalf("expected=%q != got=NIL", *expect)
	} else if !expect.Equal(*got) {
		t.Fatalf("expected=%q != got=%q", *expect, *got)
	}
}

// 'default/admin'
const goodJWT_A = "eyJhbGciOiJSUzI1NiIsImtpZCI6IiJ9.eyJpc3MiOiJrdWJlcm5ldGVzL3NlcnZpY2VhY2NvdW50Iiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9uYW1lc3BhY2UiOiJkZWZhdWx0Iiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9zZWNyZXQubmFtZSI6ImFkbWluLXRva2VuLXFsejQyIiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9zZXJ2aWNlLWFjY291bnQubmFtZSI6ImFkbWluIiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9zZXJ2aWNlLWFjY291bnQudWlkIjoiNzM4YmMyNTEtNjUzMi0xMWU5LWI2N2YtNDhlNmM4YjhlY2I1Iiwic3ViIjoic3lzdGVtOnNlcnZpY2VhY2NvdW50OmRlZmF1bHQ6YWRtaW4ifQ.ixMlnWrAG7NVuTTKu8cdcYfM7gweS3jlKaEsIBNGOVEjPE7rtXtgMkAwjQTdYR08_0QBjkgzy5fQC5ZNyglSwONJ-bPaXGvhoH1cTnRi1dz9H_63CfqOCvQP1sbdkMeRxNTGVAyWZT76rXoCUIfHP4LY2I8aab0KN9FTIcgZRF0XPTtT70UwGIrSmRpxW38zjiy2ymWL01cc5VWGhJqVysmWmYk3wNp0h5N57H_MOrz4apQR4pKaamzskzjLxO55gpbmZFC76qWuUdexAR7DT2fpbHLOw90atN_NlLMY-VrXyW3-Ei5EhYaVreMB9PSpKwkrA4jULITohV-sxpa1LA"

// 'default/demo'
const goodJWT_B = "eyJhbGciOiJSUzI1NiIsImtpZCI6IiJ9.eyJpc3MiOiJrdWJlcm5ldGVzL3NlcnZpY2VhY2NvdW50Iiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9uYW1lc3BhY2UiOiJkZWZhdWx0Iiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9zZWNyZXQubmFtZSI6ImRlbW8tdG9rZW4ta21iOW4iLCJrdWJlcm5ldGVzLmlvL3NlcnZpY2VhY2NvdW50L3NlcnZpY2UtYWNjb3VudC5uYW1lIjoiZGVtbyIsImt1YmVybmV0ZXMuaW8vc2VydmljZWFjY291bnQvc2VydmljZS1hY2NvdW50LnVpZCI6Ijc2MDkxYWY0LTRiNTYtMTFlOS1hYzRiLTcwOGIxMTgwMWNiZSIsInN1YiI6InN5c3RlbTpzZXJ2aWNlYWNjb3VudDpkZWZhdWx0OmRlbW8ifQ.ZiAHjijBAOsKdum0Aix6lgtkLkGo9_Tu87dWQ5Zfwnn3r2FejEWDAnftTft1MqqnMzivZ9Wyyki5ZjQRmTAtnMPJuHC-iivqY4Wh4S6QWCJ1SivBv5tMZR79t5t8mE7R1-OHwst46spru1pps9wt9jsA04d3LpV0eeKYgdPTVaQKklxTm397kIMUugA6yINIBQ3Rh8eQqBgNwEmL4iqyYubzHLVkGkoP9MJikFI05vfRiHtYr-piXz6JFDzXMQj9rW6xtMmrBSn79ChbyvC5nz-Nj2rJPnHsb_0rDUbmXY5PpnMhBpdSH-CbZ4j8jsiib6DtaGJhVZeEQ1GjsFAZwQ"
