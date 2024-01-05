// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package acl

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	ServiceIdentityNameMaxLength = 256
	NodeIdentityNameMaxLength    = 256
	PolicyNameMaxLength          = 128
)

var (
	validServiceIdentityName = regexp.MustCompile(`^[a-z0-9]([a-z0-9\-_]*[a-z0-9])?$`)
	validNodeIdentityName    = regexp.MustCompile(`^[a-z0-9]([a-z0-9\-_]*[a-z0-9])?$`)
	validPolicyName          = regexp.MustCompile(`^[A-Za-z0-9\-_]+\/?[A-Za-z0-9\-_]*$`)
	validRoleName            = regexp.MustCompile(`^[A-Za-z0-9\-_]{1,256}$`)
	validAuthMethodName      = regexp.MustCompile(`^[A-Za-z0-9\-_]{1,128}$`)
)

// IsValidServiceIdentityName returns true if the provided name can be used as
// an ACLServiceIdentity ServiceName. This is more restrictive than standard
// catalog registration, which basically takes the view that "everything is
// valid".
func IsValidServiceIdentityName(name string) bool {
	if len(name) < 1 || len(name) > ServiceIdentityNameMaxLength {
		return false
	}
	return validServiceIdentityName.MatchString(name)
}

// IsValidNodeIdentityName returns true if the provided name can be used as
// an ACLNodeIdentity NodeName. This is more restrictive than standard
// catalog registration, which basically takes the view that "everything is
// valid".
func IsValidNodeIdentityName(name string) bool {
	if len(name) < 1 || len(name) > NodeIdentityNameMaxLength {
		return false
	}
	return validNodeIdentityName.MatchString(name)
}

// ValidatePolicyName returns nil if the provided name can be used as an
// ACLPolicy Name otherwise a useful error is returned.
func ValidatePolicyName(name string) error {
	if len(name) < 1 || len(name) > PolicyNameMaxLength {
		return fmt.Errorf("Invalid Policy: invalid Name. Length must be greater than 0 and less than %d", PolicyNameMaxLength)
	}

	if strings.HasPrefix(name, "/") || strings.HasPrefix(name, ReservedBuiltinPrefix) {
		return fmt.Errorf("Invalid Policy: invalid Name. Names cannot be prefixed with '/' or '%s'", ReservedBuiltinPrefix)
	}

	if !validPolicyName.MatchString(name) {
		return fmt.Errorf("Invalid Policy: invalid Name. Only alphanumeric characters, a single '/', '-' and '_' are allowed")
	}
	return nil
}

// IsValidRoleName returns true if the provided name can be used as an
// ACLRole Name.
func IsValidRoleName(name string) bool {
	return validRoleName.MatchString(name)
}

// IsValidRoleName returns true if the provided name can be used as an
// ACLAuthMethod Name.
func IsValidAuthMethodName(name string) bool {
	return validAuthMethodName.MatchString(name)
}
