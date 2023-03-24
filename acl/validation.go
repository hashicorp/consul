package acl

import "regexp"

const (
	ServiceIdentityNameMaxLength = 256
	NodeIdentityNameMaxLength    = 256
)

var (
	validServiceIdentityName = regexp.MustCompile(`^[a-z0-9]([a-z0-9\-_]*[a-z0-9])?$`)
	validNodeIdentityName    = regexp.MustCompile(`^[a-z0-9]([a-z0-9\-_]*[a-z0-9])?$`)
	validPolicyName          = regexp.MustCompile(`^[A-Za-z0-9\-_]{1,128}$`)
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

// IsValidPolicyName returns true if the provided name can be used as an
// ACLPolicy Name.
func IsValidPolicyName(name string) bool {
	return validPolicyName.MatchString(name)
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
