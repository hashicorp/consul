package node

import (
	"context"

	"github.com/hashicorp/consul/api"
)

// Node represent a Consul node abstraction
type Node interface {
	Terminate() error
	GetClient() *api.Client
	GetAddr() (string, int)
	GetConfig() Config
	Upgrade(ctx context.Context, config Config) error
}

// Config is a set of configurations required to create a Node
type Config struct {
	HCL     string
	Version string
	Cmd     []string
}
