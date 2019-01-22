package agent

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/testrpc"
	"github.com/stretchr/testify/require"
)

// NOTE: The tests contained herein are designed to test the HTTP API
//       They are not intented to thoroughly test the backing RPC
//       functionality as that will be done with other tests.

func TestACL_Disabled_Response(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
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
	a := NewTestAgent(t.Name(), TestACLConfig()+`
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
	a := NewTestAgent(t.Name(), TestACLConfig())
	defer a.Shutdown()

	testrpc.WaitForLeader(t, a.RPC, "dc1")

	idMap := make(map[string]string)
	policyMap := make(map[string]*structs.ACLPolicy)
	tokenMap := make(map[string]*structs.ACLToken)

	// This is all done as a subtest for a couple reasons
	// 1. It uses only 1 test agent and these are
	//    somewhat expensive to bring up and tear down often
	// 2. Instead of having to bring up a new agent and prime
	//    the ACL system with some data before running the test
	//    we can intelligently order these tests so we can still
	//    test everything with less actual operations and do
	//    so in a manner that is less prone to being flaky
	// 3. While this test will be large it should
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

		t.Run("Update Name ID Mistmatch", func(t *testing.T) {
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
	})
}
