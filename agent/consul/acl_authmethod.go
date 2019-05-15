package consul

import (
	"fmt"

	"github.com/hashicorp/consul/agent/consul/authmethod"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-bexpr"

	// register this as a builtin auth method
	_ "github.com/hashicorp/consul/agent/consul/authmethod/kubeauth"
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
	if prevIdx, v, ok := s.getCachedAuthMethodValidator(method.Name); ok && idx <= prevIdx {
		return v, nil
	}

	v, err := authmethod.NewValidator(method)
	if err != nil {
		return nil, fmt.Errorf("auth method validator for %q could not be initialized: %v", method.Name, err)
	}

	v = s.getOrReplaceAuthMethodValidator(method.Name, idx, v)

	return v, nil
}

// getCachedAuthMethodValidator returns an AuthMethodValidator for
// the given name exclusively from the cache. If one is not found in the cache
// nil is returned.
func (s *Server) getCachedAuthMethodValidator(name string) (uint64, authmethod.Validator, bool) {
	s.aclAuthMethodValidatorLock.RLock()
	defer s.aclAuthMethodValidatorLock.RUnlock()

	if s.aclAuthMethodValidators != nil {
		v, ok := s.aclAuthMethodValidators[name]
		if ok {
			return v.ModifyIndex, v.Validator, true
		}
	}
	return 0, nil, false
}

// getOrReplaceAuthMethodValidator updates the cached validator with the
// provided one UNLESS it has been updated by another goroutine in which case
// the updated one is returned.
func (s *Server) getOrReplaceAuthMethodValidator(name string, idx uint64, v authmethod.Validator) authmethod.Validator {
	s.aclAuthMethodValidatorLock.Lock()
	defer s.aclAuthMethodValidatorLock.Unlock()

	if s.aclAuthMethodValidators == nil {
		s.aclAuthMethodValidators = make(map[string]*authMethodValidatorEntry)
	}

	prev, ok := s.aclAuthMethodValidators[name]
	if ok {
		if prev.ModifyIndex >= idx {
			return prev.Validator
		}
	}

	s.logger.Printf("[DEBUG] acl: updating cached auth method validator for %q", name)

	s.aclAuthMethodValidators[name] = &authMethodValidatorEntry{
		Validator:   v,
		ModifyIndex: idx,
	}
	return v
}

// purgeAuthMethodValidators resets the cache of validators.
func (s *Server) purgeAuthMethodValidators() {
	s.aclAuthMethodValidatorLock.Lock()
	s.aclAuthMethodValidators = make(map[string]*authMethodValidatorEntry)
	s.aclAuthMethodValidatorLock.Unlock()
}

// evaluateRoleBindings evaluates all current binding rules associated with the
// given auth method against the verified data returned from the authentication
// process.
//
// A list of role links and service identities are returned.
func (s *Server) evaluateRoleBindings(
	validator authmethod.Validator,
	verifiedFields map[string]string,
) ([]*structs.ACLServiceIdentity, []structs.ACLTokenRoleLink, error) {
	// Only fetch rules that are relevant for this method.
	_, rules, err := s.fsm.State().ACLBindingRuleList(nil, validator.Name())
	if err != nil {
		return nil, nil, err
	} else if len(rules) == 0 {
		return nil, nil, nil
	}

	// Convert the fields into something suitable for go-bexpr.
	selectableVars := validator.MakeFieldMapSelectable(verifiedFields)

	// Find all binding rules that match the provided fields.
	var matchingRules []*structs.ACLBindingRule
	for _, rule := range rules {
		if doesBindingRuleMatch(rule, selectableVars) {
			matchingRules = append(matchingRules, rule)
		}
	}
	if len(matchingRules) == 0 {
		return nil, nil, nil
	}

	// For all matching rules compute the attributes of a token.
	var (
		roleLinks         []structs.ACLTokenRoleLink
		serviceIdentities []*structs.ACLServiceIdentity
	)
	for _, rule := range matchingRules {
		bindName, valid, err := computeBindingRuleBindName(rule.BindType, rule.BindName, verifiedFields)
		if err != nil {
			return nil, nil, fmt.Errorf("cannot compute %q bind name for bind target: %v", rule.BindType, err)
		} else if !valid {
			return nil, nil, fmt.Errorf("computed %q bind name for bind target is invalid: %q", rule.BindType, bindName)
		}

		switch rule.BindType {
		case structs.BindingRuleBindTypeService:
			serviceIdentities = append(serviceIdentities, &structs.ACLServiceIdentity{
				ServiceName: bindName,
			})

		case structs.BindingRuleBindTypeRole:
			_, role, err := s.fsm.State().ACLRoleGetByName(nil, bindName)
			if err != nil {
				return nil, nil, err
			}

			if role != nil {
				roleLinks = append(roleLinks, structs.ACLTokenRoleLink{
					ID: role.ID,
				})
			}

		default:
			// skip unknown bind type; don't grant privileges
		}
	}

	return serviceIdentities, roleLinks, nil
}

// doesBindingRuleMatch checks that a single binding rule matches the provided
// vars.
func doesBindingRuleMatch(rule *structs.ACLBindingRule, selectableVars interface{}) bool {
	if rule.Selector == "" {
		return true // catch-all
	}

	eval, err := bexpr.CreateEvaluatorForType(rule.Selector, nil, selectableVars)
	if err != nil {
		return false // fails to match if selector is invalid
	}

	result, err := eval.Evaluate(selectableVars)
	if err != nil {
		return false // fails to match if evaluation fails
	}

	return result
}
