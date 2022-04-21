package node

import "github.com/hashicorp/consul/api"

// ConsulNode represent a Consul node abstraction
type ConsulNode interface {
	Terminate() error
	GetClient() *api.Client
	GetAddr() (string, int)
}

// Config is a set of configurations required to create a ConsulNode
type Config struct {
	HCL     string
	Version string
	Cmd     []string
}
