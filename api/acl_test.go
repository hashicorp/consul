package api

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAPI_ACLReplication(t *testing.T) {
	t.Parallel()
	c, s := makeACLClient(t)
	defer s.Stop()

	acl := c.ACL()

	repl, qm, err := acl.Replication(nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if repl == nil {
		t.Fatalf("bad: %v", repl)
	}

	if repl.Running {
		t.Fatal("bad: repl should not be running")
	}

	if repl.Enabled {
		t.Fatal("bad: repl should not be enabled")
	}

	if qm.RequestTime == 0 {
		t.Fatalf("bad: %v", qm)
	}
}

func TestAPI_ACLPolicy_CreateReadDelete(t *testing.T) {
	t.Parallel()
	c, s := makeACLClient(t)
	defer s.Stop()

	acl := c.ACL()

	created, wm, err := acl.PolicyCreate(&ACLPolicy{
		Name:        "test-policy",
		Description: "test-policy description",
		Rules:       `node_prefix "" { policy = "read" }`,
		Datacenters: []string{"dc1"},
	}, nil)

	require.NoError(t, err)
	require.NotNil(t, created)
	require.NotEqual(t, "", created.ID)
	require.NotEqual(t, 0, wm.RequestTime)

	read, qm, err := acl.PolicyRead(created.ID, nil)
	require.NoError(t, err)
	require.NotEqual(t, 0, qm.LastIndex)
	require.True(t, qm.KnownLeader)

	require.Equal(t, created, read)

	wm, err = acl.PolicyDelete(created.ID, nil)
	require.NoError(t, err)
	require.NotEqual(t, 0, wm.RequestTime)

	read, _, err = acl.PolicyRead(created.ID, nil)
	require.Nil(t, read)
	require.Error(t, err)
}

func TestAPI_ACLPolicy_CreateReadByNameDelete(t *testing.T) {
	t.Parallel()
	c, s := makeACLClient(t)
	defer s.Stop()

	acl := c.ACL()

	created, wm, err := acl.PolicyCreate(&ACLPolicy{
		Name:        "test-policy",
		Description: "test-policy description",
		Rules:       `node_prefix "" { policy = "read" }`,
		Datacenters: []string{"dc1"},
	}, nil)

	require.NoError(t, err)
	require.NotNil(t, created)
	require.NotEqual(t, "", created.ID)
	require.NotEqual(t, 0, wm.RequestTime)

	read, qm, err := acl.PolicyReadByName(created.Name, nil)
	require.NoError(t, err)
	require.NotEqual(t, 0, qm.LastIndex)
	require.True(t, qm.KnownLeader)

	require.Equal(t, created, read)

	wm, err = acl.PolicyDelete(created.ID, nil)
	require.NoError(t, err)
	require.NotEqual(t, 0, wm.RequestTime)

	read, _, err = acl.PolicyRead(created.ID, nil)
	require.Nil(t, read)
	require.Error(t, err)
}

func TestAPI_ACLPolicy_CreateUpdate(t *testing.T) {
	t.Parallel()
	c, s := makeACLClient(t)
	defer s.Stop()

	acl := c.ACL()

	created, _, err := acl.PolicyCreate(&ACLPolicy{
		Name:        "test-policy",
		Description: "test-policy description",
		Rules:       `node_prefix "" { policy = "read" }`,
		Datacenters: []string{"dc1"},
	}, nil)

	require.NoError(t, err)
	require.NotNil(t, created)
	require.NotEqual(t, "", created.ID)

	read, _, err := acl.PolicyRead(created.ID, nil)
	require.NoError(t, err)
	require.Equal(t, created, read)

	read.Rules += ` service_prefix "" { policy = "read" }`
	read.Datacenters = nil

	updated, wm, err := acl.PolicyUpdate(read, nil)
	require.NoError(t, err)
	require.Equal(t, created.ID, updated.ID)
	require.Equal(t, created.Description, updated.Description)
	require.Equal(t, read.Rules, updated.Rules)
	require.Equal(t, created.CreateIndex, updated.CreateIndex)
	require.NotEqual(t, created.ModifyIndex, updated.ModifyIndex)
	require.Nil(t, updated.Datacenters)
	require.NotEqual(t, 0, wm.RequestTime)

	updated_read, _, err := acl.PolicyRead(created.ID, nil)
	require.NoError(t, err)
	require.Equal(t, updated, updated_read)
}

func TestAPI_ACLPolicy_List(t *testing.T) {
	t.Parallel()
	c, s := makeACLClient(t)
	defer s.Stop()

	acl := c.ACL()

	created1, _, err := acl.PolicyCreate(&ACLPolicy{
		Name:        "policy1",
		Description: "policy1 description",
		Rules:       `node_prefix "" { policy = "read" }`,
		Datacenters: []string{"dc1"},
	}, nil)

	require.NoError(t, err)
	require.NotNil(t, created1)
	require.NotEqual(t, "", created1.ID)

	created2, _, err := acl.PolicyCreate(&ACLPolicy{
		Name:        "policy2",
		Description: "policy2 description",
		Rules:       `service "app" { policy = "write" }`,
		Datacenters: []string{"dc1", "dc2"},
	}, nil)

	require.NoError(t, err)
	require.NotNil(t, created2)
	require.NotEqual(t, "", created2.ID)

	created3, _, err := acl.PolicyCreate(&ACLPolicy{
		Name:        "policy3",
		Description: "policy3 description",
		Rules:       `acl = "read"`,
	}, nil)

	require.NoError(t, err)
	require.NotNil(t, created3)
	require.NotEqual(t, "", created3.ID)

	policies, qm, err := acl.PolicyList(nil)
	require.NoError(t, err)
	require.Len(t, policies, 4)
	require.NotEqual(t, 0, qm.LastIndex)
	require.True(t, qm.KnownLeader)

	policyMap := make(map[string]*ACLPolicyListEntry)
	for _, policy := range policies {
		policyMap[policy.ID] = policy
	}

	policy1, ok := policyMap[created1.ID]
	require.True(t, ok)
	require.NotNil(t, policy1)
	require.Equal(t, created1.Name, policy1.Name)
	require.Equal(t, created1.Description, policy1.Description)
	require.Equal(t, created1.CreateIndex, policy1.CreateIndex)
	require.Equal(t, created1.ModifyIndex, policy1.ModifyIndex)
	require.Equal(t, created1.Hash, policy1.Hash)
	require.ElementsMatch(t, created1.Datacenters, policy1.Datacenters)

	policy2, ok := policyMap[created2.ID]
	require.True(t, ok)
	require.NotNil(t, policy2)
	require.Equal(t, created2.Name, policy2.Name)
	require.Equal(t, created2.Description, policy2.Description)
	require.Equal(t, created2.CreateIndex, policy2.CreateIndex)
	require.Equal(t, created2.ModifyIndex, policy2.ModifyIndex)
	require.Equal(t, created2.Hash, policy2.Hash)
	require.ElementsMatch(t, created2.Datacenters, policy2.Datacenters)

	policy3, ok := policyMap[created3.ID]
	require.True(t, ok)
	require.NotNil(t, policy3)
	require.Equal(t, created3.Name, policy3.Name)
	require.Equal(t, created3.Description, policy3.Description)
	require.Equal(t, created3.CreateIndex, policy3.CreateIndex)
	require.Equal(t, created3.ModifyIndex, policy3.ModifyIndex)
	require.Equal(t, created3.Hash, policy3.Hash)
	require.ElementsMatch(t, created3.Datacenters, policy3.Datacenters)

	// make sure the 4th policy is the global management
	policy4, ok := policyMap["00000000-0000-0000-0000-000000000001"]
	require.True(t, ok)
	require.NotNil(t, policy4)
}

func prepTokenPolicies(t *testing.T, acl *ACL) (policies []*ACLPolicy) {
	return prepTokenPoliciesInPartition(t, acl, "")
}

func prepTokenPoliciesInPartition(t *testing.T, acl *ACL, partition string) (policies []*ACLPolicy) {
	datacenters := []string{"dc1", "dc2"}
	if partition != "" && partition != "default" {
		datacenters = []string{"dc1"}
	}
	var wqPart *WriteOptions
	if partition != "" {
		wqPart = &WriteOptions{Partition: partition}
	}
	policy, _, err := acl.PolicyCreate(&ACLPolicy{
		Name:        "one",
		Description: "one description",
		Rules:       `acl = "read"`,
		Datacenters: datacenters,
	}, wqPart)

	require.NoError(t, err)
	require.NotNil(t, policy)
	policies = append(policies, policy)

	policy, _, err = acl.PolicyCreate(&ACLPolicy{
		Name:        "two",
		Description: "two description",
		Rules:       `node_prefix "" { policy = "read" }`,
		Datacenters: datacenters,
	}, wqPart)

	require.NoError(t, err)
	require.NotNil(t, policy)
	policies = append(policies, policy)

	policy, _, err = acl.PolicyCreate(&ACLPolicy{
		Name:        "three",
		Description: "three description",
		Rules:       `service_prefix "" { policy = "read" }`,
	}, wqPart)

	require.NoError(t, err)
	require.NotNil(t, policy)
	policies = append(policies, policy)

	policy, _, err = acl.PolicyCreate(&ACLPolicy{
		Name:        "four",
		Description: "four description",
		Rules:       `agent "foo" { policy = "write" }`,
	}, wqPart)

	require.NoError(t, err)
	require.NotNil(t, policy)
	policies = append(policies, policy)
	return
}

func TestAPI_ACLBootstrap(t *testing.T) {
	t.Parallel()
	c, s := makeNonBootstrappedACLClient(t, "allow")

	s.WaitForLeader(t)
	// not bootstrapped, default allow
	mems, err := c.Agent().Members(false)
	require.NoError(t, err)
	require.True(t, len(mems) == 1)

	s.Stop()
	c, s = makeNonBootstrappedACLClient(t, "deny")
	acl := c.ACL()
	s.WaitForLeader(t)
	//not bootstrapped, default deny
	_, _, err = acl.TokenList(nil)
	require.EqualError(t, err, "Unexpected response code: 403 (Permission denied: anonymous token lacks permission 'acl:read'. The anonymous token is used implicitly when a request does not specify a token.)")
	c.config.Token = "root"
	_, _, err = acl.TokenList(nil)
	require.EqualError(t, err, "Unexpected response code: 403 (ACL system must be bootstrapped before making any requests that require authorization: ACL not found)")
	// bootstrap
	mgmtTok, _, err := acl.Bootstrap()
	require.NoError(t, err)
	// bootstrapped
	acl.c.config.Token = mgmtTok.SecretID
	toks, _, err := acl.TokenList(nil)
	require.NoError(t, err)
	// management and anonymous should be only tokens
	require.Len(t, toks, 2)
	s.Stop()
}

func TestAPI_ACLToken_CreateReadDelete(t *testing.T) {
	t.Parallel()
	c, s := makeACLClient(t)
	defer s.Stop()

	acl := c.ACL()

	policies := prepTokenPolicies(t, acl)

	created, wm, err := acl.TokenCreate(&ACLToken{
		Description: "token created",
		Policies: []*ACLTokenPolicyLink{
			{
				ID: policies[0].ID,
			},
			{
				ID: policies[1].ID,
			},
			{
				Name: policies[2].Name,
			},
			{
				Name: policies[3].Name,
			},
		},
	}, nil)

	require.NoError(t, err)
	require.NotNil(t, created)
	require.NotEqual(t, "", created.AccessorID)
	require.NotEqual(t, "", created.SecretID)
	require.NotEqual(t, 0, wm.RequestTime)

	read, qm, err := acl.TokenRead(created.AccessorID, nil)
	require.NoError(t, err)
	require.Equal(t, created, read)
	require.NotEqual(t, 0, qm.LastIndex)
	require.True(t, qm.KnownLeader)

	acl.c.config.Token = created.SecretID
	self, _, err := acl.TokenReadSelf(nil)
	require.NoError(t, err)
	require.Equal(t, created, self)
	acl.c.config.Token = "root"

	_, err = acl.TokenDelete(created.AccessorID, nil)
	require.NoError(t, err)

	read, _, err = acl.TokenRead(created.AccessorID, nil)
	require.Nil(t, read)
	require.Error(t, err)
}

func TestAPI_ACLToken_CreateUpdate(t *testing.T) {
	t.Parallel()
	c, s := makeACLClient(t)
	defer s.Stop()

	acl := c.ACL()

	policies := prepTokenPolicies(t, acl)

	created, _, err := acl.TokenCreate(&ACLToken{
		Description: "token created",
		Policies: []*ACLTokenPolicyLink{
			{
				ID: policies[0].ID,
			},
			{
				Name: policies[2].Name,
			},
		},
	}, nil)

	require.NoError(t, err)
	require.NotNil(t, created)
	require.NotEqual(t, "", created.AccessorID)
	require.NotEqual(t, "", created.SecretID)

	read, _, err := acl.TokenRead(created.AccessorID, nil)
	require.NoError(t, err)
	require.Equal(t, created, read)

	read.Policies = append(read.Policies, &ACLTokenPolicyLink{ID: policies[1].ID})
	read.Policies = append(read.Policies, &ACLTokenPolicyLink{Name: policies[2].Name})

	expectedPolicies := []*ACLTokenPolicyLink{
		{
			ID:   policies[0].ID,
			Name: policies[0].Name,
		},
		{
			ID:   policies[1].ID,
			Name: policies[1].Name,
		},
		{
			ID:   policies[2].ID,
			Name: policies[2].Name,
		},
	}

	updated, wm, err := acl.TokenUpdate(read, nil)
	require.NoError(t, err)
	require.Equal(t, created.AccessorID, updated.AccessorID)
	require.Equal(t, created.SecretID, updated.SecretID)
	require.Equal(t, created.Description, updated.Description)
	require.Equal(t, created.CreateIndex, updated.CreateIndex)
	require.NotEqual(t, created.ModifyIndex, updated.ModifyIndex)
	require.ElementsMatch(t, expectedPolicies, updated.Policies)
	require.NotEqual(t, 0, wm.RequestTime)

	updated_read, _, err := acl.TokenRead(created.AccessorID, nil)
	require.NoError(t, err)
	require.Equal(t, updated, updated_read)
}

func TestAPI_ACLToken_List(t *testing.T) {
	t.Parallel()
	c, s := makeACLClient(t)
	defer s.Stop()

	acl := c.ACL()
	s.WaitForSerfCheck(t)

	policies := prepTokenPolicies(t, acl)

	created1, _, err := acl.TokenCreate(&ACLToken{
		Description: "token created1",
		Policies: []*ACLTokenPolicyLink{
			{
				ID: policies[0].ID,
			},
		},
	}, nil)

	require.NoError(t, err)
	require.NotNil(t, created1)
	require.NotEqual(t, "", created1.AccessorID)
	require.NotEqual(t, "", created1.SecretID)

	created2, _, err := acl.TokenCreate(&ACLToken{
		Description: "token created2",
		Policies: []*ACLTokenPolicyLink{
			{
				ID: policies[1].ID,
			},
		},
	}, nil)

	require.NoError(t, err)
	require.NotNil(t, created2)
	require.NotEqual(t, "", created2.AccessorID)
	require.NotEqual(t, "", created2.SecretID)

	created3, _, err := acl.TokenCreate(&ACLToken{
		Description: "token created3",
		Policies: []*ACLTokenPolicyLink{
			{
				ID: policies[2].ID,
			},
		},
	}, nil)

	require.NoError(t, err)
	require.NotNil(t, created3)
	require.NotEqual(t, "", created3.AccessorID)
	require.NotEqual(t, "", created3.SecretID)

	tokens, qm, err := acl.TokenList(nil)
	require.NoError(t, err)
	// 3 + anon + initial management
	require.Len(t, tokens, 5)
	require.NotEqual(t, 0, qm.LastIndex)
	require.True(t, qm.KnownLeader)

	tokenMap := make(map[string]*ACLTokenListEntry)
	for _, token := range tokens {
		tokenMap[token.AccessorID] = token
	}

	token1, ok := tokenMap[created1.AccessorID]
	require.True(t, ok)
	require.NotNil(t, token1)
	require.Equal(t, created1.SecretID, token1.SecretID)
	require.Equal(t, created1.Description, token1.Description)
	require.Equal(t, created1.CreateIndex, token1.CreateIndex)
	require.Equal(t, created1.ModifyIndex, token1.ModifyIndex)
	require.Equal(t, created1.Hash, token1.Hash)
	require.ElementsMatch(t, created1.Policies, token1.Policies)

	token2, ok := tokenMap[created2.AccessorID]
	require.True(t, ok)
	require.NotNil(t, token2)
	require.Equal(t, created2.SecretID, token2.SecretID)
	require.Equal(t, created2.Description, token2.Description)
	require.Equal(t, created2.CreateIndex, token2.CreateIndex)
	require.Equal(t, created2.ModifyIndex, token2.ModifyIndex)
	require.Equal(t, created2.Hash, token2.Hash)
	require.ElementsMatch(t, created2.Policies, token2.Policies)

	token3, ok := tokenMap[created3.AccessorID]
	require.True(t, ok)
	require.NotNil(t, token3)
	require.Equal(t, created3.SecretID, token3.SecretID)
	require.Equal(t, created3.Description, token3.Description)
	require.Equal(t, created3.CreateIndex, token3.CreateIndex)
	require.Equal(t, created3.ModifyIndex, token3.ModifyIndex)
	require.Equal(t, created3.Hash, token3.Hash)
	require.ElementsMatch(t, created3.Policies, token3.Policies)

	// make sure the there is an anon token
	token4, ok := tokenMap["00000000-0000-0000-0000-000000000002"]
	require.True(t, ok)
	require.NotNil(t, token4)

	// ensure the 5th token is the initial management token
	root, _, err := acl.TokenReadSelf(nil)
	require.NoError(t, err)
	require.NotNil(t, root)
	token5, ok := tokenMap[root.AccessorID]
	require.True(t, ok)
	require.NotNil(t, token5)
}

func TestAPI_ACLToken_Clone(t *testing.T) {
	t.Parallel()
	c, s := makeACLClient(t)
	defer s.Stop()

	acl := c.ACL()

	initialManagement, _, err := acl.TokenReadSelf(nil)
	require.NoError(t, err)
	require.NotNil(t, initialManagement)

	cloned, _, err := acl.TokenClone(initialManagement.AccessorID, "cloned", nil)
	require.NoError(t, err)
	require.NotNil(t, cloned)
	require.NotEqual(t, initialManagement.AccessorID, cloned.AccessorID)
	require.NotEqual(t, initialManagement.SecretID, cloned.SecretID)
	require.Equal(t, "cloned", cloned.Description)
	require.ElementsMatch(t, initialManagement.Policies, cloned.Policies)

	read, _, err := acl.TokenRead(cloned.AccessorID, nil)
	require.NoError(t, err)
	require.NotNil(t, read)
	require.Equal(t, cloned, read)
}

func TestAPI_AuthMethod_List(t *testing.T) {
	t.Parallel()
	c, s := makeACLClient(t)
	defer s.Stop()

	acl := c.ACL()
	s.WaitForSerfCheck(t)

	method1 := ACLAuthMethod{
		Name:          "test_1",
		Type:          "kubernetes",
		Description:   "test 1",
		MaxTokenTTL:   260 * time.Second,
		TokenLocality: "global",
		Config:        AuthMethodCreateKubernetesConfigHelper(),
	}

	created1, wm, err := acl.AuthMethodCreate(&method1, nil)

	require.NoError(t, err)
	require.NotNil(t, created1)
	require.NotEqual(t, "", created1.Name)
	require.NotEqual(t, 0, wm.RequestTime)

	method2 := ACLAuthMethod{
		Name:          "test_2",
		Type:          "kubernetes",
		Description:   "test 2",
		MaxTokenTTL:   0,
		TokenLocality: "local",
		Config:        AuthMethodCreateKubernetesConfigHelper(),
	}

	_, _, err = acl.AuthMethodCreate(&method2, nil)
	require.NoError(t, err)

	entries, _, err := acl.AuthMethodList(nil)
	require.NoError(t, err)
	require.NotNil(t, entries)
	require.Equal(t, 2, len(entries))

	{
		entry := entries[0]
		require.Equal(t, "test_1", entry.Name)
		require.Equal(t, 260*time.Second, entry.MaxTokenTTL)
		require.Equal(t, "global", entry.TokenLocality)
	}
	{
		entry := entries[1]
		require.Equal(t, "test_2", entry.Name)
		require.Equal(t, time.Duration(0), entry.MaxTokenTTL)
		require.Equal(t, "local", entry.TokenLocality)
	}
}

func AuthMethodCreateKubernetesConfigHelper() (result map[string]interface{}) {
	var pemData = `
-----BEGIN CERTIFICATE-----
MIIE1DCCArwCCQC2kx7TchbxAzANBgkqhkiG9w0BAQsFADAsMQswCQYDVQQGEwJV
UzELMAkGA1UECAwCV0ExEDAOBgNVBAcMB1NlYXR0bGUwHhcNMjEwMTI3MDIzNDA1
WhcNMjIwMTI3MDIzNDA1WjAsMQswCQYDVQQGEwJVUzELMAkGA1UECAwCV0ExEDAO
BgNVBAcMB1NlYXR0bGUwggIiMA0GCSqGSIb3DQEBAQUAA4ICDwAwggIKAoICAQCt
j3zRFLg2A2DcZFwoc1HvIsGzqcfvxjee/OQjKyIuXbdpbJGIahB2piNYtd49zU/5
ofRAuqIQOco3V9LfL52I7NchNBvPQOrXjbpcM3qF2qQvunVlnnaPCIf8S5hsFMaq
w2/+jnLjaUdXGJ9bold5E/bms87uRahvhUpY7MhkSDNsAen+YThpwucc9JFRmrz3
EXGtTzcpyEn9b0s6ut9mum2UVqghAQyLeW8cNx1zeg6Bi5USjOKF6CQgF7o4kZ9X
D0Nk5vB9eePs/q5N9LHkDFKVCmzAYgzcQeGZFEzNcgK7N5y+aB2xXKpH3tydpwRd
uS+g05Jvk8M8P34wteUb8tq3jZuY7UYzlINMSrPuZdFhcGjmxPjC5hl1SZy4vF1s
GAD9RsleTZ8yeC6Cfo4mba214C9CqYkC2NBw2HO53pzO/tYI844QPhjmVBJ7bb35
S052HD7m+AzbfY6w9CDH4D4mzIM4u1yRB6OlXdXTH58BhgxHdEnugLYr13QlVWRW
4nZgMFKiTY7cBscpPcVRsne/VR9VwSatp3adj+G8+WUtwQLJC2OcCFYvmHfdSOs0
B15LH/tGeJcfKViKC9ifPq5abVZByr66jTQMAdBWet03OBnmLqJs9TI4wci0MkK/
HlHYdy734rReD81LY9fCRCRFV4ZtMx2rfj7cqgKLlwIDAQABMA0GCSqGSIb3DQEB
CwUAA4ICAQB6ji6wA9ROFx8ZhLPlEnDiielSUN8LR2K8cmAjxxffJo3GxRH/zZYl
CM+DzU5VVzW6RGWuTNzcFNsxlaRx20sj5RyXLH90wFYLO2Rrs1XKWmqpfdN0Iiue
W7rYdNPV7YPjIVQVoijEt8kwx24jE9mU5ILXe4+WKPWavG+dHA1r8lQdg7wmE/8R
E/nSVtusuX0JRVdL96iy2HB37DYj+rJEE0C7fKAk51o0C4F6fOzUsWCaP/23pZNI
rA6hCq2CJeT4ObVukCIrnylrckZs8ElcZ7PvJ9bCNvma+dAxbL0uEkv0q0feLeVh
OTttNIVTUjYjr3KE6rtE1Rr35R/6HCK+zZDOkKf+TVEQsFuI4DRVEuntzjo9bgZf
fAL6G+UXpzW440BJzmzADnSthawMZFdqVrrBzpzb+B2d9VLDEoyCCFzaJyj/Gyff
kqxRFTHZJRKC/3iIRXOX64bIr1YmXHFHCBkcq7eyh1oeaTrGZ43HimaveWwcsPv/
SxTJANJHqf4BiFtVjN7LZXi3HUIRAsceEbd0TfW5be9SQ0tbDyyGYt/bXtBLGTIh
9kerr9eWDHlpHMTyP01+Ua3EacbfgrmvD9sa3s6gC4SnwlvLdubmyLwoorCs77eF
15bSOU7NsVZfwLw+M+DyNWPxI1BR/XOP+YoyTgIEChIC9eYnmlWU2Q==
-----END CERTIFICATE-----`

	result = map[string]interface{}{
		"Host":              "https://192.0.2.42:8443",
		"CACert":            pemData,
		"ServiceAccountJWT": `eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiYWRtaW4iOnRydWUsImp0aSI6ImQxYTZiYzE5LWZiODItNDI5ZC05NmUxLTg1YTFjYjEyNGQ3MCIsImlhdCI6MTYxMTcxNTQ5NiwiZXhwIjoxNjExNzE5MDk2fQ.rrVS5h1Yw20eI41RsTl2YAqzKKikKNg3qMkDmspTPQs`,
	}
	return
}
