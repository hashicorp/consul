// DEPRECATED (ACL-Legacy-Compat)
//
// Everything within this file is deprecated and related to the original ACL
// implementation. Once support for v1 ACLs are removed this whole file can
// be deleted.

package structs

const (
	// ACLTokenTypeClient tokens have rules applied
	ACLTokenTypeClient = "client"

	// ACLTokenTypeManagement tokens have an always allow policy, so they can
	// make other tokens and can access all resources.
	ACLTokenTypeManagement = "management"
)

// ACL is used to represent a token and its rules
type ACL struct {
	ID    string
	Name  string
	Type  string
	Rules string

	RaftIndex
}

// Convert does a 1-1 mapping of the ACLCompat structure to its ACLToken
// equivalent. This will NOT fill in the other ACLToken fields or perform any other
// upgrade (other than correcting an older HCL syntax that is no longer
// supported).
// TODO(ACL-Legacy-Compat): remove in phase 2, used by snapshot restore
func (a *ACL) Convert() *ACLToken {
	// Ensure that we correct any old HCL in legacy tokens to prevent old
	// syntax from leaking elsewhere into the system.
	//
	// DEPRECATED (ACL-Legacy-Compat)
	correctedRules := SanitizeLegacyACLTokenRules(a.Rules)
	if correctedRules != "" {
		a.Rules = correctedRules
	}

	token := &ACLToken{
		AccessorID:        "",
		SecretID:          a.ID,
		Description:       a.Name,
		Policies:          nil,
		ServiceIdentities: nil,
		NodeIdentities:    nil,
		Type:              a.Type,
		Rules:             a.Rules,
		Local:             false,
		RaftIndex:         a.RaftIndex,
	}

	token.SetHash(true)
	return token
}
