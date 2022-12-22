package cluster

import (
	"context"

	"github.com/hashicorp/consul/api"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
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
	RegisterConnectSidecar(*libservice.ConnectContainer)
	Upgrade(ctx context.Context, config Config) error
	Exec(ctx context.Context, cmd []string) (int, error)
	DataDir() string
}

// Config is a set of configurations required to create a Agent
type Config struct {
	JSON    string
	Certs   map[string]string
	Image   string
	Version string
	Cmd     []string
}
