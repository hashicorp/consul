// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package netutil

import (
	"fmt"
	"net"
	"net/netip"

	"github.com/hashicorp/consul/api"
)

// GetAgentConfigFunc is the function type for getting agent config
var GetAgentConfigFunc = GetAgentConfig

var GetAgentBindAddrFunc = GetAgentBindAddr

var cachedBindAddr net.IP

func GetMockGetAgentBindAddrFunc(ip string) func(config *api.Config, cached bool) (net.IP, error) {
	return func(config *api.Config, cached bool) (net.IP, error) {
		ip := net.ParseIP(ip)
		if ip == nil {
			return nil, fmt.Errorf("unable to parse bind address")
		}
		return ip, nil
	}
}

// GetAgentConfig retrieves the agent's configuration using the local Consul agent's API.
func GetAgentConfig(config *api.Config) (map[string]map[string]interface{}, error) {
	if config == nil {
		config = api.DefaultConfig()
	}
	client, err := api.NewClient(config)
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
func GetAgentBindAddr(config *api.Config, cached bool) (net.IP, error) {
	if cachedBindAddr != nil && cached {
		return cachedBindAddr, nil
	}
	agentConfig, err := GetAgentConfigFunc(config)
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
	cachedBindAddr = ip.AsSlice()
	return cachedBindAddr, nil
}

// IsDualStack checks if the agent is configured to use both IPv4 and IPv6 addresses.
// It returns true if the agent is running in dual-stack mode, false otherwise.
// An error is returned if the agent's bind address cannot be determined.
func IsDualStack(config *api.Config, cached bool) (bool, error) {
	bindIP, err := GetAgentBindAddrFunc(config, cached)
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
