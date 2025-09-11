// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package netutil

import (
	"net"
	"net/netip"

	"github.com/hashicorp/consul/api"
)

// GetAgentConfig retrieves the agent's configuration using the local Consul agent's API.
func GetAgentConfig() (map[string]map[string]interface{}, error) {
	client, err := api.NewClient(api.DefaultConfig())
	if err != nil {
		return nil, err
	}

	self, err := client.Agent().Self()
	if err != nil {
		return nil, err
	}

	return self, nil
}

// GetAgentBindAddr retrieves the bind address from the agent's configuration.
func GetAgentBindAddr() (net.IP, error) {
	agentConfig, err := GetAgentConfig()
	if err != nil {
		return nil, err
	}

	bindAddr, ok := agentConfig["Config"]["BindAddr"].(string)
	if !ok || bindAddr == "" {
		return nil, nil
	}

	ip, err := netip.ParseAddr(bindAddr)
	if err != nil {
		return nil, err
	}

	return ip.AsSlice(), nil
}

// IsDualStack checks if the agent is configured to use both IPv4 and IPv6 addresses.
// It returns true if the agent is running in dual-stack mode, false otherwise.
// An error is returned if the agent's bind address cannot be determined.
func IsDualStack() (bool, error) {
	bindIP, err := GetAgentBindAddr()
	if err != nil {
		return false, err
	}

	// If no bind address is set, assume dual-stack is not enabled
	if bindIP == nil {
		return false, nil
	}

	// Check if the bind address is an IPv4-mapped IPv6 address
	if bindIP.To4() != nil {
		// IPv4 address
		return false, nil
	}

	// For IPv6, check if it's a dual-stack address
	return bindIP.To16() != nil && bindIP.To4() == nil, nil
}
