package auth

import (
	"fmt"

	"github.com/hashicorp/go-bexpr"
	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/authmethod"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib/template"
)

// Binder is responsible for collecting the ACL roles, service identities, node
// identities, and enterprise metadata to be assigned to a token generated as a
// result of "logging in" via an auth method.
//
// It does so by applying the auth method's configured binding rules and in the
// case of enterprise, namespace rules.
type Binder struct {
	store      BinderStateStore
	datacenter string
}

// NewBinder creates a Binder with the given state store and datacenter.
func NewBinder(store BinderStateStore, datacenter string) *Binder {
	return &Binder{store, datacenter}
}

// BinderStateStore is the subset of state store methods used by the binder.
type BinderStateStore interface {
	ACLBindingRuleList(ws memdb.WatchSet, methodName string, entMeta *acl.EnterpriseMeta) (uint64, structs.ACLBindingRules, error)
	ACLRoleGetByName(ws memdb.WatchSet, roleName string, entMeta *acl.EnterpriseMeta) (uint64, *structs.ACLRole, error)
}

// Bindings contains the ACL roles, service identities, node identities and
// enterprise meta to be assigned to the created token.
type Bindings struct {
	Roles             []structs.ACLTokenRoleLink
	ServiceIdentities []*structs.ACLServiceIdentity
	NodeIdentities    []*structs.ACLNodeIdentity
	EnterpriseMeta    acl.EnterpriseMeta
}

// None indicates that the resulting bindings would not give the created token
// access to any resources.
func (b *Bindings) None() bool {
	if b == nil {
		return true
	}

	return len(b.ServiceIdentities) == 0 &&
		len(b.NodeIdentities) == 0 &&
		len(b.Roles) == 0
}

// Bind collects the ACL roles, service identities, etc. to be assigned to the
// created token.
func (b *Binder) Bind(authMethod *structs.ACLAuthMethod, verifiedIdentity *authmethod.Identity) (*Bindings, error) {
	var (
		bindings Bindings
		err      error
	)
	if bindings.EnterpriseMeta, err = bindEnterpriseMeta(authMethod, verifiedIdentity); err != nil {
		return nil, err
	}

	// Load the auth method's binding rules.
	_, rules, err := b.store.ACLBindingRuleList(nil, authMethod.Name, &authMethod.EnterpriseMeta)
	if err != nil {
		return nil, err
	}

	// Find the rules with selectors that match the identity's fields.
	matchingRules := make(structs.ACLBindingRules, 0, len(rules))
	for _, rule := range rules {
		if doesSelectorMatch(rule.Selector, verifiedIdentity.SelectableFields) {
			matchingRules = append(matchingRules, rule)
		}
	}
	if len(matchingRules) == 0 {
		return &bindings, nil
	}

	// Compute role, service identity, or node identity names by interpolating
	// the identity's projected variables into the rule BindName templates.
	for _, rule := range matchingRules {
		bindName, valid, err := computeBindName(rule.BindType, rule.BindName, verifiedIdentity.ProjectedVars)
		switch {
		case err != nil:
			return nil, fmt.Errorf("cannot compute %q bind name for bind target: %w", rule.BindType, err)
		case !valid:
			return nil, fmt.Errorf("computed %q bind name for bind target is invalid: %q", rule.BindType, bindName)
		}

		switch rule.BindType {
		case structs.BindingRuleBindTypeService:
			bindings.ServiceIdentities = append(bindings.ServiceIdentities, &structs.ACLServiceIdentity{
				ServiceName: bindName,
			})

		case structs.BindingRuleBindTypeNode:
			bindings.NodeIdentities = append(bindings.NodeIdentities, &structs.ACLNodeIdentity{
				NodeName:   bindName,
				Datacenter: b.datacenter,
			})

		case structs.BindingRuleBindTypeRole:
			_, role, err := b.store.ACLRoleGetByName(nil, bindName, &bindings.EnterpriseMeta)
			if err != nil {
				return nil, err
			}

			if role != nil {
				bindings.Roles = append(bindings.Roles, structs.ACLTokenRoleLink{
					ID: role.ID,
				})
			}
		}
	}

	return &bindings, nil
}

// IsValidBindName returns whether the given BindName template produces valid
// results when interpolating the auth method's available variables.
func IsValidBindName(bindType, bindName string, availableVariables []string) (bool, error) {
	if bindType == "" || bindName == "" {
		return false, nil
	}

	fakeVarMap := make(map[string]string)
	for _, v := range availableVariables {
		fakeVarMap[v] = "fake"
	}

	_, valid, err := computeBindName(bindType, bindName, fakeVarMap)
	if err != nil {
		return false, err
	}
	return valid, nil
}

// computeBindName processes the HIL for the provided bind type+name using the
// projected variables.
//
// - If the HIL is invalid ("", false, AN_ERROR) is returned.
// - If the computed name is not valid for the type ("INVALID_NAME", false, nil) is returned.
// - If the computed name is valid for the type ("VALID_NAME", true, nil) is returned.
func computeBindName(bindType, bindName string, projectedVars map[string]string) (string, bool, error) {
	bindName, err := template.InterpolateHIL(bindName, projectedVars, true)
	if err != nil {
		return "", false, err
	}

	var valid bool
	switch bindType {
	case structs.BindingRuleBindTypeService:
		valid = acl.IsValidServiceIdentityName(bindName)
	case structs.BindingRuleBindTypeNode:
		valid = acl.IsValidNodeIdentityName(bindName)
	case structs.BindingRuleBindTypeRole:
		valid = acl.IsValidRoleName(bindName)
	default:
		return "", false, fmt.Errorf("unknown binding rule bind type: %s", bindType)
	}

	return bindName, valid, nil
}

// doesSelectorMatch checks that a single selector matches the provided vars.
func doesSelectorMatch(selector string, selectableVars interface{}) bool {
	if selector == "" {
		return true // catch-all
	}

	eval, err := bexpr.CreateEvaluatorForType(selector, nil, selectableVars)
	if err != nil {
		return false // fails to match if selector is invalid
	}

	result, err := eval.Evaluate(selectableVars)
	if err != nil {
		return false // fails to match if evaluation fails
	}

	return result
}
