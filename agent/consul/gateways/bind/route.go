package bind

import "github.com/hashicorp/consul/agent/structs"

// BoundRouter indicates a route that has parent gateways which
// can be accessed by calling the GetParents associated function.
type BoundRouter interface {
	GetParents() []structs.ResourceReference
}
