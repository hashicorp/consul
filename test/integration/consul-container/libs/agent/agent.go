package agent

import (
	"context"

	"github.com/hashicorp/consul/api"
)

// Agent represent a Consul agent abstraction
type Agent interface {
	GetAddr() (string, int)
	GetClient() *api.Client
	GetName() string
	GetConfig() Config
	GetDatacenter() string
	IsServer() bool
	RegisterTermination(func() error)
	Terminate() error
	Upgrade(ctx context.Context, config Config, index int) error
}

// Config is a set of configurations required to create a Agent
type Config struct {
	JSON    string
	Certs   map[string]string
	Image   string
	Version string
	Cmd     []string
}
