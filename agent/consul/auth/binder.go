// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

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
	TemplatedPolicies structs.ACLTemplatedPolicies
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
		len(b.TemplatedPolicies) == 0 &&
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

	// Compute role, service identity, node identity or templated policy names by interpolating
	// the identity's projected variables into the rule BindName templates.
	for _, rule := range matchingRules {
		bindName, templatedPolicy, valid, err := computeBindNameAndVars(rule.BindType, rule.BindName, rule.BindVars, verifiedIdentity.ProjectedVars)
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

		case structs.BindingRuleBindTypeTemplatedPolicy:
			bindings.TemplatedPolicies = append(bindings.TemplatedPolicies, templatedPolicy)

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

// IsValidBindNameOrBindVars returns whether the given BindName and/or BindVars template produces valid
// results when interpolating the auth method's available variables.
func IsValidBindNameOrBindVars(bindType, bindName string, bindVars *structs.ACLTemplatedPolicyVariables, availableVariables []string) (bool, error) {
	if bindType == "" || bindName == "" {
		return false, nil
	}

	fakeVarMap := make(map[string]string)
	for _, v := range availableVariables {
		fakeVarMap[v] = "fake"
	}

	_, _, valid, err := computeBindNameAndVars(bindType, bindName, bindVars, fakeVarMap)
	if err != nil {
		return false, err
	}
	return valid, nil
}

// computeBindNameAndVars processes the HIL for the provided bind type+name+vars using the
// projected variables. When bindtype is templated-policy, it returns the resulting templated policy
// otherwise, returns nil
//
// when bindtype is not templated-policy: it evaluates bindName
// - If the HIL is invalid ("", nil, false, AN_ERROR) is returned.
// - If the computed name is not valid for the type ("INVALID_NAME", nil, false, nil) is returned.
// - If the computed name is valid for the type ("VALID_NAME", nil, true, nil) is returned.
// when bindtype is templated-policy: it evalueates both bindName and bindVars
// - If the computed bindvars(failing templated policy schema validation) are invalid ("", nil, false, AN_ERROR) is returned.
// - if the HIL in bindvars is invalid it returns ("", nil, false, AN_ERROR)
// - if the computed bindvars are valid and templated policy validation is successful it returns (bindName, TemplatedPolicy, true, nil)
func computeBindNameAndVars(bindType, bindName string, bindVars *structs.ACLTemplatedPolicyVariables, projectedVars map[string]string) (string, *structs.ACLTemplatedPolicy, bool, error) {
	bindName, err := template.InterpolateHIL(bindName, projectedVars, true)
	if err != nil {
		return "", nil, false, err
	}

	var templatedPolicy *structs.ACLTemplatedPolicy
	var valid bool
	switch bindType {
	case structs.BindingRuleBindTypeService:
		valid = acl.IsValidServiceIdentityName(bindName)
	case structs.BindingRuleBindTypeNode:
		valid = acl.IsValidNodeIdentityName(bindName)
	case structs.BindingRuleBindTypeRole:
		valid = acl.IsValidRoleName(bindName)
	case structs.BindingRuleBindTypeTemplatedPolicy:
		templatedPolicy, valid, err = generateTemplatedPolicies(bindName, bindVars, projectedVars)
		if err != nil {
			return "", nil, false, err
		}

	default:
		return "", nil, false, fmt.Errorf("unknown binding rule bind type: %s", bindType)
	}

	return bindName, templatedPolicy, valid, nil
}

func generateTemplatedPolicies(bindName string, bindVars *structs.ACLTemplatedPolicyVariables, projectedVars map[string]string) (*structs.ACLTemplatedPolicy, bool, error) {
	computedBindVars, err := computeBindVars(bindVars, projectedVars)
	if err != nil {
		return nil, false, err
	}

	baseTemplate, ok := structs.GetACLTemplatedPolicyBase(bindName)

	if !ok {
		return nil, false, fmt.Errorf("Bind name for templated-policy bind type does not match existing template name: %s", bindName)
	}

	out := &structs.ACLTemplatedPolicy{
		TemplateName:      bindName,
		TemplateVariables: computedBindVars,
		TemplateID:        baseTemplate.TemplateID,
	}

	err = out.ValidateTemplatedPolicy(baseTemplate.Schema)
	if err != nil {
		return nil, false, fmt.Errorf("templated policy failed validation. Error: %v", err)
	}

	return out, true, nil
}

func computeBindVars(bindVars *structs.ACLTemplatedPolicyVariables, projectedVars map[string]string) (*structs.ACLTemplatedPolicyVariables, error) {
	if bindVars == nil {
		return nil, nil
	}

	out := &structs.ACLTemplatedPolicyVariables{}
	if bindVars.Name != "" {
		nameValue, err := template.InterpolateHIL(bindVars.Name, projectedVars, true)
		if err != nil {
			return nil, err
		}
		out.Name = nameValue
	}

	return out, nil
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
