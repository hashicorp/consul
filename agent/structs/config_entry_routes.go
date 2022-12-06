package structs

import "github.com/hashicorp/consul/acl"

// TCPRouteConfigEntry stub
type TCPRouteConfigEntry struct {
	Name string
	Kind string
	Meta map[string]string `json:",omitempty"`
}

func (e *TCPRouteConfigEntry) GetKind() string {
	return APIGateway
}

func (e *TCPRouteConfigEntry) GetName() string {
	if e == nil {
		return ""
	}
	return e.Name
}

func (e *TCPRouteConfigEntry) GetMeta() map[string]string {
	if e == nil {
		return nil
	}
	return e.Meta
}

func (e *TCPRouteConfigEntry) Normalize() error {
	return nil
}

func (e *TCPRouteConfigEntry) Validate() error {
	return nil
}

func (e *TCPRouteConfigEntry) CanRead(authz acl.Authorizer) error {
	return nil
}

func (e *TCPRouteConfigEntry) CanWrite(authz acl.Authorizer) error {
	return nil
}

func (e *TCPRouteConfigEntry) GetRaftIndex() *RaftIndex {
	return nil
}

func (e *TCPRouteConfigEntry) GetEnterpriseMeta() *acl.EnterpriseMeta {
	return nil
}

func (e *TCPRouteConfigEntry) Warnings() []string {
	return []string{}
}
