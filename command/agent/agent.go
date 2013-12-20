package agent

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
}

// Create is used to create a new Agent. Returns
// the agent or potentially an error.
func Create(config *Config) (*Agent, error) {
	agent := &Agent{
		config: config,
	}
	return agent, nil
}

// Leave prepares the agent for a graceful shutdown
func (a *Agent) Leave() error {
	return nil
}

// Shutdown is used to hard stop the agent. Should be preceeded
// by a call to Leave to do it gracefully.
func (a *Agent) Shutdown() error {
	return nil
}
