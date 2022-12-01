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
