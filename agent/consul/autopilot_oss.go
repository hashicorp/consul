// +build !ent

package consul

import "github.com/hashicorp/consul/agent/consul/autopilot"

func (s *Server) initAutopilot(config *Config) {
	apDelegate := &AutopilotDelegate{s}
	s.autopilot = autopilot.NewAutopilot(s.logger, apDelegate, config.AutopilotInterval, config.ServerHealthInterval)
}
