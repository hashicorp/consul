package structs

import "github.com/hashicorp/consul/acl"

// HTTPRouteConfigEntry stub
type HTTPRouteConfigEntry struct {
	Kind string
	Name string

	Meta               map[string]string `json:",omitempty"`
	acl.EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
	RaftIndex
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
	if e == nil {
		return &RaftIndex{}
	}
	return &e.RaftIndex
}

// TCPRouteConfigEntry stub
type TCPRouteConfigEntry struct {
	Kind string
	Name string

	Meta               map[string]string `json:",omitempty"`
	acl.EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
	RaftIndex
}

func (e *TCPRouteConfigEntry) GetKind() string {
	return TCPRoute
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
	if e == nil {
		return &RaftIndex{}
	}
	return &e.RaftIndex
}

func (e *TCPRouteConfigEntry) GetEnterpriseMeta() *acl.EnterpriseMeta {
	return nil
}
