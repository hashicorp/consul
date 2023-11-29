package hcp

import (
	"github.com/hashicorp/consul/internal/hcp/internal/types"
	"github.com/hashicorp/consul/internal/resource"
)

// RegisterTypes adds all resource types within the "hcp" API group
// to the given type registry
func RegisterTypes(r resource.Registry) {
	types.Register(r)
}
