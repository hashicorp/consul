package agent

import (
	"fmt"
	"github.com/hashicorp/consul/consul"
)

/*
 The agent is the long running process that is run on every machine.
 It exposes an RPC interface that is used by the CLI to control the
 agent. The agent runs the query interfaces like HTTP, DNS, and RPC.
 However, it can run in either a client, or server mode. In server
 mode, it runs a full Consul server. In client-only mode, it only forwards
 requests to other Consul servers.
*/
type Agent struct {
	config *Config

	// We have one of a client or a server, depending
	// on our configuration
	server *consul.Server
	client *consul.Client
}

// Create is used to create a new Agent. Returns
// the agent or potentially an error.
func Create(config *Config) (*Agent, error) {
	agent := &Agent{
		config: config,
	}

	// Setup either the client or the server
	var err error
	if config.Server {
		err = agent.setupServer()
	} else {
		err = agent.setupClient()
	}
	if err != nil {
		return nil, err
	}

	return agent, nil
}

// consulConfig is used to return a consul configuration
func (a *Agent) consulConfig() *consul.Config {
	// Start with the provided config or default config
	var base *consul.Config
	if a.config.ConsulConfig != nil {
		base = a.config.ConsulConfig
	} else {
		base = consul.DefaultConfig()
	}

	// Override with our config
	if a.config.Datacenter != "" {
		base.Datacenter = a.config.Datacenter
	}
	if a.config.DataDir != "" {
		base.DataDir = a.config.DataDir
	}
	if a.config.NodeName != "" {
		base.NodeName = a.config.NodeName
	}
	if a.config.SerfBindAddr != "" {
		base.SerfLANConfig.MemberlistConfig.BindAddr = a.config.SerfBindAddr
		base.SerfWANConfig.MemberlistConfig.BindAddr = a.config.SerfBindAddr
	}
	if a.config.SerfLanPort != 0 {
		base.SerfLANConfig.MemberlistConfig.Port = a.config.SerfLanPort
	}
	if a.config.SerfWanPort != 0 {
		base.SerfWANConfig.MemberlistConfig.Port = a.config.SerfWanPort
	}
	if a.config.ServerRPCAddr != "" {
		base.RPCAddr = a.config.ServerRPCAddr
	}

	return base
}

// setupServer is used to initialize the Consul server
func (a *Agent) setupServer() error {
	server, err := consul.NewServer(a.consulConfig())
	if err != nil {
		return fmt.Errorf("Failed to start Consul server: %v", err)
	}
	a.server = server
	return nil
}

// setupClient is used to initialize the Consul client
func (a *Agent) setupClient() error {
	client, err := consul.NewClient(a.consulConfig())
	if err != nil {
		return fmt.Errorf("Failed to start Consul client: %v", err)
	}
	a.client = client
	return nil
}

// RPC is used to make an RPC call to the Consul servers
// This allows the agent to implement the Consul.Interface
func (a *Agent) RPC(method string, args interface{}, reply interface{}) error {
	if a.server != nil {
		return a.server.RPC(method, args, reply)
	}
	return a.client.RPC(method, args, reply)
}

// Leave prepares the agent for a graceful shutdown
func (a *Agent) Leave() error {
	if a.server != nil {
		return a.server.Leave()
	} else {
		return a.client.Leave()
	}
}

// Shutdown is used to hard stop the agent. Should be preceeded
// by a call to Leave to do it gracefully.
func (a *Agent) Shutdown() error {
	if a.server != nil {
		return a.server.Shutdown()
	} else {
		return a.client.Shutdown()
	}
}
