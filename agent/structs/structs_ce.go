//go:build !consulent
// +build !consulent

package structs

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/types"
)

// TODO(acl-move-enterprise-meta) sync this with enterprise
var emptyEnterpriseMeta = acl.EnterpriseMeta{}

// TODO(partition): stop using this
func NewEnterpriseMetaInDefaultPartition(_ string) acl.EnterpriseMeta {
	return emptyEnterpriseMeta
}

// ReplicationEnterpriseMeta stub
func ReplicationEnterpriseMeta() *acl.EnterpriseMeta {
	return &emptyEnterpriseMeta
}

// TODO(partition): stop using this
func WildcardEnterpriseMetaInDefaultPartition() *acl.EnterpriseMeta {
	return &emptyEnterpriseMeta
}

// TODO(partition): stop using this
func DefaultEnterpriseMetaInDefaultPartition() *acl.EnterpriseMeta {
	return &emptyEnterpriseMeta
}

// DefaultEnterpriseMetaInPartition stub
func DefaultEnterpriseMetaInPartition(_ string) *acl.EnterpriseMeta {
	return &emptyEnterpriseMeta
}

// WildcardEnterpriseMetaInPartition stub
func WildcardEnterpriseMetaInPartition(_ string) *acl.EnterpriseMeta {
	return &emptyEnterpriseMeta
}

func NewEnterpriseMetaWithPartition(_, _ string) acl.EnterpriseMeta {
	return emptyEnterpriseMeta
}

func NodeEnterpriseMetaInPartition(_ string) *acl.EnterpriseMeta {
	return &emptyEnterpriseMeta
}

// TODO(partition): stop using this
func NodeEnterpriseMetaInDefaultPartition() *acl.EnterpriseMeta {
	return &emptyEnterpriseMeta
}

func (n *Node) FillAuthzContext(ctx *acl.AuthorizerContext) {
	if ctx == nil {
		return
	}
	ctx.Peer = n.PeerName
}

func (n *Node) OverridePartition(_ string) {
	n.Partition = ""
}

func (_ *Coordinate) FillAuthzContext(_ *acl.AuthorizerContext) {}

func (n *NodeInfo) FillAuthzContext(ctx *acl.AuthorizerContext) {
	ctx.Peer = n.PeerName
}

// FillAuthzContext stub
func (_ *DirEntry) FillAuthzContext(_ *acl.AuthorizerContext) {}

// FillAuthzContext stub
func (_ *RegisterRequest) FillAuthzContext(_ *acl.AuthorizerContext) {}

func (_ *RegisterRequest) GetEnterpriseMeta() *acl.EnterpriseMeta {
	return nil
}

// CE Stub
func (op *TxnNodeOp) FillAuthzContext(ctx *acl.AuthorizerContext) {}

// CE Stub
func (_ *TxnServiceOp) FillAuthzContext(_ *acl.AuthorizerContext) {}

// CE Stub
func (_ *TxnCheckOp) FillAuthzContext(_ *acl.AuthorizerContext) {}

func NodeNameString(node string, _ *acl.EnterpriseMeta) string {
	return node
}

func ServiceIDString(id string, _ *acl.EnterpriseMeta) string {
	return id
}

func ParseServiceIDString(input string) (string, *acl.EnterpriseMeta) {
	return input, DefaultEnterpriseMetaInDefaultPartition()
}

func (sid ServiceID) String() string {
	return sid.ID
}

func ServiceIDFromString(input string) ServiceID {
	id, _ := ParseServiceIDString(input)
	return ServiceID{ID: id}
}

func ParseServiceNameString(input string) (string, *acl.EnterpriseMeta) {
	return input, DefaultEnterpriseMetaInDefaultPartition()
}

func (n ServiceName) String() string {
	return n.Name
}

func ServiceNameFromString(input string) ServiceName {
	id, _ := ParseServiceNameString(input)
	return ServiceName{Name: id}
}

// Less implements sort.Interface.
func (s ServiceList) Less(i, j int) bool {
	a, b := s[i], s[j]
	return a.Name < b.Name
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

type EnterpriseServiceUsage struct{}
