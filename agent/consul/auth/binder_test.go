// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/go-uuid"

	"github.com/hashicorp/consul/agent/consul/authmethod"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
)

func TestBindings_None(t *testing.T) {
	var b *Bindings
	require.True(t, b.None())

	b = &Bindings{}
	require.True(t, b.None())

	b = &Bindings{Roles: []structs.ACLTokenRoleLink{{ID: generateID(t)}}}
	require.False(t, b.None())

	b = &Bindings{Policies: []structs.ACLTokenPolicyLink{{ID: generateID(t)}}}
	require.False(t, b.None())

	b = &Bindings{ServiceIdentities: []*structs.ACLServiceIdentity{{ServiceName: "web"}}}
	require.False(t, b.None())

	b = &Bindings{NodeIdentities: []*structs.ACLNodeIdentity{{NodeName: "node-123"}}}
	require.False(t, b.None())

	b = &Bindings{TemplatedPolicies: []*structs.ACLTemplatedPolicy{{TemplateName: api.ACLTemplatedPolicyDNSName}}}
	require.False(t, b.None())
}

func TestBinder_Policy_Success(t *testing.T) {
	store := testStateStore(t)
	binder := &Binder{store: store}

	authMethod := &structs.ACLAuthMethod{
		Name: "test-auth-method",
		Type: "testing",
	}
	require.NoError(t, store.ACLAuthMethodSet(0, authMethod))

	targetPolicy := &structs.ACLPolicy{
		ID:   generateID(t),
		Name: "foo-policy",
	}
	require.NoError(t, store.ACLPolicySet(0, targetPolicy))

	otherPolicy := &structs.ACLPolicy{
		ID:   generateID(t),
		Name: "not-my-policy",
	}
	require.NoError(t, store.ACLPolicySet(0, otherPolicy))

	bindingRules := structs.ACLBindingRules{
		{
			ID:         generateID(t),
			Selector:   "role==engineer",
			BindType:   structs.BindingRuleBindTypePolicy,
			BindName:   "${editor}-policy",
			AuthMethod: authMethod.Name,
		},
		{
			ID:         generateID(t),
			Selector:   "role==engineer",
			BindType:   structs.BindingRuleBindTypePolicy,
			BindName:   "this-policy-does-not-exist",
			AuthMethod: authMethod.Name,
		},
		{
			ID:         generateID(t),
			Selector:   "language==js",
			BindType:   structs.BindingRuleBindTypePolicy,
			BindName:   otherPolicy.Name,
			AuthMethod: authMethod.Name,
		},
	}
	require.NoError(t, store.ACLBindingRuleBatchSet(0, bindingRules))

	result, err := binder.Bind(&structs.ACLAuthMethod{}, &authmethod.Identity{
		SelectableFields: map[string]string{
			"role":     "engineer",
			"language": "go",
		},
		ProjectedVars: map[string]string{
			"editor": "foo",
		},
	})
	require.NoError(t, err)
	require.Equal(t, []structs.ACLTokenPolicyLink{
		{ID: targetPolicy.ID, Name: targetPolicy.Name},
	}, result.Policies)
}

func TestBinder_Roles_Success(t *testing.T) {
	store := testStateStore(t)
	binder := &Binder{store: store}

	authMethod := &structs.ACLAuthMethod{
		Name: "test-auth-method",
		Type: "testing",
	}
	require.NoError(t, store.ACLAuthMethodSet(0, authMethod))

	targetRole := &structs.ACLRole{
		ID:   generateID(t),
		Name: "vim-role",
	}
	require.NoError(t, store.ACLRoleSet(0, targetRole))

	otherRole := &structs.ACLRole{
		ID:   generateID(t),
		Name: "frontend-engineers",
	}
	require.NoError(t, store.ACLRoleSet(0, otherRole))

	bindingRules := structs.ACLBindingRules{
		{
			ID:         generateID(t),
			Selector:   "role==engineer",
			BindType:   structs.BindingRuleBindTypeRole,
			BindName:   "${editor}-role",
			AuthMethod: authMethod.Name,
		},
		{
			ID:         generateID(t),
			Selector:   "role==engineer",
			BindType:   structs.BindingRuleBindTypeRole,
			BindName:   "this-role-does-not-exist",
			AuthMethod: authMethod.Name,
		},
		{
			ID:         generateID(t),
			Selector:   "language==js",
			BindType:   structs.BindingRuleBindTypeRole,
			BindName:   otherRole.Name,
			AuthMethod: authMethod.Name,
		},
	}
	require.NoError(t, store.ACLBindingRuleBatchSet(0, bindingRules))

	result, err := binder.Bind(&structs.ACLAuthMethod{}, &authmethod.Identity{
		SelectableFields: map[string]string{
			"role":     "engineer",
			"language": "go",
		},
		ProjectedVars: map[string]string{
			"editor": "vim",
		},
	})
	require.NoError(t, err)
	require.Equal(t, []structs.ACLTokenRoleLink{
		{ID: targetRole.ID},
	}, result.Roles)
}

func TestBinder_Roles_NameValidation(t *testing.T) {
	store := testStateStore(t)
	binder := &Binder{store: store}

	authMethod := &structs.ACLAuthMethod{
		Name: "test-auth-method",
		Type: "testing",
	}
	require.NoError(t, store.ACLAuthMethodSet(0, authMethod))

	bindingRules := structs.ACLBindingRules{
		{
			ID:         generateID(t),
			Selector:   "",
			BindType:   structs.BindingRuleBindTypeRole,
			BindName:   "INVALID!",
			AuthMethod: authMethod.Name,
		},
	}
	require.NoError(t, store.ACLBindingRuleBatchSet(0, bindingRules))

	_, err := binder.Bind(&structs.ACLAuthMethod{}, &authmethod.Identity{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid bind name")
}

func TestBinder_Policy_NameValidation(t *testing.T) {
	store := testStateStore(t)
	binder := &Binder{store: store}

	authMethod := &structs.ACLAuthMethod{
		Name: "test-auth-method",
		Type: "testing",
	}
	require.NoError(t, store.ACLAuthMethodSet(0, authMethod))

	bindingRules := structs.ACLBindingRules{
		{
			ID:         generateID(t),
			Selector:   "",
			BindType:   structs.BindingRuleBindTypePolicy,
			BindName:   "INVALID!",
			AuthMethod: authMethod.Name,
		},
	}
	require.NoError(t, store.ACLBindingRuleBatchSet(0, bindingRules))

	_, err := binder.Bind(&structs.ACLAuthMethod{}, &authmethod.Identity{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid bind name")
}

func TestBinder_ServiceIdentities_Success(t *testing.T) {
	store := testStateStore(t)
	binder := &Binder{store: store}

	authMethod := &structs.ACLAuthMethod{
		Name: "test-auth-method",
		Type: "testing",
	}
	require.NoError(t, store.ACLAuthMethodSet(0, authMethod))

	bindingRules := structs.ACLBindingRules{
		{
			ID:         generateID(t),
			Selector:   "tier==web",
			BindType:   structs.BindingRuleBindTypeService,
			BindName:   "web-service-${name}",
			AuthMethod: authMethod.Name,
		},
		{
			ID:         generateID(t),
			Selector:   "tier==db",
			BindType:   structs.BindingRuleBindTypeService,
			BindName:   "database-${name}",
			AuthMethod: authMethod.Name,
		},
	}
	require.NoError(t, store.ACLBindingRuleBatchSet(0, bindingRules))

	result, err := binder.Bind(&structs.ACLAuthMethod{}, &authmethod.Identity{
		SelectableFields: map[string]string{
			"tier": "web",
		},
		ProjectedVars: map[string]string{
			"name": "billing",
		},
	})
	require.NoError(t, err)
	require.Equal(t, []*structs.ACLServiceIdentity{
		{ServiceName: "web-service-billing"},
	}, result.ServiceIdentities)
}

func TestBinder_ServiceIdentities_NameValidation(t *testing.T) {
	store := testStateStore(t)
	binder := &Binder{store: store}

	authMethod := &structs.ACLAuthMethod{
		Name: "test-auth-method",
		Type: "testing",
	}
	require.NoError(t, store.ACLAuthMethodSet(0, authMethod))

	bindingRules := structs.ACLBindingRules{
		{
			ID:         generateID(t),
			Selector:   "",
			BindType:   structs.BindingRuleBindTypeService,
			BindName:   "INVALID!",
			AuthMethod: authMethod.Name,
		},
	}
	require.NoError(t, store.ACLBindingRuleBatchSet(0, bindingRules))

	_, err := binder.Bind(&structs.ACLAuthMethod{}, &authmethod.Identity{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid bind name")
}

func TestBinder_NodeIdentities_Success(t *testing.T) {
	store := testStateStore(t)
	binder := &Binder{store: store, datacenter: "dc1"}

	authMethod := &structs.ACLAuthMethod{
		Name: "test-auth-method",
		Type: "testing",
	}
	require.NoError(t, store.ACLAuthMethodSet(0, authMethod))

	bindingRules := structs.ACLBindingRules{
		{
			ID:         generateID(t),
			Selector:   "provider==gcp",
			BindType:   structs.BindingRuleBindTypeNode,
			BindName:   "gcp-${os}",
			AuthMethod: authMethod.Name,
		},
		{
			ID:         generateID(t),
			Selector:   "provider==aws",
			BindType:   structs.BindingRuleBindTypeNode,
			BindName:   "aws-${os}",
			AuthMethod: authMethod.Name,
		},
	}
	require.NoError(t, store.ACLBindingRuleBatchSet(0, bindingRules))

	result, err := binder.Bind(&structs.ACLAuthMethod{}, &authmethod.Identity{
		SelectableFields: map[string]string{
			"provider": "gcp",
		},
		ProjectedVars: map[string]string{
			"os": "linux",
		},
	})
	require.NoError(t, err)
	require.Equal(t, []*structs.ACLNodeIdentity{
		{NodeName: "gcp-linux", Datacenter: "dc1"},
	}, result.NodeIdentities)
}

func TestBinder_NodeIdentities_NameValidation(t *testing.T) {
	store := testStateStore(t)
	binder := &Binder{store: store}

	authMethod := &structs.ACLAuthMethod{
		Name: "test-auth-method",
		Type: "testing",
	}
	require.NoError(t, store.ACLAuthMethodSet(0, authMethod))

	bindingRules := structs.ACLBindingRules{
		{
			ID:         generateID(t),
			Selector:   "",
			BindType:   structs.BindingRuleBindTypeNode,
			BindName:   "INVALID!",
			AuthMethod: authMethod.Name,
		},
	}
	require.NoError(t, store.ACLBindingRuleBatchSet(0, bindingRules))

	_, err := binder.Bind(&structs.ACLAuthMethod{}, &authmethod.Identity{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid bind name")
}

func Test_IsValidBindingRule(t *testing.T) {
	type testcase struct {
		name     string
		bindType string
		bindName string
		bindVars *structs.ACLTemplatedPolicyVariables
		fields   string
		err      bool
	}

	for _, test := range []testcase{
		{"no bind type",
			"", "", nil, "", true},
		{"bad bind type",
			"invalid", "blah", nil, "", true},
		// valid HIL, invalid name
		{"empty",
			"all", "", nil, "", true},
		{"just end",
			"all", "}", nil, "", true},
		{"var without start",
			"all", " item }", nil, "item", true},
		{"two vars missing second start",
			"all", "before-${ item }after--more }", nil, "item,more", true},
		// names for the two types are validated differently
		{"@ is disallowed",
			"all", "bad@name", nil, "", true},
		{"leading dash",
			"role", "-name", nil, "", false},
		{"leading dash",
			"policy", "-name", nil, "", false},
		{"leading dash",
			"service", "-name", nil, "", true},
		{"trailing dash",
			"role", "name-", nil, "", false},
		{"trailing dash",
			"policy", "name-", nil, "", false},
		{"trailing dash",
			"service", "name-", nil, "", true},
		{"inner dash",
			"all", "name-end", nil, "", false},
		{"upper case",
			"role", "NAME", nil, "", false},
		{"upper case",
			"policy", "NAME", nil, "", false},
		{"upper case",
			"service", "NAME", nil, "", true},
		// valid HIL, valid name
		{"no vars",
			"all", "nothing", nil, "", false},
		{"just var",
			"all", "${item}", nil, "item", false},
		{"var in middle",
			"all", "before-${item}after", nil, "item", false},
		{"two vars",
			"all", "before-${item}after-${more}", nil, "item,more", false},
		// bad
		{"no bind name",
			"all", "", nil, "", true},
		{"just start",
			"all", "${", nil, "", true},
		{"backwards",
			"all", "}${", nil, "", true},
		{"no varname",
			"all", "${}", nil, "", true},
		{"missing map key",
			"all", "${item}", nil, "", true},
		{"var without end",
			"all", "${ item ", nil, "item", true},
		{"two vars missing first end",
			"all", "before-${ item after-${ more }", nil, "item,more", true},

		// bind type: templated policy - bad input
		{"templated-policy missing bindvars", "templated-policy", "builtin/service", nil, "", true},
		{"templated-policy with unknown templated policy name",
			"templated-policy", "builtin/service", &structs.ACLTemplatedPolicyVariables{Name: "before-${item}after-${more}"}, "", true},
		{"templated-policy with correct bindvars and unknown vars",
			"templated-policy", "builtin/fake", &structs.ACLTemplatedPolicyVariables{Name: "test"}, "", true},
		{"templated-policy with correct bindvars but incorrect HIL",
			"templated-policy", "builtin/service", &structs.ACLTemplatedPolicyVariables{Name: "before-${ item }after--more }"}, "", true},

		// bind type: templated policy - good input
		{"templated-policy with appropriate bindvars",
			"templated-policy", "builtin/service", &structs.ACLTemplatedPolicyVariables{Name: "before-${item}after-${more}"}, "item,more", false},
	} {
		var cases []testcase
		if test.bindType == "all" {
			test1 := test
			test1.bindType = "role"
			test2 := test
			test2.bindType = "service"
			test3 := test
			test3.bindType = "policy"
			test4 := test
			test4.bindType = "node"
			cases = []testcase{test1, test2, test3, test4}
		} else {
			cases = []testcase{test}
		}

		for _, test := range cases {
			test := test
			t.Run(test.bindType+"--"+test.name, func(t *testing.T) {
				t.Parallel()
				err := IsValidBindingRule(
					test.bindType,
					test.bindName,
					test.bindVars,
					strings.Split(test.fields, ","),
				)
				require.Equal(t, test.err, err != nil)
			})
		}
	}
}

func generateID(t *testing.T) string {
	t.Helper()

	id, err := uuid.GenerateUUID()
	require.NoError(t, err)

	return id
}

func testStateStore(t *testing.T) *state.Store {
	t.Helper()

	gc, err := state.NewTombstoneGC(time.Second, time.Millisecond)
	require.NoError(t, err)

	return state.NewStateStore(gc)
}
