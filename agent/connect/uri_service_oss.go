//go:build !consulent
// +build !consulent

package connect

import (
	"fmt"

	"github.com/hashicorp/consul/acl"
)

// GetEnterpriseMeta will synthesize an EnterpriseMeta struct from the SpiffeIDService.
// in OSS this just returns an empty (but never nil) struct pointer
func (id SpiffeIDService) GetEnterpriseMeta() *acl.EnterpriseMeta {
	return &acl.EnterpriseMeta{}
}

func (id SpiffeIDService) uriPath() string {
	return fmt.Sprintf("/ns/%s/dc/%s/svc/%s",
		id.NamespaceOrDefault(),
		id.Datacenter,
		id.Service,
	)
}
