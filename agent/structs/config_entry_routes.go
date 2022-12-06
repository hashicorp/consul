package structs

import "github.com/hashicorp/consul/acl"

// HTTPRouteConfigEntry stub
type HTTPRouteConfigEntry struct {
	Name string
	Kind string
	Meta map[string]string `json:",omitempty"`
}

func (e *HTTPRouteConfigEntry) GetKind() string {
	return HTTPRoute
}

func (e *HTTPRouteConfigEntry) GetName() string {
	if e == nil {
		return ""
	}
	return e.Name
}

func (e *HTTPRouteConfigEntry) Normalize() error {
	return nil
}

func (e *HTTPRouteConfigEntry) Validate() error {
	return nil
}

func (e *HTTPRouteConfigEntry) CanRead(authz acl.Authorizer) error {
	return nil
}

func (e *HTTPRouteConfigEntry) CanWrite(authz acl.Authorizer) error {
	return nil
}

func (e *HTTPRouteConfigEntry) GetMeta() map[string]string {
	return nil
}

func (e *HTTPRouteConfigEntry) GetEnterpriseMeta() *acl.EnterpriseMeta {
	return nil
}

func (e *HTTPRouteConfigEntry) GetRaftIndex() *RaftIndex {
	return nil
}

func (e *HTTPRouteConfigEntry) Warnings() []string {
	return []string{}
}

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
