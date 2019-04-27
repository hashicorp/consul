package agent

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/authmethod/testauth"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/testrpc"
	"github.com/stretchr/testify/require"
)

// NOTE: The tests contained herein are designed to test the HTTP API
//       They are not intended to thoroughly test the backing RPC
//       functionality as that will be done with other tests.

func TestACL_Disabled_Response(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, t.Name(), "")
	defer a.Shutdown()

	type testCase struct {
		name string
		fn   func(resp http.ResponseWriter, req *http.Request) (interface{}, error)
	}

	tests := []testCase{
		{"ACLBootstrap", a.srv.ACLBootstrap},
		{"ACLReplicationStatus", a.srv.ACLReplicationStatus},
		{"AgentToken", a.srv.AgentToken}, // See TestAgent_Token
		{"ACLRulesTranslate", a.srv.ACLRulesTranslate},
		{"ACLRulesTranslateLegacyToken", a.srv.ACLRulesTranslateLegacyToken},
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
	}
	testrpc.WaitForLeader(t, a.RPC, "dc1")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("PUT", "/should/not/care", nil)
			resp := httptest.NewRecorder()
			obj, err := tt.fn(resp, req)
			require.NoError(t, err)
			require.Nil(t, obj)
			require.Equal(t, http.StatusUnauthorized, resp.Code)
			require.Contains(t, resp.Body.String(), "ACL support disabled")
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
	t.Parallel()
	a := NewTestAgent(t, t.Name(), TestACLConfig()+`
      acl_master_token = ""
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
			if got, want := resp.Code, tt.code; got != want {
				t.Fatalf("got %d want %d", got, want)
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
	t.Parallel()
	a := NewTestAgent(t, t.Name(), TestACLConfig())
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
			_, ok := err.(BadRequestError)
			require.True(t, ok)
		})

		t.Run("Policy CRUD Missing ID in URL", func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/v1/acl/policy/?token=root", nil)
			resp := httptest.NewRecorder()
			_, err := a.srv.ACLPolicyCRUD(resp, req)
			require.Error(t, err)
			_, ok := err.(BadRequestError)
			require.True(t, ok)
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
			_, ok := err.(BadRequestError)
			require.True(t, ok)
		})

		t.Run("Invalid payload", func(t *testing.T) {
			body := bytes.NewBuffer(nil)
			body.Write([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

			req, _ := http.NewRequest("PUT", "/v1/acl/policy?token=root", body)
			resp := httptest.NewRecorder()
			_, err := a.srv.ACLPolicyCreate(resp, req)
			require.Error(t, err)
			_, ok := err.(BadRequestError)
			require.True(t, ok)
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
	})

	t.Run("Role", func(t *testing.T) {
		t.Run("Create", func(t *testing.T) {
			roleInput := &structs.ACLRole{
				Name:        "test",
				Description: "test",
				Policies: []structs.ACLRolePolicyLink{
					structs.ACLRolePolicyLink{
						ID:   idMap["policy-test"],
						Name: policyMap[idMap["policy-test"]].Name,
					},
					structs.ACLRolePolicyLink{
						ID:   idMap["policy-read-all-nodes"],
						Name: policyMap[idMap["policy-read-all-nodes"]].Name,
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
					&structs.ACLServiceIdentity{
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
					&structs.ACLServiceIdentity{
						ServiceName: "db",
					},
				},
			}

			req, _ := http.NewRequest("PUT", "/v1/acl/role/"+idMap["role-test"]+"?token=root", jsonBody(roleInput))
			resp := httptest.NewRecorder()
			_, err := a.srv.ACLRoleCRUD(resp, req)
			require.Error(t, err)
			_, ok := err.(BadRequestError)
			require.True(t, ok)
		})

		t.Run("Role CRUD Missing ID in URL", func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/v1/acl/role/?token=root", nil)
			resp := httptest.NewRecorder()
			_, err := a.srv.ACLRoleCRUD(resp, req)
			require.Error(t, err)
			_, ok := err.(BadRequestError)
			require.True(t, ok)
		})

		t.Run("Update", func(t *testing.T) {
			roleInput := &structs.ACLRole{
				Name:        "test",
				Description: "test",
				ServiceIdentities: []*structs.ACLServiceIdentity{
					&structs.ACLServiceIdentity{
						ServiceName: "web-indexer",
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
					&structs.ACLServiceIdentity{
						ServiceName: "foobar",
					},
				},
			}

			req, _ := http.NewRequest("PUT", "/v1/acl/role?token=root", jsonBody(roleInput))
			resp := httptest.NewRecorder()
			_, err := a.srv.ACLRoleCreate(resp, req)
			require.Error(t, err)
			_, ok := err.(BadRequestError)
			require.True(t, ok)
		})

		t.Run("Invalid payload", func(t *testing.T) {
			body := bytes.NewBuffer(nil)
			body.Write([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

			req, _ := http.NewRequest("PUT", "/v1/acl/role?token=root", body)
			resp := httptest.NewRecorder()
			_, err := a.srv.ACLRoleCreate(resp, req)
			require.Error(t, err)
			_, ok := err.(BadRequestError)
			require.True(t, ok)
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
					structs.ACLTokenPolicyLink{
						ID:   idMap["policy-test"],
						Name: policyMap[idMap["policy-test"]].Name,
					},
					structs.ACLTokenPolicyLink{
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
			require.Len(t, token.AccessorID, 36)
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
		t.Run("Create Local", func(t *testing.T) {
			tokenInput := &structs.ACLToken{
				Description: "local",
				Policies: []structs.ACLTokenPolicyLink{
					structs.ACLTokenPolicyLink{
						ID:   idMap["policy-test"],
						Name: policyMap[idMap["policy-test"]].Name,
					},
					structs.ACLTokenPolicyLink{
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
					structs.ACLTokenPolicyLink{
						ID:   idMap["policy-read-all-nodes"],
						Name: policyMap[idMap["policy-read-all-nodes"]].Name,
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
			_, ok := err.(BadRequestError)
			require.True(t, ok)
		})
		t.Run("Update Accessor Mismatch", func(t *testing.T) {
			originalToken := tokenMap[idMap["token-cloned"]]

			// Accessor and Secret will be filled in
			tokenInput := &structs.ACLToken{
				AccessorID:  "e8aeb69a-0ace-42b9-b95f-d1d9eafe1561",
				Description: "Better description for this cloned token",
				Policies: []structs.ACLTokenPolicyLink{
					structs.ACLTokenPolicyLink{
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
			_, ok := err.(BadRequestError)
			require.True(t, ok)
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

			// 3 tokens created but 1 was deleted + master token + anon token
			require.Len(t, tokens, 4)

			// this loop doesn't verify anything about the master token
			for tokenID, expected := range tokenMap {
				found := false
				for _, actual := range tokens {
					if actual.AccessorID == tokenID {
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
			require.Equal(t, "Master Token", token.Description)
			require.Len(t, token.Policies, 1)
			require.Equal(t, structs.ACLPolicyGlobalManagementID, token.Policies[0].ID)
		})
		t.Run("Create with Accessor", func(t *testing.T) {
			tokenInput := &structs.ACLToken{
				AccessorID:  "56e8e6a3-708b-4a2f-8ab3-b973cce39108",
				Description: "test",
				Policies: []structs.ACLTokenPolicyLink{
					structs.ACLTokenPolicyLink{
						ID:   idMap["policy-test"],
						Name: policyMap[idMap["policy-test"]].Name,
					},
					structs.ACLTokenPolicyLink{
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
					structs.ACLTokenPolicyLink{
						ID:   idMap["policy-test"],
						Name: policyMap[idMap["policy-test"]].Name,
					},
					structs.ACLTokenPolicyLink{
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
					structs.ACLTokenPolicyLink{
						ID:   idMap["policy-test"],
						Name: policyMap[idMap["policy-test"]].Name,
					},
					structs.ACLTokenPolicyLink{
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
					structs.ACLTokenPolicyLink{
						ID:   idMap["policy-test"],
						Name: policyMap[idMap["policy-test"]].Name,
					},
					structs.ACLTokenPolicyLink{
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
					structs.ACLTokenPolicyLink{
						ID:   idMap["policy-test"],
						Name: policyMap[idMap["policy-test"]].Name,
					},
					structs.ACLTokenPolicyLink{
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
					structs.ACLTokenPolicyLink{
						ID:   idMap["policy-test"],
						Name: policyMap[idMap["policy-test"]].Name,
					},
					structs.ACLTokenPolicyLink{
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
					structs.ACLTokenPolicyLink{
						ID:   idMap["policy-test"],
						Name: policyMap[idMap["policy-test"]].Name,
					},
					structs.ACLTokenPolicyLink{
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
					structs.ACLTokenPolicyLink{
						ID:   idMap["policy-test"],
						Name: policyMap[idMap["policy-test"]].Name,
					},
					structs.ACLTokenPolicyLink{
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
					structs.ACLTokenPolicyLink{
						ID:   idMap["policy-test"],
						Name: policyMap[idMap["policy-test"]].Name,
					},
					structs.ACLTokenPolicyLink{
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
	})
}

func TestACL_LoginProcedure_HTTP(t *testing.T) {
	// This tests AuthMethods, BindingRules, Login, and Logout.
	t.Parallel()
	a := NewTestAgent(t, t.Name(), TestACLConfig())
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
			_, ok := err.(BadRequestError)
			require.True(t, ok)
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
			_, ok := err.(BadRequestError)
			require.True(t, ok)
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

		t.Run("BindingRule CRUD Missing ID in URL", func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/v1/acl/binding-rule/?token=root", nil)
			resp := httptest.NewRecorder()
			_, err := a.srv.ACLBindingRuleCRUD(resp, req)
			require.Error(t, err)
			_, ok := err.(BadRequestError)
			require.True(t, ok)
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
			_, ok := err.(BadRequestError)
			require.True(t, ok)
		})

		t.Run("Invalid payload", func(t *testing.T) {
			body := bytes.NewBuffer(nil)
			body.Write([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

			req, _ := http.NewRequest("PUT", "/v1/acl/binding-rule?token=root", body)
			resp := httptest.NewRecorder()
			_, err := a.srv.ACLBindingRuleCreate(resp, req)
			require.Error(t, err)
			_, ok := err.(BadRequestError)
			require.True(t, ok)
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
