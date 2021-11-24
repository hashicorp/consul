//go:build !consulent
// +build !consulent

package connect

import (
	"fmt"

	"github.com/hashicorp/consul/agent/structs"
)

// GetEnterpriseMeta will synthesize an EnterpriseMeta struct from the SpiffeIDAgent.
// in OSS this just returns an empty (but never nil) struct pointer
func (id SpiffeIDAgent) GetEnterpriseMeta() *structs.EnterpriseMeta {
	return &structs.EnterpriseMeta{}
}

func (id SpiffeIDAgent) uriPath() string {
	return fmt.Sprintf("/agent/client/dc/%s/id/%s", id.Datacenter, id.Agent)
}
