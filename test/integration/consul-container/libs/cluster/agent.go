// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package cluster

import (
	"context"
	"io"

	"github.com/testcontainers/testcontainers-go"
	"google.golang.org/grpc"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
)

// Agent represent a Consul agent abstraction
type Agent interface {
	GetIP() string
	GetClient() *api.Client
	NewClient(string, bool) (*api.Client, error)
	GetName() string
	GetAgentName() string
	GetPartition() string
	GetPod() testcontainers.Container
	Logs(context.Context) (io.ReadCloser, error)
	ClaimAdminPort() (int, error)
	GetConfig() Config
	GetInfo() AgentInfo
	GetDatacenter() string
	GetNetwork() string
	IsServer() bool
	RegisterTermination(func() error)
	Terminate() error
	TerminateAndRetainPod(bool) error
	Upgrade(ctx context.Context, config Config) error
	Exec(ctx context.Context, cmd []string) (string, error)
	DataDir() string
	GetGRPCConn() *grpc.ClientConn
}

// Config is a set of configurations required to create a Agent
//
// Constructed by (Builder).ToAgentConfig()
type Config struct {
	// NodeName is set for the consul agent name and container name
	// Equivalent to the -node command-line flag.
	// If empty, a randam name will be generated
	NodeName string
	// NodeID is used to configure node_id in agent config file
	// Equivalent to the -node-id command-line flag.
	// If empty, a randam name will be generated
	NodeID string

	// ExternalDataDir is data directory to copy consul data from, if set.
	// This directory contains subdirectories like raft, serf, services
	ExternalDataDir string

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

	ACLEnabled bool
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
	DebugURI      string
}
