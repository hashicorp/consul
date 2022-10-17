package node

import (
	"context"

	"github.com/hashicorp/consul/api"
)

// Node represent a Consul node abstraction
type Node interface {
	GetAddr() (string, int)
	GetClient() *api.Client
	GetName() string
	GetConfig() Config
	RegisterTermination(func() error)
	Terminate() error
	Upgrade(ctx context.Context, config Config) error
}

// Config is a set of configurations required to create a Node
type Config struct {
	HCL     string
	Image   string
	Version string
	Cmd     []string
}
