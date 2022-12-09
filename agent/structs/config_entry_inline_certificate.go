package structs

import "github.com/hashicorp/consul/acl"

// InlineCertificateConfigEntry TODO
type InlineCertificateConfigEntry struct {
	// Kind of config entry. This will be set to structs.InlineCertificateConfig
	Kind string

	// Name is used to match the config entry with its associated inline certificate.
	Name string

	Meta               map[string]string `json:",omitempty"`
	acl.EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
	RaftIndex
}

func (e *InlineCertificateConfigEntry) GetKind() string {
	return InlineCertificate
}

func (e *InlineCertificateConfigEntry) GetName() string {
	return e.Name
}

func (e *InlineCertificateConfigEntry) Normalize() error {
	return nil
}

func (e *InlineCertificateConfigEntry) Validate() error {
	return nil
}

func (e *InlineCertificateConfigEntry) CanRead(authz acl.Authorizer) error {
	var authzContext acl.AuthorizerContext
	e.FillAuthzContext(&authzContext)
	return authz.ToAllowAuthorizer().MeshReadAllowed(&authzContext)
}

func (e *InlineCertificateConfigEntry) CanWrite(authz acl.Authorizer) error {
	var authzContext acl.AuthorizerContext
	e.FillAuthzContext(&authzContext)
	return authz.ToAllowAuthorizer().MeshWriteAllowed(&authzContext)
}

func (e *InlineCertificateConfigEntry) GetMeta() map[string]string {
	if e == nil {
		return nil
	}
	return e.Meta
}

func (e *InlineCertificateConfigEntry) GetEnterpriseMeta() *acl.EnterpriseMeta {
	if e == nil {
		return nil
	}
	return &e.EnterpriseMeta
}

func (e *InlineCertificateConfigEntry) GetRaftIndex() *RaftIndex {
	if e == nil {
		return &RaftIndex{}
	}
	return &e.RaftIndex
}
