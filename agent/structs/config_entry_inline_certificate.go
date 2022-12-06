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
	//TODO implement me
	panic("implement me")
}

func (i *InlineCertificateConfigEntry) Validate() error {
	//TODO implement me
	panic("implement me")
}

func (i *InlineCertificateConfigEntry) CanRead(authorizer acl.Authorizer) error {
	//TODO implement me
	panic("implement me")
}

func (i *InlineCertificateConfigEntry) CanWrite(authorizer acl.Authorizer) error {
	//TODO implement me
	panic("implement me")
}

func (i *InlineCertificateConfigEntry) GetMeta() map[string]string {
	//TODO implement me
	panic("implement me")
}

func (i *InlineCertificateConfigEntry) GetEnterpriseMeta() *acl.EnterpriseMeta {
	//TODO implement me
	panic("implement me")
}

func (i *InlineCertificateConfigEntry) GetRaftIndex() *RaftIndex {
	if i == nil {
		return &RaftIndex{}
	}
	return &i.RaftIndex
}
