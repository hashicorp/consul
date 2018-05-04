package structs

import (
	"github.com/mitchellh/go-testing-interface"
)

// TestServiceDefinition returns a ServiceDefinition for a typical service.
func TestServiceDefinition(t testing.T) *ServiceDefinition {
	return &ServiceDefinition{
		Name: "db",
	}
}

// TestServiceDefinitionProxy returns a ServiceDefinition for a proxy.
func TestServiceDefinitionProxy(t testing.T) *ServiceDefinition {
	return &ServiceDefinition{
		Kind:             ServiceKindConnectProxy,
		ProxyDestination: "db",
	}
}
