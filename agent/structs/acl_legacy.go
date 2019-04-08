// DEPRECATED (ACL-Legacy-Compat)
//
// Everything within this file is deprecated and related to the original ACL
// implementation. Once support for v1 ACLs are removed this whole file can
// be deleted.

package structs

import (
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/consul/acl"
)

const (
	// ACLBootstrapInit is used to perform a scan for existing tokens which
	// will decide whether bootstrapping is allowed for a cluster. This is
	// initiated by the leader when it steps up, if necessary.
	ACLBootstrapInit ACLOp = "bootstrap-init"

	// ACLBootstrapNow is used to perform a one-time ACL bootstrap operation on
	// a cluster to get the first management token.
	ACLBootstrapNow ACLOp = "bootstrap-now"

	// ACLForceSet is deprecated, but left for backwards compatibility.
	ACLForceSet ACLOp = "force-set"
)

// ACLBootstrapNotInitializedErr is returned when a bootstrap is attempted but
// we haven't yet initialized ACL bootstrap. It provides some guidance to
// operators on how to proceed.
var ACLBootstrapNotInitializedErr = errors.New("ACL bootstrap not initialized, need to force a leader election and ensure all Consul servers support this feature")

const (
	// ACLTokenTypeClient tokens have rules applied
	ACLTokenTypeClient = "client"

	// ACLTokenTypeManagement tokens have an always allow policy, so they can
	// make other tokens and can access all resources.
	ACLTokenTypeManagement = "management"

	// ACLTokenTypeNone
	ACLTokenTypeNone = ""
)

// ACL is used to represent a token and its rules
type ACL struct {
	ID    string
	Name  string
	Type  string
	Rules string

	RaftIndex
}

// ACLs is a slice of ACLs.
type ACLs []*ACL

// Convert does a 1-1 mapping of the ACLCompat structure to its ACLToken
// equivalent. This will NOT fill in the other ACLToken fields or perform any other
// upgrade (other than correcting an older HCL syntax that is no longer
// supported).
func (a *ACL) Convert() *ACLToken {
	// Ensure that we correct any old HCL in legacy tokens to prevent old
	// syntax from leaking elsewhere into the system.
	//
	// DEPRECATED (ACL-Legacy-Compat)
	correctedRules := SanitizeLegacyACLTokenRules(a.Rules)
	if correctedRules != "" {
		a.Rules = correctedRules
	}

	return &ACLToken{
		AccessorID:        "",
		SecretID:          a.ID,
		Description:       a.Name,
		Policies:          nil,
		ServiceIdentities: nil,
		Type:              a.Type,
		Rules:             a.Rules,
		Local:             false,
		RaftIndex:         a.RaftIndex,
	}
}

// Convert attempts to convert an ACLToken into an ACLCompat.
func (tok *ACLToken) Convert() (*ACL, error) {
	if tok.Type == "" {
		return nil, fmt.Errorf("Cannot convert ACLToken into compat token")
	}

	compat := &ACL{
		ID:        tok.SecretID,
		Name:      tok.Description,
		Type:      tok.Type,
		Rules:     tok.Rules,
		RaftIndex: tok.RaftIndex,
	}
	return compat, nil
}

// IsSame checks if one ACL is the same as another, without looking
// at the Raft information (that's why we didn't call it IsEqual). This is
// useful for seeing if an update would be idempotent for all the functional
// parts of the structure.
func (a *ACL) IsSame(other *ACL) bool {
	if a.ID != other.ID ||
		a.Name != other.Name ||
		a.Type != other.Type ||
		a.Rules != other.Rules {
		return false
	}

	return true
}

// ACLRequest is used to create, update or delete an ACL
type ACLRequest struct {
	Datacenter string
	Op         ACLOp
	ACL        ACL
	WriteRequest
}

func (r *ACLRequest) RequestDatacenter() string {
	return r.Datacenter
}

// ACLRequests is a list of ACL change requests.
type ACLRequests []*ACLRequest

// ACLSpecificRequest is used to request an ACL by ID
type ACLSpecificRequest struct {
	Datacenter string
	ACL        string
	QueryOptions
}

// RequestDatacenter returns the DC this request is targeted to.
func (r *ACLSpecificRequest) RequestDatacenter() string {
	return r.Datacenter
}

// IndexedACLs has tokens along with the Raft metadata about them.
type IndexedACLs struct {
	ACLs ACLs
	QueryMeta
}

// ACLBootstrap keeps track of whether bootstrapping ACLs is allowed for a
// cluster.
type ACLBootstrap struct {
	// AllowBootstrap will only be true if no existing management tokens
	// have been found.
	AllowBootstrap bool

	RaftIndex
}

// ACLPolicyResolveLegacyRequest is used to request an ACL by Token SecretID, conditionally
// filtering on an ID
type ACLPolicyResolveLegacyRequest struct {
	Datacenter string // The Datacenter the RPC may be sent to
	ACL        string // The Tokens Secret ID
	ETag       string // Caching ETag to prevent resending the policy when not needed
	QueryOptions
}

// RequestDatacenter returns the DC this request is targeted to.
func (r *ACLPolicyResolveLegacyRequest) RequestDatacenter() string {
	return r.Datacenter
}

type ACLPolicyResolveLegacyResponse struct {
	ETag   string
	Parent string
	Policy *acl.Policy
	TTL    time.Duration
	QueryMeta
}
