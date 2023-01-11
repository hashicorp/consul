package service

import "github.com/hashicorp/consul/api"

// Service represents a process that will be registered with the
// Consul catalog, including Consul components such as sidecars and gateways
type Service interface {
	Terminate() error
	GetName() string
	GetAddr() (string, int)
	Start() (err error)
	// Export a service to the peering cluster
	Export(partition, peer string, client *api.Client) error
	GetServiceName() string
}
