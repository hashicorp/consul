package types

import "github.com/hashicorp/consul/internal/resource"

func Register(r resource.Registry) {
	RegisterHCCLink(r)
}
