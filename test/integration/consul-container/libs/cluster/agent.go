package cluster

import (
	"context"

	"github.com/testcontainers/testcontainers-go"

	"github.com/hashicorp/consul/api"

	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
)

// Agent represent a Consul agent abstraction
type Agent interface {
	GetIP() string
	GetClient() *api.Client
	GetName() string
	GetPod() testcontainers.Container
	ClaimAdminPort() (int, error)
	GetConfig() Config
	GetInfo() AgentInfo
	GetDatacenter() string
	IsServer() bool
	RegisterTermination(func() error)
	Terminate() error
	TerminateAndRetainPod() error
	Upgrade(ctx context.Context, config Config) error
	Exec(ctx context.Context, cmd []string) (int, error)
	DataDir() string
}

// Config is a set of configurations required to create a Agent
//
// Constructed by (Builder).ToAgentConfig()
type Config struct {
	ScratchDir    string
	CertVolume    string
	CACert        string
	JSON          string
	ConfigBuilder *ConfigBuilder
	Image         string
	Version       string
	Cmd           []string
	LogConsumer   testcontainers.LogConsumer

	// service defaults
	UseAPIWithTLS  bool // TODO
	UseGRPCWithTLS bool
}

func (c *Config) DockerImage() string {
	return utils.DockerImage(c.Image, c.Version)
}

// Clone copies everything. It is the caller's job to replace fields that
// should be unique.
func (c Config) Clone() Config {
	c2 := c
	if c.Cmd != nil {
		c2.Cmd = make([]string, len(c.Cmd))
		for i, v := range c.Cmd {
			c2.Cmd[i] = v
		}
	}
	return c2
}

// TODO: refactor away
type AgentInfo struct {
	CACertFile    string
	UseTLSForAPI  bool
	UseTLSForGRPC bool
}
