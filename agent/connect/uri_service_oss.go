// +build !consulent

package connect

import (
	"github.com/hashicorp/consul/agent/structs"
)

// GetEnterpriseMeta will synthesize an EnterpriseMeta struct from the SpiffeIDService.
// in OSS this just returns an empty (but never nil) struct pointer
func (id *SpiffeIDService) GetEnterpriseMeta() *structs.EnterpriseMeta {
	return &structs.EnterpriseMeta{}
}
