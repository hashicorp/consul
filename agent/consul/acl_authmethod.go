package consul

import (
	"fmt"

	"github.com/hashicorp/consul/agent/consul/authmethod"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-bexpr"

	// register these as a builtin auth method
	_ "github.com/hashicorp/consul/agent/consul/authmethod/kubeauth"
	_ "github.com/hashicorp/consul/agent/consul/authmethod/ssoauth"
)

type authMethodValidatorEntry struct {
	Validator   authmethod.Validator
	ModifyIndex uint64 // the raft index when this last changed
}

// loadAuthMethodValidator returns an authmethod.Validator for the given auth
// method configuration. If the cache is up to date as-of the provided index
// then the cached version is returned, otherwise a new validator is created
// and cached.
func (s *Server) loadAuthMethodValidator(idx uint64, method *structs.ACLAuthMethod) (authmethod.Validator, error) {
	if prevIdx, v, ok := s.aclAuthMethodValidators.GetValidator(method); ok && idx <= prevIdx {
		return v, nil
	}

	v, err := authmethod.NewValidator(s.logger, method)
	if err != nil {
		return nil, fmt.Errorf("auth method validator for %q could not be initialized: %v", method.Name, err)
	}

	v = s.aclAuthMethodValidators.PutValidatorIfNewer(method, v, idx)

	return v, nil
}

type aclBindings struct {
	roles             []structs.ACLTokenRoleLink
	serviceIdentities []*structs.ACLServiceIdentity
	nodeIdentities    []*structs.ACLNodeIdentity
}

// evaluateRoleBindings evaluates all current binding rules associated with the
// given auth method against the verified data returned from the authentication
// process.
//
// A list of role links and service identities are returned.
func (s *Server) evaluateRoleBindings(
	validator authmethod.Validator,
	verifiedIdentity *authmethod.Identity,
	methodMeta *structs.EnterpriseMeta,
	targetMeta *structs.EnterpriseMeta,
) (*aclBindings, error) {
	// Only fetch rules that are relevant for this method.
	_, rules, err := s.fsm.State().ACLBindingRuleList(nil, validator.Name(), methodMeta)
	if err != nil {
		return nil, err
	} else if len(rules) == 0 {
		return nil, nil
	}

	// Find all binding rules that match the provided fields.
	var matchingRules []*structs.ACLBindingRule
	for _, rule := range rules {
		if doesSelectorMatch(rule.Selector, verifiedIdentity.SelectableFields) {
			matchingRules = append(matchingRules, rule)
		}
	}
	if len(matchingRules) == 0 {
		return nil, nil
	}

	// For all matching rules compute the attributes of a token.
	var bindings aclBindings
	for _, rule := range matchingRules {
		bindName, valid, err := computeBindingRuleBindName(rule.BindType, rule.BindName, verifiedIdentity.ProjectedVars)
		if err != nil {
			return nil, fmt.Errorf("cannot compute %q bind name for bind target: %v", rule.BindType, err)
		} else if !valid {
			return nil, fmt.Errorf("computed %q bind name for bind target is invalid: %q", rule.BindType, bindName)
		}

		switch rule.BindType {
		case structs.BindingRuleBindTypeService:
			bindings.serviceIdentities = append(bindings.serviceIdentities, &structs.ACLServiceIdentity{
				ServiceName: bindName,
			})

		case structs.BindingRuleBindTypeNode:
			bindings.nodeIdentities = append(bindings.nodeIdentities, &structs.ACLNodeIdentity{
				NodeName:   bindName,
				Datacenter: s.config.Datacenter,
			})

		case structs.BindingRuleBindTypeRole:
			_, role, err := s.fsm.State().ACLRoleGetByName(nil, bindName, targetMeta)
			if err != nil {
				return nil, err
			}

			if role != nil {
				bindings.roles = append(bindings.roles, structs.ACLTokenRoleLink{
					ID: role.ID,
				})
			}

		default:
			// skip unknown bind type; don't grant privileges
		}
	}

	return &bindings, nil
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
