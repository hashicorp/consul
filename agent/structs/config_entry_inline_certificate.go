package structs

import "github.com/hashicorp/consul/acl"

// InlineCertificateConfigEntry TODO
type InlineCertificateConfigEntry struct {
	// Kind of config entry. This will be set to structs.InlineCertificateConfig
	Kind string

	// Name is used to match the config entry with its associated inline certificate.
	Name string

	acl.EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
	RaftIndex
}

func (i *InlineCertificateConfigEntry) GetKind() string {
	return InlineCertificate
}

func (i *InlineCertificateConfigEntry) GetName() string {
	return i.Name
}

func (i *InlineCertificateConfigEntry) Normalize() error {
	return nil
}

func (i *InlineCertificateConfigEntry) Validate() error {
	return nil
}

func (i *InlineCertificateConfigEntry) CanRead(authorizer acl.Authorizer) error {
	return nil
}

func (i *InlineCertificateConfigEntry) CanWrite(authorizer acl.Authorizer) error {
	return nil
}

func (i *InlineCertificateConfigEntry) GetMeta() map[string]string {
	return nil
}

func (i *InlineCertificateConfigEntry) GetEnterpriseMeta() *acl.EnterpriseMeta {
	return nil
}

func (i *InlineCertificateConfigEntry) GetRaftIndex() *RaftIndex {

	if i == nil {
		return &RaftIndex{}
	}
	return &i.RaftIndex
}
