// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package auth

import (
	"errors"
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
	ACLPolicyGetByName(ws memdb.WatchSet, policyName string, entMeta *acl.EnterpriseMeta) (uint64, *structs.ACLPolicy, error)
}

// Bindings contains the ACL roles, service identities, node identities, policies,
// templated policies, and enterprise meta to be assigned to the created token.
type Bindings struct {
	Roles             []structs.ACLTokenRoleLink
	ServiceIdentities []*structs.ACLServiceIdentity
	NodeIdentities    []*structs.ACLNodeIdentity
	Policies          []structs.ACLTokenPolicyLink
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
		len(b.Roles) == 0 &&
		len(b.Policies) == 0
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
		switch rule.BindType {
		case structs.BindingRuleBindTypeService:
			bindName, err := computeBindName(rule.BindName, verifiedIdentity.ProjectedVars, acl.IsValidServiceIdentityName)
			if err != nil {
				return nil, err
			}
			bindings.ServiceIdentities = append(bindings.ServiceIdentities, &structs.ACLServiceIdentity{
				ServiceName: bindName,
			})

		case structs.BindingRuleBindTypeNode:
			bindName, err := computeBindName(rule.BindName, verifiedIdentity.ProjectedVars, acl.IsValidNodeIdentityName)
			if err != nil {
				return nil, err
			}
			bindings.NodeIdentities = append(bindings.NodeIdentities, &structs.ACLNodeIdentity{
				NodeName:   bindName,
				Datacenter: b.datacenter,
			})

		case structs.BindingRuleBindTypeTemplatedPolicy:
			templatedPolicy, err := generateTemplatedPolicies(rule.BindName, rule.BindVars, verifiedIdentity.ProjectedVars)
			if err != nil {
				return nil, err
			}
			bindings.TemplatedPolicies = append(bindings.TemplatedPolicies, templatedPolicy)

		case structs.BindingRuleBindTypePolicy:
			bindName, err := computeBindName(rule.BindName, verifiedIdentity.ProjectedVars, acl.IsValidRoleName)
			if err != nil {
				return nil, err
			}

			_, policy, err := b.store.ACLPolicyGetByName(nil, bindName, &bindings.EnterpriseMeta)
			if err != nil {
				return nil, err
			}

			if policy != nil {
				bindings.Policies = append(bindings.Policies, structs.ACLTokenPolicyLink{
					ID:   policy.ID,
					Name: policy.Name,
				})
			}

		case structs.BindingRuleBindTypeRole:
			bindName, err := computeBindName(rule.BindName, verifiedIdentity.ProjectedVars, acl.IsValidRoleName)
			if err != nil {
				return nil, err
			}

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

// IsValidBindingRule returns whether the given BindName and/or BindVars template produces valid
// results when interpolating the auth method's available variables.
func IsValidBindingRule(bindType, bindName string, bindVars *structs.ACLTemplatedPolicyVariables, availableVariables []string) error {
	if bindType == "" || bindName == "" {
		return errors.New("bindType and bindName must not be empty")
	}

	fakeVarMap := make(map[string]string)
	for _, v := range availableVariables {
		fakeVarMap[v] = "fake"
	}

	switch bindType {
	case structs.BindingRuleBindTypeService:
		if _, err := computeBindName(bindName, fakeVarMap, acl.IsValidServiceIdentityName); err != nil {
			return fmt.Errorf("failed to validate bindType %q: %w", bindType, err)
		}

	case structs.BindingRuleBindTypeNode:
		if _, err := computeBindName(bindName, fakeVarMap, acl.IsValidNodeIdentityName); err != nil {
			return fmt.Errorf("failed to validate bindType %q: %w", bindType, err)
		}

	case structs.BindingRuleBindTypeTemplatedPolicy:
		// If user-defined templated policies are supported in the future,
		// we will need to lookup state to ensure a template exists for given
		// bindName. A possible solution is to rip out the check for templated
		// policy into its own step which has access to the state store.
		if _, err := generateTemplatedPolicies(bindName, bindVars, fakeVarMap); err != nil {
			return fmt.Errorf("failed to validate bindType %q: %w", bindType, err)
		}

	case structs.BindingRuleBindTypeRole:
		if _, err := computeBindName(bindName, fakeVarMap, acl.IsValidRoleName); err != nil {
			return fmt.Errorf("failed to validate bindType %q: %w", bindType, err)
		}

	case structs.BindingRuleBindTypePolicy:
		if _, err := computeBindName(bindName, fakeVarMap, acl.IsValidPolicyName); err != nil {
			return fmt.Errorf("failed to validate bindType %q: %w", bindType, err)
		}
	default:
		return fmt.Errorf("invalid Binding Rule: unknown BindType %q", bindType)
	}

	return nil
}

// computeBindName interprets given HIL bindName with any given variables in projectedVars.
// validate (if not nil) will be called on the interpreted string.
func computeBindName(bindName string, projectedVars map[string]string, validate func(string) bool) (string, error) {
	computed, err := template.InterpolateHIL(bindName, projectedVars, true)
	if err != nil {
		return "", fmt.Errorf("error interpreting template: %w", err)
	}
	if validate != nil && !validate(computed) {
		return "", fmt.Errorf("invalid bind name: %q", computed)
	}
	return computed, nil
}

// generateTemplatedPolicies fetches a templated policy by bindName then attempts to interpret
// bindVars with any given variables in projectedVars. The resulting template is validated
// by the template's schema.
func generateTemplatedPolicies(
	bindName string,
	bindVars *structs.ACLTemplatedPolicyVariables,
	projectedVars map[string]string,
) (*structs.ACLTemplatedPolicy, error) {
	baseTemplate, ok := structs.GetACLTemplatedPolicyBase(bindName)
	if !ok {
		return nil, fmt.Errorf("Bind name for templated-policy bind type does not match existing template name: %s", bindName)
	}

	computedBindVars, err := computeBindVars(bindVars, projectedVars)
	if err != nil {
		return nil, fmt.Errorf("failed to interpret templated policy variables: %w", err)
	}

	out := &structs.ACLTemplatedPolicy{
		TemplateName:      bindName,
		TemplateVariables: computedBindVars,
		TemplateID:        baseTemplate.TemplateID,
	}

	if err := out.ValidateTemplatedPolicy(baseTemplate.Schema); err != nil {
		return nil, fmt.Errorf("templated policy failed validation: %w", err)
	}

	return out, nil
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
