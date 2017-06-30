package structs

import (
	"github.com/hashicorp/consul/types"
)

// These are used to manage the built-in "serfHealth" check that's attached
// to every node in the catalog.
const (
	SerfCheckID           types.CheckID = "serfHealth"
	SerfCheckName                       = "Serf Health Status"
	SerfCheckAliveOutput                = "Agent alive and reachable"
	SerfCheckFailedOutput               = "Agent not live or unreachable"
)

// These are used to manage the "consul" service that's attached to every Consul
// server node in the catalog.
const (
	ConsulServiceID   = "consul"
	ConsulServiceName = "consul"
)
