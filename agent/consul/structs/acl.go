package structs

import (
	"time"

	"github.com/hashicorp/consul/acl"
)

// ACLOp is used in RPCs to encode ACL operations.
type ACLOp string

const (
	// ACLSet creates or updates a token.
	ACLSet ACLOp = "set"

	// ACLForceSet is deprecated, but left for backwards compatibility.
	ACLForceSet = "force-set"

	// ACLDelete deletes a token.
	ACLDelete = "delete"
)

const (
	// ACLTypeClient tokens have rules applied
	ACLTypeClient = "client"

	// ACLTypeManagement tokens have an always allow policy, so they can
	// make other tokens and can access all resources.
	ACLTypeManagement = "management"
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

// ACLPolicyRequest is used to request an ACL by ID, conditionally
// filtering on an ID
type ACLPolicyRequest struct {
	Datacenter string
	ACL        string
	ETag       string
	QueryOptions
}

// RequestDatacenter returns the DC this request is targeted to.
func (r *ACLPolicyRequest) RequestDatacenter() string {
	return r.Datacenter
}

// IndexedACLs has tokens along with the Raft metadata about them.
type IndexedACLs struct {
	ACLs ACLs
	QueryMeta
}

// ACLPolicy is a policy that can be associated with a token.
type ACLPolicy struct {
	ETag   string
	Parent string
	Policy *acl.Policy
	TTL    time.Duration
	QueryMeta
}

// ACLReplicationStatus provides information about the health of the ACL
// replication system.
type ACLReplicationStatus struct {
	Enabled          bool
	Running          bool
	SourceDatacenter string
	ReplicatedIndex  uint64
	LastSuccess      time.Time
	LastError        time.Time
}
