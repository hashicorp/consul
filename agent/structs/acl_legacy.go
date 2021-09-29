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
