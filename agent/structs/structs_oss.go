// +build !consulent

package structs

import (
	"hash"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/types"
)

var emptyEnterpriseMeta = EnterpriseMeta{}

// EnterpriseMeta stub
type EnterpriseMeta struct{}

func (m *EnterpriseMeta) estimateSize() int {
	return 0
}

func (m *EnterpriseMeta) addToHash(_ hash.Hash, _ bool) {
	// do nothing
}

func (m *EnterpriseMeta) Merge(_ *EnterpriseMeta) {
	// do nothing
}

func (m *EnterpriseMeta) Matches(_ *EnterpriseMeta) bool {
	return true
}

func (m *EnterpriseMeta) IsSame(_ *EnterpriseMeta) bool {
	return true
}

func (m *EnterpriseMeta) LessThan(_ *EnterpriseMeta) bool {
	return false
}

// ReplicationEnterpriseMeta stub
func ReplicationEnterpriseMeta() *EnterpriseMeta {
	return &emptyEnterpriseMeta
}

// DefaultEnterpriseMeta stub
func DefaultEnterpriseMeta() *EnterpriseMeta {
	return &emptyEnterpriseMeta
}

// WildcardEnterpriseMeta stub
func WildcardEnterpriseMeta() *EnterpriseMeta {
	return &emptyEnterpriseMeta
}

// FillAuthzContext stub
func (_ *EnterpriseMeta) FillAuthzContext(_ *acl.AuthorizerContext) {}

func (_ *EnterpriseMeta) Normalize() {}

// FillAuthzContext stub
func (_ *DirEntry) FillAuthzContext(_ *acl.AuthorizerContext) {}

// FillAuthzContext stub
func (_ *RegisterRequest) FillAuthzContext(_ *acl.AuthorizerContext) {}

func (_ *RegisterRequest) GetEnterpriseMeta() *EnterpriseMeta {
	return nil
}

// OSS Stub
func (op *TxnNodeOp) FillAuthzContext(ctx *acl.AuthorizerContext) {}

// OSS Stub
func (_ *TxnServiceOp) FillAuthzContext(_ *acl.AuthorizerContext) {}

// OSS Stub
func (_ *TxnCheckOp) FillAuthzContext(_ *acl.AuthorizerContext) {}

func ServiceIDString(id string, _ *EnterpriseMeta) string {
	return id
}

func (sid *ServiceID) String() string {
	return sid.ID
}

func (cid *CheckID) String() string {
	return string(cid.ID)
}

func (_ *HealthCheck) Validate() error {
	return nil
}

// CheckIDs returns the IDs for all checks associated with a session, regardless of type
func (s *Session) CheckIDs() []types.CheckID {
	// Merge all check IDs into a single slice, since they will be handled the same way
	checks := make([]types.CheckID, 0, len(s.Checks)+len(s.NodeChecks)+len(s.ServiceChecks))
	checks = append(checks, s.Checks...)

	for _, c := range s.NodeChecks {
		checks = append(checks, types.CheckID(c))
	}

	for _, c := range s.ServiceChecks {
		checks = append(checks, types.CheckID(c.ID))
	}
	return checks
}
