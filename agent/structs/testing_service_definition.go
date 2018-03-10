package structs

import (
	"github.com/mitchellh/go-testing-interface"
)

// TestServiceDefinitionProxy returns a ServiceDefinition for a proxy.
func TestServiceDefinitionProxy(t testing.T) *ServiceDefinition {
	return &ServiceDefinition{
		Kind:             ServiceKindConnectProxy,
		ProxyDestination: "db",
	}
}
