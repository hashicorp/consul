package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hashicorp/go-uuid"
	"github.com/stretchr/testify/require"
	"gopkg.in/square/go-jose.v2/jwt"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/authmethod/testauth"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/internal/go-sso/oidcauth/oidcauthtest"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/testrpc"
)

// NOTE: The tests contained herein are designed to test the HTTP API
//       They are not intended to thoroughly test the backing RPC
//       functionality as that will be done with other tests.

func isHTTPBadRequest(err error) bool {
	if err, ok := err.(HTTPError); ok {
		if err.StatusCode != 400 {
			return false
		}
		return true
	}
	return false
}

func TestACL_Disabled_Response(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	type testCase struct {
		name string
		fn   func(resp http.ResponseWriter, req *http.Request) (interface{}, error)
	}

	tests := []testCase{
		{"ACLBootstrap", a.srv.ACLBootstrap},
		{"ACLReplicationStatus", a.srv.ACLReplicationStatus},
		{"AgentToken", a.srv.AgentToken}, // See TestAgent_Token
		{"ACLPolicyList", a.srv.ACLPolicyList},
		{"ACLPolicyCRUD", a.srv.ACLPolicyCRUD},
		{"ACLPolicyCreate", a.srv.ACLPolicyCreate},
		{"ACLTokenList", a.srv.ACLTokenList},
		{"ACLTokenCreate", a.srv.ACLTokenCreate},
		{"ACLTokenSelf", a.srv.ACLTokenSelf},
		{"ACLTokenCRUD", a.srv.ACLTokenCRUD},
		{"ACLRoleList", a.srv.ACLRoleList},
		{"ACLRoleCreate", a.srv.ACLRoleCreate},
		{"ACLRoleCRUD", a.srv.ACLRoleCRUD},
		{"ACLBindingRuleList", a.srv.ACLBindingRuleList},
		{"ACLBindingRuleCreate", a.srv.ACLBindingRuleCreate},
		{"ACLBindingRuleCRUD", a.srv.ACLBindingRuleCRUD},
		{"ACLAuthMethodList", a.srv.ACLAuthMethodList},
		{"ACLAuthMethodCreate", a.srv.ACLAuthMethodCreate},
		{"ACLAuthMethodCRUD", a.srv.ACLAuthMethodCRUD},
		{"ACLLogin", a.srv.ACLLogin},
		{"ACLLogout", a.srv.ACLLogout},
		{"ACLAuthorize", a.srv.ACLAuthorize},
	}
	testrpc.WaitForLeader(t, a.RPC, "dc1")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("PUT", "/should/not/care", nil)
			resp := httptest.NewRecorder()
			obj, err := tt.fn(resp, req)
			require.Nil(t, obj)
			require.ErrorIs(t, err, HTTPError{StatusCode: http.StatusUnauthorized, Reason: "ACL support disabled"})
		})
	}
}

func jsonBody(v interface{}) io.Reader {
	body := bytes.NewBuffer(nil)
	enc := json.NewEncoder(body)
	enc.Encode(v)
	return body
}

func TestACL_Bootstrap(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, `
		primary_datacenter = "dc1"

		acl {
			enabled = true
			default_policy = "deny"
		}
	`)
	defer a.Shutdown()

	tests := []struct {
		name   string
		method string
		code   int
		token  bool
	}{
		{"bootstrap", "PUT", http.StatusOK, true},
		{"not again", "PUT", http.StatusForbidden, false},
	}
	testrpc.WaitForLeader(t, a.RPC, "dc1")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := httptest.NewRecorder()
			req, _ := http.NewRequest(tt.method, "/v1/acl/bootstrap", nil)
			out, err := a.srv.ACLBootstrap(resp, req)
			if tt.token && err != nil {
				t.Fatalf("err: %v", err)
			}
			if tt.token {
				wrap, ok := out.(*aclBootstrapResponse)
				if !ok {
					t.Fatalf("bad: %T", out)
				}
				if len(wrap.ID) != len("xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx") {
					t.Fatalf("bad: %v", wrap)
				}
				if wrap.ID != wrap.SecretID {
					t.Fatalf("bad: %v", wrap)
				}
			} else {
				if out != nil {
					t.Fatalf("bad: %T", out)
				}
			}
		})
	}
}

func TestACL_HTTP(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, TestACLConfig())
	defer a.Shutdown()

	testrpc.WaitForLeader(t, a.RPC, "dc1")

	idMap := make(map[string]string)
	policyMap := make(map[string]*structs.ACLPolicy)
	roleMap := make(map[string]*structs.ACLRole)
	tokenMap := make(map[string]*structs.ACLToken)

	// This is all done as a subtest for a couple reasons
	// 1. It uses only 1 test agent and these are
	//    somewhat expensive to bring up and tear down often
	// 2. Instead of having to bring up a new agent and prime
	//    the ACL system with some data before running the test
	//    we can intelligently order these tests so we can still
	//    test everything with less actual operations and do
	//    so in a manner that is less prone to being flaky
	//
	// This could be accomplished with just blocks of code but I find
	// the go test output nicer to pinpoint the error if they are grouped.
	//
	// NOTE: None of the subtests should be parallelized in order for
	// any of it to work properly.
	t.Run("Policy", func(t *testing.T) {
		t.Run("Create", func(t *testing.T) {
			policyInput := &structs.ACLPolicy{
				Name:        "test",
				Description: "test",
				Rules:       `acl = "read"`,
				Datacenters: []string{"dc1"},
			}

			req, _ := http.NewRequest("PUT", "/v1/acl/policy?token=root", jsonBody(policyInput))
			resp := httptest.NewRecorder()
			obj, err := a.srv.ACLPolicyCreate(resp, req)
			require.NoError(t, err)

			policy, ok := obj.(*structs.ACLPolicy)
			require.True(t, ok)

			// 36 = length of the string form of uuids
			require.Len(t, policy.ID, 36)
			require.Equal(t, policyInput.Name, policy.Name)
			require.Equal(t, policyInput.Description, policy.Description)
			require.Equal(t, policyInput.Rules, policy.Rules)
			require.Equal(t, policyInput.Datacenters, policy.Datacenters)
			require.True(t, policy.CreateIndex > 0)
			require.Equal(t, policy.CreateIndex, policy.ModifyIndex)
			require.NotNil(t, policy.Hash)
			require.NotEqual(t, policy.Hash, []byte{})

			idMap["policy-test"] = policy.ID
			policyMap[policy.ID] = policy
		})

		t.Run("Minimal", func(t *testing.T) {
			policyInput := &structs.ACLPolicy{
				Name:  "minimal",
				Rules: `key_prefix "" { policy = "read" }`,
			}

			req, _ := http.NewRequest("PUT", "/v1/acl/policy?token=root", jsonBody(policyInput))
			resp := httptest.NewRecorder()
			obj, err := a.srv.ACLPolicyCreate(resp, req)
			require.NoError(t, err)

			policy, ok := obj.(*structs.ACLPolicy)
			require.True(t, ok)

			// 36 = length of the string form of uuids
			require.Len(t, policy.ID, 36)
			require.Equal(t, policyInput.Name, policy.Name)
			require.Equal(t, policyInput.Description, policy.Description)
			require.Equal(t, policyInput.Rules, policy.Rules)
			require.Equal(t, policyInput.Datacenters, policy.Datacenters)
			require.True(t, policy.CreateIndex > 0)
			require.Equal(t, policy.CreateIndex, policy.ModifyIndex)
			require.NotNil(t, policy.Hash)
			require.NotEqual(t, policy.Hash, []byte{})

			idMap["policy-minimal"] = policy.ID
			policyMap[policy.ID] = policy
		})

		t.Run("Name Chars", func(t *testing.T) {
			policyInput := &structs.ACLPolicy{
				Name:  "read-all_nodes-012",
				Rules: `node_prefix "" { policy = "read" }`,
			}

			req, _ := http.NewRequest("PUT", "/v1/acl/policy?token=root", jsonBody(policyInput))
			resp := httptest.NewRecorder()
			obj, err := a.srv.ACLPolicyCreate(resp, req)
			require.NoError(t, err)

			policy, ok := obj.(*structs.ACLPolicy)
			require.True(t, ok)

			// 36 = length of the string form of uuids
			require.Len(t, policy.ID, 36)
			require.Equal(t, policyInput.Name, policy.Name)
			require.Equal(t, policyInput.Description, policy.Description)
			require.Equal(t, policyInput.Rules, policy.Rules)
			require.Equal(t, policyInput.Datacenters, policy.Datacenters)
			require.True(t, policy.CreateIndex > 0)
			require.Equal(t, policy.CreateIndex, policy.ModifyIndex)
			require.NotNil(t, policy.Hash)
			require.NotEqual(t, policy.Hash, []byte{})

			idMap["policy-read-all-nodes"] = policy.ID
			policyMap[policy.ID] = policy
		})

		t.Run("Update Name ID Mismatch", func(t *testing.T) {
			policyInput := &structs.ACLPolicy{
				ID:          "ac7560be-7f11-4d6d-bfcf-15633c2090fd",
				Name:        "read-all-nodes",
				Description: "Can read all node information",
				Rules:       `node_prefix "" { policy = "read" }`,
				Datacenters: []string{"dc1"},
			}

			req, _ := http.NewRequest("PUT", "/v1/acl/policy/"+idMap["policy-read-all-nodes"]+"?token=root", jsonBody(policyInput))
			resp := httptest.NewRecorder()
			_, err := a.srv.ACLPolicyCRUD(resp, req)
			require.Error(t, err)
			require.True(t, isHTTPBadRequest(err))
		})

		t.Run("Policy CRUD Missing ID in URL", func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/v1/acl/policy/?token=root", nil)
			resp := httptest.NewRecorder()
			_, err := a.srv.ACLPolicyCRUD(resp, req)
			require.Error(t, err)
			require.True(t, isHTTPBadRequest(err))
		})

		t.Run("Update", func(t *testing.T) {
			policyInput := &structs.ACLPolicy{
				Name:        "read-all-nodes",
				Description: "Can read all node information",
				Rules:       `node_prefix "" { policy = "read" }`,
				Datacenters: []string{"dc1"},
			}

			req, _ := http.NewRequest("PUT", "/v1/acl/policy/"+idMap["policy-read-all-nodes"]+"?token=root", jsonBody(policyInput))
			resp := httptest.NewRecorder()
			obj, err := a.srv.ACLPolicyCRUD(resp, req)
			require.NoError(t, err)

			policy, ok := obj.(*structs.ACLPolicy)
			require.True(t, ok)

			// 36 = length of the string form of uuids
			require.Len(t, policy.ID, 36)
			require.Equal(t, policyInput.Name, policy.Name)
			require.Equal(t, policyInput.Description, policy.Description)
			require.Equal(t, policyInput.Rules, policy.Rules)
			require.Equal(t, policyInput.Datacenters, policy.Datacenters)
			require.True(t, policy.CreateIndex > 0)
			require.True(t, policy.CreateIndex < policy.ModifyIndex)
			require.NotNil(t, policy.Hash)
			require.NotEqual(t, policy.Hash, []byte{})

			idMap["policy-read-all-nodes"] = policy.ID
			policyMap[policy.ID] = policy
		})

		t.Run("ID Supplied", func(t *testing.T) {
			policyInput := &structs.ACLPolicy{
				ID:          "12123d01-37f1-47e6-b55b-32328652bd38",
				Name:        "with-id",
				Description: "test",
				Rules:       `acl = "read"`,
				Datacenters: []string{"dc1"},
			}

			req, _ := http.NewRequest("PUT", "/v1/acl/policy?token=root", jsonBody(policyInput))
			resp := httptest.NewRecorder()
			_, err := a.srv.ACLPolicyCreate(resp, req)
			require.Error(t, err)
			require.True(t, isHTTPBadRequest(err))
		})

		t.Run("Invalid payload", func(t *testing.T) {
			body := bytes.NewBuffer(nil)
			body.Write([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

			req, _ := http.NewRequest("PUT", "/v1/acl/policy?token=root", body)
			resp := httptest.NewRecorder()
			_, err := a.srv.ACLPolicyCreate(resp, req)
			require.Error(t, err)
			require.True(t, isHTTPBadRequest(err))
		})

		t.Run("Delete", func(t *testing.T) {
			req, _ := http.NewRequest("DELETE", "/v1/acl/policy/"+idMap["policy-minimal"]+"?token=root", nil)
			resp := httptest.NewRecorder()
			_, err := a.srv.ACLPolicyCRUD(resp, req)
			require.NoError(t, err)
			delete(policyMap, idMap["policy-minimal"])
			delete(idMap, "policy-minimal")
		})

		t.Run("List", func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/v1/acl/policies?token=root", nil)
			resp := httptest.NewRecorder()
			raw, err := a.srv.ACLPolicyList(resp, req)
			require.NoError(t, err)
			policies, ok := raw.(structs.ACLPolicyListStubs)
			require.True(t, ok)

			// 2 we just created + global management
			require.Len(t, policies, 3)

			for policyID, expected := range policyMap {
				found := false
				for _, actual := range policies {
					if actual.ID == policyID {
						require.Equal(t, expected.Name, actual.Name)
						require.Equal(t, expected.Datacenters, actual.Datacenters)
						require.Equal(t, expected.Hash, actual.Hash)
						require.Equal(t, expected.CreateIndex, actual.CreateIndex)
						require.Equal(t, expected.ModifyIndex, actual.ModifyIndex)
						found = true
						break
					}
				}

				require.True(t, found)
			}
		})

		t.Run("Read", func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/v1/acl/policy/"+idMap["policy-read-all-nodes"]+"?token=root", nil)
			resp := httptest.NewRecorder()
			raw, err := a.srv.ACLPolicyCRUD(resp, req)
			require.NoError(t, err)
			policy, ok := raw.(*structs.ACLPolicy)
			require.True(t, ok)
			require.Equal(t, policyMap[idMap["policy-read-all-nodes"]], policy)
		})

		t.Run("Read Name", func(t *testing.T) {
			policyName := "read-all-nodes"
			req, _ := http.NewRequest("GET", "/v1/acl/policy/name/"+policyName+"?token=root", nil)
			resp := httptest.NewRecorder()
			raw, err := a.srv.ACLPolicyReadByName(resp, req)
			require.NoError(t, err)
			policy, ok := raw.(*structs.ACLPolicy)
			require.True(t, ok)
			require.Equal(t, policyMap[idMap["policy-"+policyName]], policy)
		})
	})

	t.Run("Role", func(t *testing.T) {
		t.Run("Create", func(t *testing.T) {
			roleInput := &structs.ACLRole{
				Name:        "test",
				Description: "test",
				Policies: []structs.ACLRolePolicyLink{
					{
						ID:   idMap["policy-test"],
						Name: policyMap[idMap["policy-test"]].Name,
					},
					{
						ID:   idMap["policy-read-all-nodes"],
						Name: policyMap[idMap["policy-read-all-nodes"]].Name,
					},
				},
				NodeIdentities: []*structs.ACLNodeIdentity{
					{
						NodeName:   "web-node",
						Datacenter: "foo",
					},
				},
			}

			req, _ := http.NewRequest("PUT", "/v1/acl/role?token=root", jsonBody(roleInput))
			resp := httptest.NewRecorder()
			obj, err := a.srv.ACLRoleCreate(resp, req)
			require.NoError(t, err)

			role, ok := obj.(*structs.ACLRole)
			require.True(t, ok)

			// 36 = length of the string form of uuids
			require.Len(t, role.ID, 36)
			require.Equal(t, roleInput.Name, role.Name)
			require.Equal(t, roleInput.Description, role.Description)
			require.Equal(t, roleInput.Policies, role.Policies)
			require.Equal(t, roleInput.NodeIdentities, role.NodeIdentities)
			require.True(t, role.CreateIndex > 0)
			require.Equal(t, role.CreateIndex, role.ModifyIndex)
			require.NotNil(t, role.Hash)
			require.NotEqual(t, role.Hash, []byte{})

			idMap["role-test"] = role.ID
			roleMap[role.ID] = role
		})

		t.Run("Name Chars", func(t *testing.T) {
			roleInput := &structs.ACLRole{
				Name: "service-id-web",
				ServiceIdentities: []*structs.ACLServiceIdentity{
					{
						ServiceName: "web",
					},
				},
			}

			req, _ := http.NewRequest("PUT", "/v1/acl/role?token=root", jsonBody(roleInput))
			resp := httptest.NewRecorder()
			obj, err := a.srv.ACLRoleCreate(resp, req)
			require.NoError(t, err)

			role, ok := obj.(*structs.ACLRole)
			require.True(t, ok)

			// 36 = length of the string form of uuids
			require.Len(t, role.ID, 36)
			require.Equal(t, roleInput.Name, role.Name)
			require.Equal(t, roleInput.Description, role.Description)
			require.Equal(t, roleInput.ServiceIdentities, role.ServiceIdentities)
			require.True(t, role.CreateIndex > 0)
			require.Equal(t, role.CreateIndex, role.ModifyIndex)
			require.NotNil(t, role.Hash)
			require.NotEqual(t, role.Hash, []byte{})

			idMap["role-service-id-web"] = role.ID
			roleMap[role.ID] = role
		})

		t.Run("Update Name ID Mismatch", func(t *testing.T) {
			roleInput := &structs.ACLRole{
				ID:          "ac7560be-7f11-4d6d-bfcf-15633c2090fd",
				Name:        "test",
				Description: "test",
				ServiceIdentities: []*structs.ACLServiceIdentity{
					{
						ServiceName: "db",
					},
				},
			}

			req, _ := http.NewRequest("PUT", "/v1/acl/role/"+idMap["role-test"]+"?token=root", jsonBody(roleInput))
			resp := httptest.NewRecorder()
			_, err := a.srv.ACLRoleCRUD(resp, req)
			require.Error(t, err)
			require.True(t, isHTTPBadRequest(err))
		})

		t.Run("Role CRUD Missing ID in URL", func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/v1/acl/role/?token=root", nil)
			resp := httptest.NewRecorder()
			_, err := a.srv.ACLRoleCRUD(resp, req)
			require.Error(t, err)
			require.True(t, isHTTPBadRequest(err))
		})

		t.Run("Update", func(t *testing.T) {
			roleInput := &structs.ACLRole{
				Name:        "test",
				Description: "test",
				ServiceIdentities: []*structs.ACLServiceIdentity{
					{
						ServiceName: "web-indexer",
					},
				},
				NodeIdentities: []*structs.ACLNodeIdentity{
					{
						NodeName:   "web-node",
						Datacenter: "foo",
					},
				},
			}

			req, _ := http.NewRequest("PUT", "/v1/acl/role/"+idMap["role-test"]+"?token=root", jsonBody(roleInput))
			resp := httptest.NewRecorder()
			obj, err := a.srv.ACLRoleCRUD(resp, req)
			require.NoError(t, err)

			role, ok := obj.(*structs.ACLRole)
			require.True(t, ok)

			// 36 = length of the string form of uuids
			require.Len(t, role.ID, 36)
			require.Equal(t, roleInput.Name, role.Name)
			require.Equal(t, roleInput.Description, role.Description)
			require.Equal(t, roleInput.Policies, role.Policies)
			require.Equal(t, roleInput.ServiceIdentities, role.ServiceIdentities)
			require.Equal(t, roleInput.NodeIdentities, role.NodeIdentities)
			require.True(t, role.CreateIndex > 0)
			require.True(t, role.CreateIndex < role.ModifyIndex)
			require.NotNil(t, role.Hash)
			require.NotEqual(t, role.Hash, []byte{})

			idMap["role-test"] = role.ID
			roleMap[role.ID] = role
		})

		t.Run("ID Supplied", func(t *testing.T) {
			roleInput := &structs.ACLRole{
				ID:          "12123d01-37f1-47e6-b55b-32328652bd38",
				Name:        "with-id",
				Description: "test",
				ServiceIdentities: []*structs.ACLServiceIdentity{
					{
						ServiceName: "foobar",
					},
				},
			}

			req, _ := http.NewRequest("PUT", "/v1/acl/role?token=root", jsonBody(roleInput))
			resp := httptest.NewRecorder()
			_, err := a.srv.ACLRoleCreate(resp, req)
			require.Error(t, err)
			require.True(t, isHTTPBadRequest(err))
		})

		t.Run("Invalid payload", func(t *testing.T) {
			body := bytes.NewBuffer(nil)
			body.Write([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

			req, _ := http.NewRequest("PUT", "/v1/acl/role?token=root", body)
			resp := httptest.NewRecorder()
			_, err := a.srv.ACLRoleCreate(resp, req)
			require.Error(t, err)
			require.True(t, isHTTPBadRequest(err))
		})

		t.Run("Delete", func(t *testing.T) {
			req, _ := http.NewRequest("DELETE", "/v1/acl/role/"+idMap["role-service-id-web"]+"?token=root", nil)
			resp := httptest.NewRecorder()
			_, err := a.srv.ACLRoleCRUD(resp, req)
			require.NoError(t, err)
			delete(roleMap, idMap["role-service-id-web"])
			delete(idMap, "role-service-id-web")
		})

		t.Run("List", func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/v1/acl/roles?token=root", nil)
			resp := httptest.NewRecorder()
			raw, err := a.srv.ACLRoleList(resp, req)
			require.NoError(t, err)
			roles, ok := raw.(structs.ACLRoles)
			require.True(t, ok)

			// 1 we just created
			require.Len(t, roles, 1)

			for roleID, expected := range roleMap {
				found := false
				for _, actual := range roles {
					if actual.ID == roleID {
						require.Equal(t, expected.Name, actual.Name)
						require.Equal(t, expected.Policies, actual.Policies)
						require.Equal(t, expected.ServiceIdentities, actual.ServiceIdentities)
						require.Equal(t, expected.Hash, actual.Hash)
						require.Equal(t, expected.CreateIndex, actual.CreateIndex)
						require.Equal(t, expected.ModifyIndex, actual.ModifyIndex)
						found = true
						break
					}
				}

				require.True(t, found)
			}
		})

		t.Run("Read", func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/v1/acl/role/"+idMap["role-test"]+"?token=root", nil)
			resp := httptest.NewRecorder()
			raw, err := a.srv.ACLRoleCRUD(resp, req)
			require.NoError(t, err)
			role, ok := raw.(*structs.ACLRole)
			require.True(t, ok)
			require.Equal(t, roleMap[idMap["role-test"]], role)
		})
	})

	t.Run("Token", func(t *testing.T) {
		t.Run("Create", func(t *testing.T) {
			tokenInput := &structs.ACLToken{
				Description: "test",
				Policies: []structs.ACLTokenPolicyLink{
					{
						ID:   idMap["policy-test"],
						Name: policyMap[idMap["policy-test"]].Name,
					},
					{
						ID:   idMap["policy-read-all-nodes"],
						Name: policyMap[idMap["policy-read-all-nodes"]].Name,
					},
				},
				NodeIdentities: []*structs.ACLNodeIdentity{
					{
						NodeName:   "foo",
						Datacenter: "bar",
					},
				},
			}

			req, _ := http.NewRequest("PUT", "/v1/acl/token?token=root", jsonBody(tokenInput))
			resp := httptest.NewRecorder()
			obj, err := a.srv.ACLTokenCreate(resp, req)
			require.NoError(t, err)

			token, ok := obj.(*structs.ACLToken)
			require.True(t, ok)

			// 36 = length of the string form of uuids
			require.Len(t, token.AccessorID, 36)
			require.Len(t, token.SecretID, 36)
			require.Equal(t, tokenInput.Description, token.Description)
			require.Equal(t, tokenInput.Policies, token.Policies)
			require.Equal(t, tokenInput.NodeIdentities, token.NodeIdentities)
			require.True(t, token.CreateIndex > 0)
			require.Equal(t, token.CreateIndex, token.ModifyIndex)
			require.NotNil(t, token.Hash)
			require.NotEqual(t, token.Hash, []byte{})

			idMap["token-test"] = token.AccessorID
			tokenMap[token.AccessorID] = token
		})
		t.Run("Create Local", func(t *testing.T) {
			tokenInput := &structs.ACLToken{
				Description: "local",
				Policies: []structs.ACLTokenPolicyLink{
					{
						ID:   idMap["policy-test"],
						Name: policyMap[idMap["policy-test"]].Name,
					},
					{
						ID:   idMap["policy-read-all-nodes"],
						Name: policyMap[idMap["policy-read-all-nodes"]].Name,
					},
				},
				Local: true,
			}

			req, _ := http.NewRequest("PUT", "/v1/acl/token?token=root", jsonBody(tokenInput))
			resp := httptest.NewRecorder()
			obj, err := a.srv.ACLTokenCreate(resp, req)
			require.NoError(t, err)

			token, ok := obj.(*structs.ACLToken)
			require.True(t, ok)

			// 36 = length of the string form of uuids
			require.Len(t, token.AccessorID, 36)
			require.Len(t, token.SecretID, 36)
			require.Equal(t, tokenInput.Description, token.Description)
			require.Equal(t, tokenInput.Policies, token.Policies)
			require.True(t, token.CreateIndex > 0)
			require.Equal(t, token.CreateIndex, token.ModifyIndex)
			require.NotNil(t, token.Hash)
			require.NotEqual(t, token.Hash, []byte{})

			idMap["token-local"] = token.AccessorID
			tokenMap[token.AccessorID] = token
		})
		t.Run("Read", func(t *testing.T) {
			expected := tokenMap[idMap["token-test"]]
			req, _ := http.NewRequest("GET", "/v1/acl/token/"+expected.AccessorID+"?token=root", nil)
			resp := httptest.NewRecorder()
			obj, err := a.srv.ACLTokenCRUD(resp, req)
			require.NoError(t, err)
			token, ok := obj.(*structs.ACLToken)
			require.True(t, ok)
			require.Equal(t, expected, token)
		})
		t.Run("Read-expanded", func(t *testing.T) {
			expected := tokenMap[idMap["token-test"]]
			req, _ := http.NewRequest("GET", "/v1/acl/token/"+expected.AccessorID+"?token=root&expanded=true", nil)
			resp := httptest.NewRecorder()
			obj, err := a.srv.ACLTokenCRUD(resp, req)
			require.NoError(t, err)
			tokenResp, ok := obj.(*structs.ACLTokenExpanded)
			require.True(t, ok)
			require.Equal(t, expected, tokenResp.ACLToken)
			require.Len(t, tokenResp.ExpandedPolicies, 3)
		})
		t.Run("Self", func(t *testing.T) {
			expected := tokenMap[idMap["token-test"]]
			req, _ := http.NewRequest("GET", "/v1/acl/token/self?token="+expected.SecretID, nil)
			resp := httptest.NewRecorder()
			obj, err := a.srv.ACLTokenSelf(resp, req)
			require.NoError(t, err)
			token, ok := obj.(*structs.ACLToken)
			require.True(t, ok)
			require.Equal(t, expected, token)
		})
		t.Run("Clone", func(t *testing.T) {
			tokenInput := &structs.ACLToken{
				Description: "cloned token",
			}

			baseToken := tokenMap[idMap["token-test"]]

			req, _ := http.NewRequest("PUT", "/v1/acl/token/"+baseToken.AccessorID+"/clone?token=root", jsonBody(tokenInput))
			resp := httptest.NewRecorder()
			obj, err := a.srv.ACLTokenCRUD(resp, req)
			require.NoError(t, err)
			token, ok := obj.(*structs.ACLToken)
			require.True(t, ok)

			require.NotEqual(t, baseToken.AccessorID, token.AccessorID)
			require.NotEqual(t, baseToken.SecretID, token.SecretID)
			require.Equal(t, tokenInput.Description, token.Description)
			require.Equal(t, baseToken.Policies, token.Policies)
			require.True(t, token.CreateIndex > 0)
			require.Equal(t, token.CreateIndex, token.ModifyIndex)
			require.NotNil(t, token.Hash)
			require.NotEqual(t, token.Hash, []byte{})

			idMap["token-cloned"] = token.AccessorID
			tokenMap[token.AccessorID] = token
		})
		t.Run("Update", func(t *testing.T) {
			originalToken := tokenMap[idMap["token-cloned"]]

			// Accessor and Secret will be filled in
			tokenInput := &structs.ACLToken{
				Description: "Better description for this cloned token",
				Policies: []structs.ACLTokenPolicyLink{
					{
						ID:   idMap["policy-read-all-nodes"],
						Name: policyMap[idMap["policy-read-all-nodes"]].Name,
					},
				},
				NodeIdentities: []*structs.ACLNodeIdentity{
					{
						NodeName:   "foo",
						Datacenter: "bar",
					},
				},
			}

			req, _ := http.NewRequest("PUT", "/v1/acl/token/"+originalToken.AccessorID+"?token=root", jsonBody(tokenInput))
			resp := httptest.NewRecorder()
			obj, err := a.srv.ACLTokenCRUD(resp, req)
			require.NoError(t, err)
			token, ok := obj.(*structs.ACLToken)
			require.True(t, ok)

			require.Equal(t, originalToken.AccessorID, token.AccessorID)
			require.Equal(t, originalToken.SecretID, token.SecretID)
			require.Equal(t, tokenInput.Description, token.Description)
			require.Equal(t, tokenInput.Policies, token.Policies)
			require.Equal(t, tokenInput.NodeIdentities, token.NodeIdentities)
			require.True(t, token.CreateIndex > 0)
			require.True(t, token.CreateIndex < token.ModifyIndex)
			require.NotNil(t, token.Hash)
			require.NotEqual(t, token.Hash, []byte{})
			require.NotEqual(t, token.Hash, originalToken.Hash)

			tokenMap[token.AccessorID] = token
		})

		t.Run("CRUD Missing Token Accessor ID", func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/v1/acl/token/?token=root", nil)
			resp := httptest.NewRecorder()
			obj, err := a.srv.ACLTokenCRUD(resp, req)
			require.Error(t, err)
			require.Nil(t, obj)
			require.True(t, isHTTPBadRequest(err))
		})
		t.Run("Update Accessor Mismatch", func(t *testing.T) {
			originalToken := tokenMap[idMap["token-cloned"]]

			// Accessor and Secret will be filled in
			tokenInput := &structs.ACLToken{
				AccessorID:  "e8aeb69a-0ace-42b9-b95f-d1d9eafe1561",
				Description: "Better description for this cloned token",
				Policies: []structs.ACLTokenPolicyLink{
					{
						ID:   idMap["policy-read-all-nodes"],
						Name: policyMap[idMap["policy-read-all-nodes"]].Name,
					},
				},
			}

			req, _ := http.NewRequest("PUT", "/v1/acl/token/"+originalToken.AccessorID+"?token=root", jsonBody(tokenInput))
			resp := httptest.NewRecorder()
			obj, err := a.srv.ACLTokenCRUD(resp, req)
			require.Error(t, err)
			require.Nil(t, obj)
			require.True(t, isHTTPBadRequest(err))
		})
		t.Run("Delete", func(t *testing.T) {
			req, _ := http.NewRequest("DELETE", "/v1/acl/token/"+idMap["token-cloned"]+"?token=root", nil)
			resp := httptest.NewRecorder()
			_, err := a.srv.ACLTokenCRUD(resp, req)
			require.NoError(t, err)
			delete(tokenMap, idMap["token-cloned"])
			delete(idMap, "token-cloned")
		})
		t.Run("List", func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/v1/acl/tokens?token=root", nil)
			resp := httptest.NewRecorder()
			raw, err := a.srv.ACLTokenList(resp, req)
			require.NoError(t, err)
			tokens, ok := raw.(structs.ACLTokenListStubs)
			require.True(t, ok)

			// 3 tokens created but 1 was deleted + initial management token + anon token
			require.Len(t, tokens, 4)

			// this loop doesn't verify anything about the initial management token
			for tokenID, expected := range tokenMap {
				found := false
				for _, actual := range tokens {
					if actual.AccessorID == tokenID {
						require.Equal(t, expected.SecretID, actual.SecretID)
						require.Equal(t, expected.Description, actual.Description)
						require.Equal(t, expected.Policies, actual.Policies)
						require.Equal(t, expected.Local, actual.Local)
						require.Equal(t, expected.CreateTime, actual.CreateTime)
						require.Equal(t, expected.Hash, actual.Hash)
						require.Equal(t, expected.CreateIndex, actual.CreateIndex)
						require.Equal(t, expected.ModifyIndex, actual.ModifyIndex)
						found = true
						break
					}
				}
				require.True(t, found)
			}
		})
		t.Run("List by Policy", func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/v1/acl/tokens?token=root&policy="+structs.ACLPolicyGlobalManagementID, nil)
			resp := httptest.NewRecorder()
			raw, err := a.srv.ACLTokenList(resp, req)
			require.NoError(t, err)
			tokens, ok := raw.(structs.ACLTokenListStubs)
			require.True(t, ok)
			require.Len(t, tokens, 1)
			token := tokens[0]
			require.Equal(t, "Initial Management Token", token.Description)
			require.Len(t, token.Policies, 1)
			require.Equal(t, structs.ACLPolicyGlobalManagementID, token.Policies[0].ID)
		})
		t.Run("Create with Accessor", func(t *testing.T) {
			tokenInput := &structs.ACLToken{
				AccessorID:  "56e8e6a3-708b-4a2f-8ab3-b973cce39108",
				Description: "test",
				Policies: []structs.ACLTokenPolicyLink{
					{
						ID:   idMap["policy-test"],
						Name: policyMap[idMap["policy-test"]].Name,
					},
					{
						ID:   idMap["policy-read-all-nodes"],
						Name: policyMap[idMap["policy-read-all-nodes"]].Name,
					},
				},
			}

			req, _ := http.NewRequest("PUT", "/v1/acl/token?token=root", jsonBody(tokenInput))
			resp := httptest.NewRecorder()
			obj, err := a.srv.ACLTokenCreate(resp, req)
			require.NoError(t, err)

			token, ok := obj.(*structs.ACLToken)
			require.True(t, ok)

			// 36 = length of the string form of uuids
			require.Equal(t, tokenInput.AccessorID, token.AccessorID)
			require.Len(t, token.SecretID, 36)
			require.Equal(t, tokenInput.Description, token.Description)
			require.Equal(t, tokenInput.Policies, token.Policies)
			require.True(t, token.CreateIndex > 0)
			require.Equal(t, token.CreateIndex, token.ModifyIndex)
			require.NotNil(t, token.Hash)
			require.NotEqual(t, token.Hash, []byte{})

			idMap["token-test"] = token.AccessorID
			tokenMap[token.AccessorID] = token
		})

		t.Run("Create with Secret", func(t *testing.T) {
			tokenInput := &structs.ACLToken{
				SecretID:    "4e3efd15-d06c-442e-a7cc-1744f55c8dea",
				Description: "test",
				Policies: []structs.ACLTokenPolicyLink{
					{
						ID:   idMap["policy-test"],
						Name: policyMap[idMap["policy-test"]].Name,
					},
					{
						ID:   idMap["policy-read-all-nodes"],
						Name: policyMap[idMap["policy-read-all-nodes"]].Name,
					},
				},
			}

			req, _ := http.NewRequest("PUT", "/v1/acl/token?token=root", jsonBody(tokenInput))
			resp := httptest.NewRecorder()
			obj, err := a.srv.ACLTokenCreate(resp, req)
			require.NoError(t, err)

			token, ok := obj.(*structs.ACLToken)
			require.True(t, ok)

			// 36 = length of the string form of uuids
			require.Equal(t, tokenInput.SecretID, token.SecretID)
			require.Len(t, token.AccessorID, 36)
			require.Equal(t, tokenInput.Description, token.Description)
			require.Equal(t, tokenInput.Policies, token.Policies)
			require.True(t, token.CreateIndex > 0)
			require.Equal(t, token.CreateIndex, token.ModifyIndex)
			require.NotNil(t, token.Hash)
			require.NotEqual(t, token.Hash, []byte{})

			idMap["token-test"] = token.AccessorID
			tokenMap[token.AccessorID] = token
		})

		t.Run("Create with Accessor and Secret", func(t *testing.T) {
			tokenInput := &structs.ACLToken{
				AccessorID:  "dee863fa-e548-4c61-a96f-9aa07999249f",
				SecretID:    "10126ffa-b28f-4137-b9a9-e89ab1e97c5b",
				Description: "test",
				Policies: []structs.ACLTokenPolicyLink{
					{
						ID:   idMap["policy-test"],
						Name: policyMap[idMap["policy-test"]].Name,
					},
					{
						ID:   idMap["policy-read-all-nodes"],
						Name: policyMap[idMap["policy-read-all-nodes"]].Name,
					},
				},
			}

			req, _ := http.NewRequest("PUT", "/v1/acl/token?token=root", jsonBody(tokenInput))
			resp := httptest.NewRecorder()
			obj, err := a.srv.ACLTokenCreate(resp, req)
			require.NoError(t, err)

			token, ok := obj.(*structs.ACLToken)
			require.True(t, ok)

			// 36 = length of the string form of uuids
			require.Equal(t, tokenInput.SecretID, token.SecretID)
			require.Equal(t, tokenInput.AccessorID, token.AccessorID)
			require.Equal(t, tokenInput.Description, token.Description)
			require.Equal(t, tokenInput.Policies, token.Policies)
			require.True(t, token.CreateIndex > 0)
			require.Equal(t, token.CreateIndex, token.ModifyIndex)
			require.NotNil(t, token.Hash)
			require.NotEqual(t, token.Hash, []byte{})

			idMap["token-test"] = token.AccessorID
			tokenMap[token.AccessorID] = token
		})

		t.Run("Create with Accessor Dup", func(t *testing.T) {
			tokenInput := &structs.ACLToken{
				AccessorID:  "dee863fa-e548-4c61-a96f-9aa07999249f",
				Description: "test",
				Policies: []structs.ACLTokenPolicyLink{
					{
						ID:   idMap["policy-test"],
						Name: policyMap[idMap["policy-test"]].Name,
					},
					{
						ID:   idMap["policy-read-all-nodes"],
						Name: policyMap[idMap["policy-read-all-nodes"]].Name,
					},
				},
			}

			req, _ := http.NewRequest("PUT", "/v1/acl/token?token=root", jsonBody(tokenInput))
			resp := httptest.NewRecorder()
			_, err := a.srv.ACLTokenCreate(resp, req)
			require.Error(t, err)
		})

		t.Run("Create with Secret as Accessor Dup", func(t *testing.T) {
			tokenInput := &structs.ACLToken{
				SecretID:    "dee863fa-e548-4c61-a96f-9aa07999249f",
				Description: "test",
				Policies: []structs.ACLTokenPolicyLink{
					{
						ID:   idMap["policy-test"],
						Name: policyMap[idMap["policy-test"]].Name,
					},
					{
						ID:   idMap["policy-read-all-nodes"],
						Name: policyMap[idMap["policy-read-all-nodes"]].Name,
					},
				},
			}

			req, _ := http.NewRequest("PUT", "/v1/acl/token?token=root", jsonBody(tokenInput))
			resp := httptest.NewRecorder()
			_, err := a.srv.ACLTokenCreate(resp, req)
			require.Error(t, err)
		})

		t.Run("Create with Secret Dup", func(t *testing.T) {
			tokenInput := &structs.ACLToken{
				SecretID:    "10126ffa-b28f-4137-b9a9-e89ab1e97c5b",
				Description: "test",
				Policies: []structs.ACLTokenPolicyLink{
					{
						ID:   idMap["policy-test"],
						Name: policyMap[idMap["policy-test"]].Name,
					},
					{
						ID:   idMap["policy-read-all-nodes"],
						Name: policyMap[idMap["policy-read-all-nodes"]].Name,
					},
				},
			}

			req, _ := http.NewRequest("PUT", "/v1/acl/token?token=root", jsonBody(tokenInput))
			resp := httptest.NewRecorder()
			_, err := a.srv.ACLTokenCreate(resp, req)
			require.Error(t, err)
		})

		t.Run("Create with Accessor as Secret Dup", func(t *testing.T) {
			tokenInput := &structs.ACLToken{
				AccessorID:  "10126ffa-b28f-4137-b9a9-e89ab1e97c5b",
				Description: "test",
				Policies: []structs.ACLTokenPolicyLink{
					{
						ID:   idMap["policy-test"],
						Name: policyMap[idMap["policy-test"]].Name,
					},
					{
						ID:   idMap["policy-read-all-nodes"],
						Name: policyMap[idMap["policy-read-all-nodes"]].Name,
					},
				},
			}

			req, _ := http.NewRequest("PUT", "/v1/acl/token?token=root", jsonBody(tokenInput))
			resp := httptest.NewRecorder()
			_, err := a.srv.ACLTokenCreate(resp, req)
			require.Error(t, err)
		})

		t.Run("Create with Reserved Accessor", func(t *testing.T) {
			tokenInput := &structs.ACLToken{
				AccessorID:  "00000000-0000-0000-0000-00000000005b",
				Description: "test",
				Policies: []structs.ACLTokenPolicyLink{
					{
						ID:   idMap["policy-test"],
						Name: policyMap[idMap["policy-test"]].Name,
					},
					{
						ID:   idMap["policy-read-all-nodes"],
						Name: policyMap[idMap["policy-read-all-nodes"]].Name,
					},
				},
			}

			req, _ := http.NewRequest("PUT", "/v1/acl/token?token=root", jsonBody(tokenInput))
			resp := httptest.NewRecorder()
			_, err := a.srv.ACLTokenCreate(resp, req)
			require.Error(t, err)
		})

		t.Run("Create with Reserved Secret", func(t *testing.T) {
			tokenInput := &structs.ACLToken{
				SecretID:    "00000000-0000-0000-0000-00000000005b",
				Description: "test",
				Policies: []structs.ACLTokenPolicyLink{
					{
						ID:   idMap["policy-test"],
						Name: policyMap[idMap["policy-test"]].Name,
					},
					{
						ID:   idMap["policy-read-all-nodes"],
						Name: policyMap[idMap["policy-read-all-nodes"]].Name,
					},
				},
			}

			req, _ := http.NewRequest("PUT", "/v1/acl/token?token=root", jsonBody(tokenInput))
			resp := httptest.NewRecorder()
			_, err := a.srv.ACLTokenCreate(resp, req)
			require.Error(t, err)
		})

		t.Run("Create with uppercase node identity", func(t *testing.T) {
			tokenInput := &structs.ACLToken{
				Description: "agent token for foo node",
				NodeIdentities: []*structs.ACLNodeIdentity{
					{
						NodeName:   "FOO",
						Datacenter: "bar",
					},
				},
			}

			req, _ := http.NewRequest("PUT", "/v1/acl/token?token=root", jsonBody(tokenInput))
			resp := httptest.NewRecorder()
			_, err := a.srv.ACLTokenCreate(resp, req)
			require.Error(t, err)
			testutil.RequireErrorContains(t, err, "Only lowercase alphanumeric")
		})

		t.Run("Create with uppercase service identity", func(t *testing.T) {
			tokenInput := &structs.ACLToken{
				Description: "token for service identity foo",
				ServiceIdentities: []*structs.ACLServiceIdentity{
					{
						ServiceName: "FOO",
					},
				},
			}

			req, _ := http.NewRequest("PUT", "/v1/acl/token?token=root", jsonBody(tokenInput))
			resp := httptest.NewRecorder()
			_, err := a.srv.ACLTokenCreate(resp, req)
			require.Error(t, err)
			testutil.RequireErrorContains(t, err, "Only lowercase alphanumeric")
		})
	})
}

func TestACL_LoginProcedure_HTTP(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	// This tests AuthMethods, BindingRules, Login, and Logout.
	t.Parallel()
	a := NewTestAgent(t, TestACLConfig())
	defer a.Shutdown()

	testrpc.WaitForLeader(t, a.RPC, "dc1")

	idMap := make(map[string]string)
	methodMap := make(map[string]*structs.ACLAuthMethod)
	ruleMap := make(map[string]*structs.ACLBindingRule)
	tokenMap := make(map[string]*structs.ACLToken)

	testSessionID := testauth.StartSession()
	defer testauth.ResetSession(testSessionID)

	// This is all done as a subtest for a couple reasons
	// 1. It uses only 1 test agent and these are
	//    somewhat expensive to bring up and tear down often
	// 2. Instead of having to bring up a new agent and prime
	//    the ACL system with some data before running the test
	//    we can intelligently order these tests so we can still
	//    test everything with less actual operations and do
	//    so in a manner that is less prone to being flaky
	// 3. While this test will be large it should
	t.Run("AuthMethod", func(t *testing.T) {
		t.Run("Create", func(t *testing.T) {
			methodInput := &structs.ACLAuthMethod{
				Name:        "test",
				Type:        "testing",
				Description: "test",
				Config: map[string]interface{}{
					"SessionID": testSessionID,
				},
			}

			req, _ := http.NewRequest("PUT", "/v1/acl/auth-method?token=root", jsonBody(methodInput))
			resp := httptest.NewRecorder()
			obj, err := a.srv.ACLAuthMethodCreate(resp, req)
			require.NoError(t, err)

			method, ok := obj.(*structs.ACLAuthMethod)
			require.True(t, ok)

			require.Equal(t, methodInput.Name, method.Name)
			require.Equal(t, methodInput.Type, method.Type)
			require.Equal(t, methodInput.Description, method.Description)
			require.Equal(t, methodInput.Config, method.Config)
			require.True(t, method.CreateIndex > 0)
			require.Equal(t, method.CreateIndex, method.ModifyIndex)

			methodMap[method.Name] = method
		})

		t.Run("Create other", func(t *testing.T) {
			methodInput := &structs.ACLAuthMethod{
				Name:        "other",
				Type:        "testing",
				Description: "test",
				Config: map[string]interface{}{
					"SessionID": testSessionID,
				},
				TokenLocality: "global",
				MaxTokenTTL:   500_000_000_000,
			}

			req, _ := http.NewRequest("PUT", "/v1/acl/auth-method?token=root", jsonBody(methodInput))
			resp := httptest.NewRecorder()
			obj, err := a.srv.ACLAuthMethodCreate(resp, req)
			require.NoError(t, err)

			method, ok := obj.(*structs.ACLAuthMethod)
			require.True(t, ok)

			require.Equal(t, methodInput.Name, method.Name)
			require.Equal(t, methodInput.Type, method.Type)
			require.Equal(t, methodInput.Description, method.Description)
			require.Equal(t, methodInput.Config, method.Config)
			require.True(t, method.CreateIndex > 0)
			require.Equal(t, method.CreateIndex, method.ModifyIndex)

			methodMap[method.Name] = method
		})

		t.Run("Create in remote datacenter", func(t *testing.T) {
			methodInput := &structs.ACLAuthMethod{
				Name:        "other",
				Type:        "testing",
				Description: "test",
				Config: map[string]interface{}{
					"SessionID": testSessionID,
				},
				TokenLocality: "global",
				MaxTokenTTL:   500_000_000_000,
			}

			req, _ := http.NewRequest("PUT", "/v1/acl/auth-method?token=root&dc=remote", jsonBody(methodInput))
			resp := httptest.NewRecorder()
			_, err := a.srv.ACLAuthMethodCRUD(resp, req)
			require.Error(t, err)
			require.True(t, isHTTPBadRequest(err))
		})

		t.Run("Update Name URL Mismatch", func(t *testing.T) {
			methodInput := &structs.ACLAuthMethod{
				Name:        "test",
				Type:        "testing",
				Description: "test",
				Config: map[string]interface{}{
					"SessionID": testSessionID,
				},
			}

			req, _ := http.NewRequest("PUT", "/v1/acl/auth-method/not-test?token=root", jsonBody(methodInput))
			resp := httptest.NewRecorder()
			_, err := a.srv.ACLAuthMethodCRUD(resp, req)
			require.Error(t, err)
			require.True(t, isHTTPBadRequest(err))
		})

		t.Run("Update", func(t *testing.T) {
			methodInput := &structs.ACLAuthMethod{
				Name:        "test",
				Type:        "testing",
				Description: "updated description",
				Config: map[string]interface{}{
					"SessionID": testSessionID,
				},
			}

			req, _ := http.NewRequest("PUT", "/v1/acl/auth-method/test?token=root", jsonBody(methodInput))
			resp := httptest.NewRecorder()
			obj, err := a.srv.ACLAuthMethodCRUD(resp, req)
			require.NoError(t, err)

			method, ok := obj.(*structs.ACLAuthMethod)
			require.True(t, ok)

			require.Equal(t, methodInput.Name, method.Name)
			require.Equal(t, methodInput.Type, method.Type)
			require.Equal(t, methodInput.Description, method.Description)
			require.Equal(t, methodInput.Config, method.Config)
			require.True(t, method.CreateIndex > 0)
			require.True(t, method.CreateIndex < method.ModifyIndex)

			methodMap[method.Name] = method
		})

		t.Run("Invalid payload", func(t *testing.T) {
			body := bytes.NewBuffer(nil)
			body.Write([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

			req, _ := http.NewRequest("PUT", "/v1/acl/auth-method?token=root", body)
			resp := httptest.NewRecorder()
			_, err := a.srv.ACLAuthMethodCreate(resp, req)
			require.Error(t, err)
			require.True(t, isHTTPBadRequest(err))
		})

		t.Run("List", func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/v1/acl/auth-methods?token=root", nil)
			resp := httptest.NewRecorder()
			raw, err := a.srv.ACLAuthMethodList(resp, req)
			require.NoError(t, err)

			methods, ok := raw.(structs.ACLAuthMethodListStubs)
			require.True(t, ok)

			// 2 we just created
			require.Len(t, methods, 2)

			for methodName, expected := range methodMap {
				found := false
				for _, actual := range methods {
					if actual.Name == methodName {
						require.Equal(t, expected.Name, actual.Name)
						require.Equal(t, expected.Type, actual.Type)
						require.Equal(t, expected.Description, actual.Description)
						require.Equal(t, expected.MaxTokenTTL, actual.MaxTokenTTL)
						require.Equal(t, expected.TokenLocality, actual.TokenLocality)
						require.Equal(t, expected.CreateIndex, actual.CreateIndex)
						require.Equal(t, expected.ModifyIndex, actual.ModifyIndex)
						found = true
						break
					}
				}

				require.True(t, found)
			}
		})

		t.Run("Delete", func(t *testing.T) {
			req, _ := http.NewRequest("DELETE", "/v1/acl/auth-method/other?token=root", nil)
			resp := httptest.NewRecorder()
			_, err := a.srv.ACLAuthMethodCRUD(resp, req)
			require.NoError(t, err)
			delete(methodMap, "other")
		})

		t.Run("Read", func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/v1/acl/auth-method/test?token=root", nil)
			resp := httptest.NewRecorder()
			raw, err := a.srv.ACLAuthMethodCRUD(resp, req)
			require.NoError(t, err)
			method, ok := raw.(*structs.ACLAuthMethod)
			require.True(t, ok)
			require.Equal(t, methodMap["test"], method)
		})
	})

	t.Run("BindingRule", func(t *testing.T) {
		t.Run("Create", func(t *testing.T) {
			ruleInput := &structs.ACLBindingRule{
				Description: "test",
				AuthMethod:  "test",
				Selector:    "serviceaccount.namespace==default",
				BindType:    structs.BindingRuleBindTypeService,
				BindName:    "web",
			}

			req, _ := http.NewRequest("PUT", "/v1/acl/binding-rule?token=root", jsonBody(ruleInput))
			resp := httptest.NewRecorder()
			obj, err := a.srv.ACLBindingRuleCreate(resp, req)
			require.NoError(t, err)

			rule, ok := obj.(*structs.ACLBindingRule)
			require.True(t, ok)

			// 36 = length of the string form of uuids
			require.Len(t, rule.ID, 36)
			require.Equal(t, ruleInput.Description, rule.Description)
			require.Equal(t, ruleInput.AuthMethod, rule.AuthMethod)
			require.Equal(t, ruleInput.Selector, rule.Selector)
			require.Equal(t, ruleInput.BindType, rule.BindType)
			require.Equal(t, ruleInput.BindName, rule.BindName)
			require.True(t, rule.CreateIndex > 0)
			require.Equal(t, rule.CreateIndex, rule.ModifyIndex)

			idMap["rule-test"] = rule.ID
			ruleMap[rule.ID] = rule
		})

		t.Run("Create other", func(t *testing.T) {
			ruleInput := &structs.ACLBindingRule{
				Description: "other",
				AuthMethod:  "test",
				Selector:    "serviceaccount.namespace==default",
				BindType:    structs.BindingRuleBindTypeRole,
				BindName:    "fancy-role",
			}

			req, _ := http.NewRequest("PUT", "/v1/acl/binding-rule?token=root", jsonBody(ruleInput))
			resp := httptest.NewRecorder()
			obj, err := a.srv.ACLBindingRuleCreate(resp, req)
			require.NoError(t, err)

			rule, ok := obj.(*structs.ACLBindingRule)
			require.True(t, ok)

			// 36 = length of the string form of uuids
			require.Len(t, rule.ID, 36)
			require.Equal(t, ruleInput.Description, rule.Description)
			require.Equal(t, ruleInput.AuthMethod, rule.AuthMethod)
			require.Equal(t, ruleInput.Selector, rule.Selector)
			require.Equal(t, ruleInput.BindType, rule.BindType)
			require.Equal(t, ruleInput.BindName, rule.BindName)
			require.True(t, rule.CreateIndex > 0)
			require.Equal(t, rule.CreateIndex, rule.ModifyIndex)

			idMap["rule-other"] = rule.ID
			ruleMap[rule.ID] = rule
		})

		t.Run("Create in remote datacenter", func(t *testing.T) {
			ruleInput := &structs.ACLBindingRule{
				Description: "other",
				AuthMethod:  "test",
				Selector:    "serviceaccount.namespace==default",
				BindType:    structs.BindingRuleBindTypeRole,
				BindName:    "fancy-role",
			}

			req, _ := http.NewRequest("PUT", "/v1/acl/binding-rule?token=root&dc=remote", jsonBody(ruleInput))
			resp := httptest.NewRecorder()
			_, err := a.srv.ACLBindingRuleCRUD(resp, req)
			require.EqualError(t, err, "No path to datacenter")
		})

		t.Run("BindingRule CRUD Missing ID in URL", func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/v1/acl/binding-rule/?token=root", nil)
			resp := httptest.NewRecorder()
			_, err := a.srv.ACLBindingRuleCRUD(resp, req)
			require.Error(t, err)
			require.True(t, isHTTPBadRequest(err))
		})

		t.Run("Update", func(t *testing.T) {
			ruleInput := &structs.ACLBindingRule{
				Description: "updated",
				AuthMethod:  "test",
				Selector:    "serviceaccount.namespace==default",
				BindType:    structs.BindingRuleBindTypeService,
				BindName:    "${serviceaccount.name}",
			}

			req, _ := http.NewRequest("PUT", "/v1/acl/binding-rule/"+idMap["rule-test"]+"?token=root", jsonBody(ruleInput))
			resp := httptest.NewRecorder()
			obj, err := a.srv.ACLBindingRuleCRUD(resp, req)
			require.NoError(t, err)

			rule, ok := obj.(*structs.ACLBindingRule)
			require.True(t, ok)

			// 36 = length of the string form of uuids
			require.Len(t, rule.ID, 36)
			require.Equal(t, ruleInput.Description, rule.Description)
			require.Equal(t, ruleInput.AuthMethod, rule.AuthMethod)
			require.Equal(t, ruleInput.Selector, rule.Selector)
			require.Equal(t, ruleInput.BindType, rule.BindType)
			require.Equal(t, ruleInput.BindName, rule.BindName)
			require.True(t, rule.CreateIndex > 0)
			require.True(t, rule.CreateIndex < rule.ModifyIndex)

			idMap["rule-test"] = rule.ID
			ruleMap[rule.ID] = rule
		})

		t.Run("ID Supplied", func(t *testing.T) {
			ruleInput := &structs.ACLBindingRule{
				ID:          "12123d01-37f1-47e6-b55b-32328652bd38",
				Description: "with-id",
				AuthMethod:  "test",
				Selector:    "serviceaccount.namespace==default",
				BindType:    structs.BindingRuleBindTypeService,
				BindName:    "vault",
			}

			req, _ := http.NewRequest("PUT", "/v1/acl/binding-rule?token=root", jsonBody(ruleInput))
			resp := httptest.NewRecorder()
			_, err := a.srv.ACLBindingRuleCreate(resp, req)
			require.Error(t, err)
			require.True(t, isHTTPBadRequest(err))
		})

		t.Run("Invalid payload", func(t *testing.T) {
			body := bytes.NewBuffer(nil)
			body.Write([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

			req, _ := http.NewRequest("PUT", "/v1/acl/binding-rule?token=root", body)
			resp := httptest.NewRecorder()
			_, err := a.srv.ACLBindingRuleCreate(resp, req)
			require.Error(t, err)
			require.True(t, isHTTPBadRequest(err))
		})

		t.Run("List", func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/v1/acl/binding-rules?token=root", nil)
			resp := httptest.NewRecorder()
			raw, err := a.srv.ACLBindingRuleList(resp, req)
			require.NoError(t, err)
			rules, ok := raw.(structs.ACLBindingRules)
			require.True(t, ok)

			// 2 we just created
			require.Len(t, rules, 2)

			for ruleID, expected := range ruleMap {
				found := false
				for _, actual := range rules {
					if actual.ID == ruleID {
						require.Equal(t, expected.Description, actual.Description)
						require.Equal(t, expected.AuthMethod, actual.AuthMethod)
						require.Equal(t, expected.Selector, actual.Selector)
						require.Equal(t, expected.BindType, actual.BindType)
						require.Equal(t, expected.BindName, actual.BindName)
						require.Equal(t, expected.CreateIndex, actual.CreateIndex)
						require.Equal(t, expected.ModifyIndex, actual.ModifyIndex)
						found = true
						break
					}
				}

				require.True(t, found)
			}
		})

		t.Run("Delete", func(t *testing.T) {
			req, _ := http.NewRequest("DELETE", "/v1/acl/binding-rule/"+idMap["rule-other"]+"?token=root", nil)
			resp := httptest.NewRecorder()
			_, err := a.srv.ACLBindingRuleCRUD(resp, req)
			require.NoError(t, err)
			delete(ruleMap, idMap["rule-other"])
			delete(idMap, "rule-other")
		})

		t.Run("Read", func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/v1/acl/binding-rule/"+idMap["rule-test"]+"?token=root", nil)
			resp := httptest.NewRecorder()
			raw, err := a.srv.ACLBindingRuleCRUD(resp, req)
			require.NoError(t, err)
			rule, ok := raw.(*structs.ACLBindingRule)
			require.True(t, ok)
			require.Equal(t, ruleMap[idMap["rule-test"]], rule)
		})
	})

	testauth.InstallSessionToken(testSessionID, "token1", "default", "demo1", "abc123")
	testauth.InstallSessionToken(testSessionID, "token2", "default", "demo2", "def456")

	t.Run("Login", func(t *testing.T) {
		t.Run("Create Token 1", func(t *testing.T) {
			loginInput := &structs.ACLLoginParams{
				AuthMethod:  "test",
				BearerToken: "token1",
				Meta:        map[string]string{"foo": "bar"},
			}

			req, _ := http.NewRequest("POST", "/v1/acl/login?token=root", jsonBody(loginInput))
			resp := httptest.NewRecorder()
			obj, err := a.srv.ACLLogin(resp, req)
			require.NoError(t, err)

			token, ok := obj.(*structs.ACLToken)
			require.True(t, ok)

			// 36 = length of the string form of uuids
			require.Len(t, token.AccessorID, 36)
			require.Len(t, token.SecretID, 36)
			require.Equal(t, `token created via login: {"foo":"bar"}`, token.Description)
			require.True(t, token.Local)
			require.Len(t, token.Policies, 0)
			require.Len(t, token.Roles, 0)
			require.Len(t, token.ServiceIdentities, 1)
			require.Equal(t, "demo1", token.ServiceIdentities[0].ServiceName)
			require.Len(t, token.ServiceIdentities[0].Datacenters, 0)
			require.True(t, token.CreateIndex > 0)
			require.Equal(t, token.CreateIndex, token.ModifyIndex)
			require.NotNil(t, token.Hash)
			require.NotEqual(t, token.Hash, []byte{})

			idMap["token-test-1"] = token.AccessorID
			tokenMap[token.AccessorID] = token
		})
		t.Run("Create Token 2", func(t *testing.T) {
			loginInput := &structs.ACLLoginParams{
				AuthMethod:  "test",
				BearerToken: "token2",
				Meta:        map[string]string{"blah": "woot"},
			}

			req, _ := http.NewRequest("POST", "/v1/acl/login?token=root", jsonBody(loginInput))
			resp := httptest.NewRecorder()
			obj, err := a.srv.ACLLogin(resp, req)
			require.NoError(t, err)

			token, ok := obj.(*structs.ACLToken)
			require.True(t, ok)

			// 36 = length of the string form of uuids
			require.Len(t, token.AccessorID, 36)
			require.Len(t, token.SecretID, 36)
			require.Equal(t, `token created via login: {"blah":"woot"}`, token.Description)
			require.True(t, token.Local)
			require.Len(t, token.Policies, 0)
			require.Len(t, token.Roles, 0)
			require.Len(t, token.ServiceIdentities, 1)
			require.Equal(t, "demo2", token.ServiceIdentities[0].ServiceName)
			require.Len(t, token.ServiceIdentities[0].Datacenters, 0)
			require.True(t, token.CreateIndex > 0)
			require.Equal(t, token.CreateIndex, token.ModifyIndex)
			require.NotNil(t, token.Hash)
			require.NotEqual(t, token.Hash, []byte{})

			idMap["token-test-2"] = token.AccessorID
			tokenMap[token.AccessorID] = token
		})

		t.Run("List Tokens by (incorrect) Method", func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/v1/acl/tokens?token=root&authmethod=other", nil)
			resp := httptest.NewRecorder()
			raw, err := a.srv.ACLTokenList(resp, req)
			require.NoError(t, err)
			tokens, ok := raw.(structs.ACLTokenListStubs)
			require.True(t, ok)
			require.Len(t, tokens, 0)
		})

		t.Run("List Tokens by (correct) Method", func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/v1/acl/tokens?token=root&authmethod=test", nil)
			resp := httptest.NewRecorder()
			raw, err := a.srv.ACLTokenList(resp, req)
			require.NoError(t, err)
			tokens, ok := raw.(structs.ACLTokenListStubs)
			require.True(t, ok)
			require.Len(t, tokens, 2)

			for tokenID, expected := range tokenMap {
				found := false
				for _, actual := range tokens {
					if actual.AccessorID == tokenID {
						require.Equal(t, expected.Description, actual.Description)
						require.Equal(t, expected.Policies, actual.Policies)
						require.Equal(t, expected.Roles, actual.Roles)
						require.Equal(t, expected.ServiceIdentities, actual.ServiceIdentities)
						require.Equal(t, expected.Local, actual.Local)
						require.Equal(t, expected.CreateTime, actual.CreateTime)
						require.Equal(t, expected.Hash, actual.Hash)
						require.Equal(t, expected.CreateIndex, actual.CreateIndex)
						require.Equal(t, expected.ModifyIndex, actual.ModifyIndex)
						found = true
						break
					}
				}
				require.True(t, found)
			}
		})

		t.Run("Logout", func(t *testing.T) {
			tok := tokenMap[idMap["token-test-1"]]
			req, _ := http.NewRequest("POST", "/v1/acl/logout?token="+tok.SecretID, nil)
			resp := httptest.NewRecorder()
			_, err := a.srv.ACLLogout(resp, req)
			require.NoError(t, err)
		})

		t.Run("Token is gone after Logout", func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/v1/acl/token/"+idMap["token-test-1"]+"?token=root", nil)
			resp := httptest.NewRecorder()
			_, err := a.srv.ACLTokenCRUD(resp, req)
			require.Error(t, err)
			require.True(t, acl.IsErrNotFound(err), err.Error())
		})
	})
}

func TestACLEndpoint_LoginLogout_jwt(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, TestACLConfigWithParams(nil))
	defer a.Shutdown()

	testrpc.WaitForLeader(t, a.RPC, "dc1")

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
			method, err := upsertTestCustomizedAuthMethod(a.RPC, TestDefaultInitialManagementToken, "dc1", func(method *structs.ACLAuthMethod) {
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
				loginInput := &structs.ACLLoginParams{
					AuthMethod:  method.Name,
					BearerToken: "invalid",
				}

				req, _ := http.NewRequest("POST", "/v1/acl/login", jsonBody(loginInput))
				resp := httptest.NewRecorder()
				_, err := a.srv.ACLLogin(resp, req)
				require.Error(t, err)
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
				loginInput := &structs.ACLLoginParams{
					AuthMethod:  method.Name,
					BearerToken: jwtData,
				}

				req, _ := http.NewRequest("POST", "/v1/acl/login", jsonBody(loginInput))
				resp := httptest.NewRecorder()
				_, err := a.srv.ACLLogin(resp, req)

				testutil.RequireErrorContains(t, err, "Permission denied")
			})

			_, err = upsertTestCustomizedBindingRule(a.RPC, TestDefaultInitialManagementToken, "dc1", func(rule *structs.ACLBindingRule) {
				rule.AuthMethod = method.Name
				rule.BindType = structs.BindingRuleBindTypeService
				rule.BindName = "test--${value.name}--${value.primary_org}"
				rule.Selector = "value.name == jeff2 and value.primary_org == engineering and foo in list.groups"
			})
			require.NoError(t, err)

			t.Run("valid bearer token 1 service binding", func(t *testing.T) {
				loginInput := &structs.ACLLoginParams{
					AuthMethod:  method.Name,
					BearerToken: jwtData,
				}

				req, _ := http.NewRequest("POST", "/v1/acl/login", jsonBody(loginInput))
				resp := httptest.NewRecorder()
				obj, err := a.srv.ACLLogin(resp, req)
				require.NoError(t, err)

				token, ok := obj.(*structs.ACLToken)
				require.True(t, ok)

				require.Equal(t, method.Name, token.AuthMethod)
				require.Equal(t, `token created via login`, token.Description)
				require.True(t, token.Local)
				require.Len(t, token.Roles, 0)
				require.Len(t, token.ServiceIdentities, 1)
				svcid := token.ServiceIdentities[0]
				require.Len(t, svcid.Datacenters, 0)
				require.Equal(t, "test--jeff2--engineering", svcid.ServiceName)

				// and delete it
				req, _ = http.NewRequest("GET", "/v1/acl/logout", nil)
				req.Header.Add("X-Consul-Token", token.SecretID)
				resp = httptest.NewRecorder()
				_, err = a.srv.ACLLogout(resp, req)
				require.NoError(t, err)

				// verify the token was deleted
				req, _ = http.NewRequest("GET", "/v1/acl/token/"+token.AccessorID, nil)
				req.Header.Add("X-Consul-Token", TestDefaultInitialManagementToken)
				resp = httptest.NewRecorder()

				// make the request
				_, err = a.srv.ACLTokenCRUD(resp, req)
				require.Error(t, err)
				require.Equal(t, acl.ErrNotFound, err)
			})
		})
	}
}

func TestACL_Authorize(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a1 := NewTestAgent(t, TestACLConfigWithParams(nil))
	defer a1.Shutdown()

	testrpc.WaitForTestAgent(t, a1.RPC, "dc1", testrpc.WithToken(TestDefaultInitialManagementToken))

	policyReq := structs.ACLPolicySetRequest{
		Policy: structs.ACLPolicy{
			Name:  "test",
			Rules: `acl = "read" operator = "write" service_prefix "" { policy = "read"} node_prefix "" { policy= "write" } key_prefix "/foo" { policy = "write" } `,
		},
		Datacenter:   "dc1",
		WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
	}
	var policy structs.ACLPolicy
	require.NoError(t, a1.RPC("ACL.PolicySet", &policyReq, &policy))

	tokenReq := structs.ACLTokenSetRequest{
		ACLToken: structs.ACLToken{
			Policies: []structs.ACLTokenPolicyLink{
				{
					ID: policy.ID,
				},
			},
		},
		Datacenter:   "dc1",
		WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
	}

	var token structs.ACLToken
	require.NoError(t, a1.RPC("ACL.TokenSet", &tokenReq, &token))

	// secondary also needs to setup a replication token to pull tokens and policies
	secondaryParams := DefaultTestACLConfigParams()
	secondaryParams.ReplicationToken = secondaryParams.InitialManagementToken
	secondaryParams.EnableTokenReplication = true

	a2 := NewTestAgent(t, `datacenter = "dc2" `+TestACLConfigWithParams(secondaryParams))
	defer a2.Shutdown()

	addr := fmt.Sprintf("127.0.0.1:%d", a1.Config.SerfPortWAN)
	_, err := a2.JoinWAN([]string{addr})
	require.NoError(t, err)

	testrpc.WaitForTestAgent(t, a2.RPC, "dc2", testrpc.WithToken(TestDefaultInitialManagementToken))
	// this actually ensures a few things. First the dcs got connect okay, secondly that the policy we
	// are about ready to use in our local token creation exists in the secondary DC
	testrpc.WaitForACLReplication(t, a2.RPC, "dc2", structs.ACLReplicateTokens, policy.CreateIndex, 1, 0)

	localTokenReq := structs.ACLTokenSetRequest{
		ACLToken: structs.ACLToken{
			Policies: []structs.ACLTokenPolicyLink{
				{
					ID: policy.ID,
				},
			},
			Local: true,
		},
		Datacenter:   "dc2",
		WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
	}

	var localToken structs.ACLToken
	require.NoError(t, a2.RPC("ACL.TokenSet", &localTokenReq, &localToken))

	t.Run("initial-management-token", func(t *testing.T) {
		request := []structs.ACLAuthorizationRequest{
			{
				Resource: "acl",
				Access:   "read",
			},
			{
				Resource: "acl",
				Access:   "write",
			},
			{
				Resource: "agent",
				Segment:  "foo",
				Access:   "read",
			},
			{
				Resource: "agent",
				Segment:  "foo",
				Access:   "write",
			},
			{
				Resource: "event",
				Segment:  "foo",
				Access:   "read",
			},
			{
				Resource: "event",
				Segment:  "foo",
				Access:   "write",
			},
			{
				Resource: "intention",
				Segment:  "foo",
				Access:   "read",
			},
			{
				Resource: "intention",
				Segment:  "foo",
				Access:   "write",
			},
			{
				Resource: "key",
				Segment:  "foo",
				Access:   "read",
			},
			{
				Resource: "key",
				Segment:  "foo",
				Access:   "list",
			},
			{
				Resource: "key",
				Segment:  "foo",
				Access:   "write",
			},
			{
				Resource: "keyring",
				Access:   "read",
			},
			{
				Resource: "keyring",
				Access:   "write",
			},
			{
				Resource: "node",
				Segment:  "foo",
				Access:   "read",
			},
			{
				Resource: "node",
				Segment:  "foo",
				Access:   "write",
			},
			{
				Resource: "operator",
				Access:   "read",
			},
			{
				Resource: "operator",
				Access:   "write",
			},
			{
				Resource: "mesh",
				Access:   "read",
			},
			{
				Resource: "mesh",
				Access:   "write",
			},
			{
				Resource: "peering",
				Access:   "read",
			},
			{
				Resource: "peering",
				Access:   "write",
			},
			{
				Resource: "query",
				Segment:  "foo",
				Access:   "read",
			},
			{
				Resource: "query",
				Segment:  "foo",
				Access:   "write",
			},
			{
				Resource: "service",
				Segment:  "foo",
				Access:   "read",
			},
			{
				Resource: "service",
				Segment:  "foo",
				Access:   "write",
			},
			{
				Resource: "session",
				Segment:  "foo",
				Access:   "read",
			},
			{
				Resource: "session",
				Segment:  "foo",
				Access:   "write",
			},
		}

		for _, dc := range []string{"dc1", "dc2"} {
			t.Run(dc, func(t *testing.T) {
				req, _ := http.NewRequest("POST", "/v1/internal/acl/authorize?dc="+dc, jsonBody(request))
				req.Header.Add("X-Consul-Token", TestDefaultInitialManagementToken)
				recorder := httptest.NewRecorder()
				raw, err := a1.srv.ACLAuthorize(recorder, req)
				require.NoError(t, err)
				responses, ok := raw.([]structs.ACLAuthorizationResponse)
				require.True(t, ok)
				require.Len(t, responses, len(request))

				for idx, req := range request {
					resp := responses[idx]

					require.Equal(t, req, resp.ACLAuthorizationRequest)
					require.True(t, resp.Allow, "should have allowed all access for initial management token")
				}
			})
		}

	})

	customAuthorizationRequests := []structs.ACLAuthorizationRequest{
		{
			Resource: "acl",
			Access:   "read",
		},
		{
			Resource: "acl",
			Access:   "write",
		},
		{
			Resource: "agent",
			Segment:  "foo",
			Access:   "read",
		},
		{
			Resource: "agent",
			Segment:  "foo",
			Access:   "write",
		},
		{
			Resource: "event",
			Segment:  "foo",
			Access:   "read",
		},
		{
			Resource: "event",
			Segment:  "foo",
			Access:   "write",
		},
		{
			Resource: "intention",
			Segment:  "foo",
			Access:   "read",
		},
		{
			Resource: "intention",
			Segment:  "foo",
			Access:   "write",
		},
		{
			Resource: "key",
			Segment:  "foo",
			Access:   "read",
		},
		{
			Resource: "key",
			Segment:  "foo",
			Access:   "list",
		},
		{
			Resource: "key",
			Segment:  "foo",
			Access:   "write",
		},
		{
			Resource: "keyring",
			Access:   "read",
		},
		{
			Resource: "keyring",
			Access:   "write",
		},
		{
			Resource: "node",
			Segment:  "foo",
			Access:   "read",
		},
		{
			Resource: "node",
			Segment:  "foo",
			Access:   "write",
		},
		{
			Resource: "operator",
			Access:   "read",
		},
		{
			Resource: "operator",
			Access:   "write",
		},
		{
			Resource: "mesh",
			Access:   "read",
		},
		{
			Resource: "mesh",
			Access:   "write",
		},
		{
			Resource: "peering",
			Access:   "read",
		},
		{
			Resource: "peering",
			Access:   "write",
		},
		{
			Resource: "query",
			Segment:  "foo",
			Access:   "read",
		},
		{
			Resource: "query",
			Segment:  "foo",
			Access:   "write",
		},
		{
			Resource: "service",
			Segment:  "foo",
			Access:   "read",
		},
		{
			Resource: "service",
			Segment:  "foo",
			Access:   "write",
		},
		{
			Resource: "session",
			Segment:  "foo",
			Access:   "read",
		},
		{
			Resource: "session",
			Segment:  "foo",
			Access:   "write",
		},
	}

	expectedCustomAuthorizationResponses := []bool{
		true,  // acl:read
		false, // acl:write
		false, // agent:read
		false, // agent:write
		false, // event:read
		false, // event:write
		true,  // intentions:read
		false, // intention:write
		false, // key:read
		false, // key:list
		false, // key:write
		false, // keyring:read
		false, // keyring:write
		true,  // node:read
		true,  // node:write
		true,  // operator:read
		true,  // operator:write
		true,  // mesh:read
		true,  // mesh:write
		true,  // peering:read
		true,  // peering:write
		false, // query:read
		false, // query:write
		true,  // service:read
		false, // service:write
		false, // session:read
		false, // session:write
	}

	t.Run("custom-token", func(t *testing.T) {
		for _, dc := range []string{"dc1", "dc2"} {
			t.Run(dc, func(t *testing.T) {
				req, _ := http.NewRequest("POST", "/v1/internal/acl/authorize", jsonBody(customAuthorizationRequests))
				req.Header.Add("X-Consul-Token", token.SecretID)
				recorder := httptest.NewRecorder()
				raw, err := a1.srv.ACLAuthorize(recorder, req)
				require.NoError(t, err)
				responses, ok := raw.([]structs.ACLAuthorizationResponse)
				require.True(t, ok)
				require.Len(t, responses, len(customAuthorizationRequests))
				require.Len(t, responses, len(expectedCustomAuthorizationResponses))

				for idx, req := range customAuthorizationRequests {
					resp := responses[idx]

					require.Equal(t, req, resp.ACLAuthorizationRequest)
					require.Equal(t, expectedCustomAuthorizationResponses[idx], resp.Allow, "request %d - %+v returned unexpected response", idx, resp.ACLAuthorizationRequest)
				}
			})
		}
	})

	t.Run("too-many-requests", func(t *testing.T) {
		var request []structs.ACLAuthorizationRequest

		for i := 0; i < 100; i++ {
			request = append(request, structs.ACLAuthorizationRequest{Resource: "acl", Access: "read"})
		}

		req, _ := http.NewRequest("POST", "/v1/internal/acl/authorize", jsonBody(request))
		req.Header.Add("X-Consul-Token", token.SecretID)
		recorder := httptest.NewRecorder()
		raw, err := a1.srv.ACLAuthorize(recorder, req)
		require.Error(t, err)
		require.Contains(t, err.Error(), "Refusing to process more than 64 authorizations at once")
		require.Nil(t, raw)
	})

	t.Run("decode-failure", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/v1/internal/acl/authorize", jsonBody(structs.ACLAuthorizationRequest{Resource: "acl", Access: "read"}))
		req.Header.Add("X-Consul-Token", token.SecretID)
		recorder := httptest.NewRecorder()
		raw, err := a1.srv.ACLAuthorize(recorder, req)
		require.Error(t, err)
		require.Contains(t, err.Error(), "Failed to decode request body")
		require.Nil(t, raw)
	})

	t.Run("acl-not-found", func(t *testing.T) {
		request := []structs.ACLAuthorizationRequest{
			{
				Resource: "acl",
				Access:   "read",
			},
		}

		req, _ := http.NewRequest("POST", "/v1/internal/acl/authorize", jsonBody(request))
		req.Header.Add("X-Consul-Token", "d908c0be-22e1-433e-84db-8718e1a019de")
		recorder := httptest.NewRecorder()
		raw, err := a1.srv.ACLAuthorize(recorder, req)
		require.Error(t, err)
		require.Equal(t, acl.ErrNotFound, err)
		require.Nil(t, raw)
	})

	t.Run("local-token-in-secondary-dc", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/v1/internal/acl/authorize?dc=dc2", jsonBody(customAuthorizationRequests))
		req.Header.Add("X-Consul-Token", localToken.SecretID)
		recorder := httptest.NewRecorder()
		raw, err := a1.srv.ACLAuthorize(recorder, req)
		require.NoError(t, err)
		responses, ok := raw.([]structs.ACLAuthorizationResponse)
		require.True(t, ok)
		require.Len(t, responses, len(customAuthorizationRequests))
		require.Len(t, responses, len(expectedCustomAuthorizationResponses))

		for idx, req := range customAuthorizationRequests {
			resp := responses[idx]

			require.Equal(t, req, resp.ACLAuthorizationRequest)
			require.Equal(t, expectedCustomAuthorizationResponses[idx], resp.Allow, "request %d - %+v returned unexpected response", idx, resp.ACLAuthorizationRequest)
		}
	})

	t.Run("local-token-wrong-dc", func(t *testing.T) {
		request := []structs.ACLAuthorizationRequest{
			{
				Resource: "acl",
				Access:   "read",
			},
		}

		req, _ := http.NewRequest("POST", "/v1/internal/acl/authorize", jsonBody(request))
		req.Header.Add("X-Consul-Token", localToken.SecretID)
		recorder := httptest.NewRecorder()
		raw, err := a1.srv.ACLAuthorize(recorder, req)
		require.Error(t, err)
		require.Equal(t, acl.ErrNotFound, err)
		require.Nil(t, raw)
	})
}

type rpcFn func(string, interface{}, interface{}) error

func upsertTestCustomizedAuthMethod(
	rpc rpcFn, initialManagementToken string, datacenter string,
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

	err = rpc("ACL.AuthMethodSet", &req, &out)
	if err != nil {
		return nil, err
	}

	return &out, nil
}

func upsertTestCustomizedBindingRule(rpc rpcFn, initialManagementToken string, datacenter string, modify func(rule *structs.ACLBindingRule)) (*structs.ACLBindingRule, error) {
	req := structs.ACLBindingRuleSetRequest{
		Datacenter:   datacenter,
		BindingRule:  structs.ACLBindingRule{},
		WriteRequest: structs.WriteRequest{Token: initialManagementToken},
	}

	if modify != nil {
		modify(&req.BindingRule)
	}

	var out structs.ACLBindingRule

	err := rpc("ACL.BindingRuleSet", &req, &out)
	if err != nil {
		return nil, err
	}

	return &out, nil
}

func TestHTTPHandlers_ACLReplicationStatus(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, TestACLConfig())
	defer a.Shutdown()

	req, _ := http.NewRequest("GET", "/v1/acl/replication", nil)
	resp := httptest.NewRecorder()
	testrpc.WaitForLeader(t, a.RPC, "dc1")
	obj, err := a.srv.ACLReplicationStatus(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	_, ok := obj.(structs.ACLReplicationStatus)
	if !ok {
		t.Fatalf("should work")
	}
}
