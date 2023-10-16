// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"github.com/mitchellh/go-testing-interface"
)

// TestServiceDefinition returns a ServiceDefinition for a typical service.
func TestServiceDefinition(t testing.T) *ServiceDefinition {
	return &ServiceDefinition{
		Name: "db",
		Port: 1234,
	}
}

// TestServiceDefinitionProxy returns a ServiceDefinition for a proxy.
func TestServiceDefinitionProxy(t testing.T) *ServiceDefinition {
	return &ServiceDefinition{
		Kind: ServiceKindConnectProxy,
		Name: "foo-proxy",
		Port: 1234,
		Proxy: &ConnectProxyConfig{
			DestinationServiceName: "db",
		},
	}
}
