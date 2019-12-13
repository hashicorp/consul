// +build !consulent

package consul

import "github.com/hashicorp/consul/agent/consul/autopilot"

func (s *Server) initAutopilot(config *Config) {
	apDelegate := &AutopilotDelegate{s}
	s.autopilot = autopilot.NewAutopilot(s.logger, s.logger2, apDelegate, config.AutopilotInterval, config.ServerHealthInterval)
}
