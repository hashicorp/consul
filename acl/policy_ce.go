// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package acl

import (
	"fmt"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/hcl"
	"regexp"

	"strings"
)

// EnterprisePolicyMeta stub
type EnterprisePolicyMeta struct{}

// EnterpriseRule stub
type EnterpriseRule struct{}

func (r *EnterpriseRule) Validate(string, *Config) error {
	// nothing to validate
	return nil
}

// EnterprisePolicyRules stub
type EnterprisePolicyRules struct{}

func (r *EnterprisePolicyRules) Validate(*Config) error {
	// nothing to validate
	return nil
}

func decodeRules(rules string, warnOnDuplicateKey bool, _ *Config, _ *EnterprisePolicyMeta) (*Policy, error) {
	p := &Policy{}

	err := hcl.Decode(p, rules)

	if errIsDuplicateKey(err) && warnOnDuplicateKey {
		//because the snapshot saves the unparsed rules we have to assume some snapshots exist that shouldn't fail, but
		// have duplicates
		rules = cleanDuplicates(rules, err)
		if err := hcl.Decode(p, rules); err != nil {
			hclog.Default().Warn("Warning- Duplicate key in ACL Policy ignored", "errorMessage", err.Error())
			return nil, fmt.Errorf("Failed to parse ACL rules: %v", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("Failed to parse ACL rules: %v", err)
	}

	return p, nil
}

func errIsDuplicateKey(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "was already set. Each argument can only be defined once")
}

// This replicates the behavior of the previous HCL parser in certain conditions, like when reading
// old ACL from the cache
func cleanDuplicates(rules string, err error) string {
	p := &Policy{}
	//hcl doesn't care about spaces or commas so we can remove them to make life easier
	rules = strings.ReplaceAll(rules, " ", "")
	rules = strings.ReplaceAll(rules, ",", "")

	errRegexp, _ := regexp.Compile("The argument \"(.+)\"")
	//we need to do this to maintain backwards compatabiliy, luckily the rules only contain string values
	for errIsDuplicateKey(err) {
		//capture current duplicate from error message. We already know there is a match from the above check
		submatch := errRegexp.FindAllStringSubmatch(err.Error(), 1)
		key := submatch[0][1]
		keyRegexp, _ := regexp.Compile(key + `="[A-Za-z0-9]+"`)
		keysubmatch := keyRegexp.FindAllStringSubmatch(rules, 1)
		match := ""
		if len(keysubmatch) > 0 {
			//found a match
			match = keysubmatch[0][0]
		} else {
			//no match found, return error up to caller
			return rules
		}
		//replace the captured block 1 time to preserve the other instance of the key, then check the cleaned string
		//for duplicate key errors
		rules = strings.Replace(rules, match, "", 1)
		err = hcl.Decode(p, rules)
	}
	return rules
}
