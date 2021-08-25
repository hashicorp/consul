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

func (m *EnterpriseMeta) MergeNoWildcard(_ *EnterpriseMeta) {
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

func (m *EnterpriseMeta) WildcardEnterpriseMetaForPartition() *EnterpriseMeta {
	return &emptyEnterpriseMeta
}

func (m *EnterpriseMeta) DefaultEnterpriseMetaForPartition() *EnterpriseMeta {
	return &emptyEnterpriseMeta
}

func (m *EnterpriseMeta) NodeEnterpriseMetaForPartition() *EnterpriseMeta {
	return &emptyEnterpriseMeta
}

func (m *EnterpriseMeta) NewEnterpriseMetaInPartition(_ string) *EnterpriseMeta {
	return &emptyEnterpriseMeta
}

// TODO(partition): stop using this
func NewEnterpriseMetaInDefaultPartition(_ string) EnterpriseMeta {
	return emptyEnterpriseMeta
}

func NewEnterpriseMetaWithPartition(_, _ string) EnterpriseMeta {
	return emptyEnterpriseMeta
}

func (m *EnterpriseMeta) NamespaceOrDefault() string {
	return IntentionDefaultNamespace
}

func NamespaceOrDefault(_ string) string {
	return IntentionDefaultNamespace
}

func (m *EnterpriseMeta) NamespaceOrEmpty() string {
	return ""
}

func (m *EnterpriseMeta) InDefaultNamespace() bool {
	return true
}

func (m *EnterpriseMeta) PartitionOrDefault() string {
	return "default"
}

func PartitionOrDefault(_ string) string {
	return "default"
}

func (m *EnterpriseMeta) PartitionOrEmpty() string {
	return ""
}

func (m *EnterpriseMeta) InDefaultPartition() bool {
	return true
}

// ReplicationEnterpriseMeta stub
func ReplicationEnterpriseMeta() *EnterpriseMeta {
	return &emptyEnterpriseMeta
}

// TODO(partition): stop using this
func DefaultEnterpriseMetaInDefaultPartition() *EnterpriseMeta {
	return &emptyEnterpriseMeta
}

// DefaultEnterpriseMetaInPartition stub
func DefaultEnterpriseMetaInPartition(_ string) *EnterpriseMeta {
	return &emptyEnterpriseMeta
}

func NodeEnterpriseMetaInPartition(_ string) *EnterpriseMeta {
	return &emptyEnterpriseMeta
}

// TODO(partition): stop using this
func NodeEnterpriseMetaInDefaultPartition() *EnterpriseMeta {
	return &emptyEnterpriseMeta
}

// TODO(partition): stop using this
func WildcardEnterpriseMetaInDefaultPartition() *EnterpriseMeta {
	return &emptyEnterpriseMeta
}

// WildcardEnterpriseMetaInPartition stub
func WildcardEnterpriseMetaInPartition(_ string) *EnterpriseMeta {
	return &emptyEnterpriseMeta
}

// FillAuthzContext stub
func (_ *EnterpriseMeta) FillAuthzContext(_ *acl.AuthorizerContext) {}

func (_ *Node) FillAuthzContext(_ *acl.AuthorizerContext) {}

func (_ *Coordinate) FillAuthzContext(_ *acl.AuthorizerContext) {}

func (_ *NodeInfo) FillAuthzContext(_ *acl.AuthorizerContext) {}

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

func NodeNameString(node string, _ *EnterpriseMeta) string {
	return node
}

func ServiceIDString(id string, _ *EnterpriseMeta) string {
	return id
}

func ParseServiceIDString(input string) (string, *EnterpriseMeta) {
	return input, DefaultEnterpriseMetaInDefaultPartition()
}

func (sid ServiceID) String() string {
	return sid.ID
}

func ServiceIDFromString(input string) ServiceID {
	id, _ := ParseServiceIDString(input)
	return ServiceID{ID: id}
}

func ParseServiceNameString(input string) (string, *EnterpriseMeta) {
	return input, DefaultEnterpriseMetaInDefaultPartition()
}

func (n ServiceName) String() string {
	return n.Name
}

func ServiceNameFromString(input string) ServiceName {
	id, _ := ParseServiceNameString(input)
	return ServiceName{Name: id}
}

func (cid CheckID) String() string {
	return string(cid.ID)
}

func (_ *HealthCheck) Validate() error {
	return nil
}

func enterpriseRequestType(m MessageType) (string, bool) {
	return "", false
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

func (t *Intention) HasWildcardSource() bool {
	return t.SourceName == WildcardSpecifier
}

func (t *Intention) HasWildcardDestination() bool {
	return t.DestinationName == WildcardSpecifier
}

func (s *ServiceNode) NodeIdentity() Identity {
	return Identity{ID: s.Node}
}
