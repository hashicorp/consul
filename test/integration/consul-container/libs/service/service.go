package service

import (
	"context"
	"github.com/hashicorp/consul/api"
	"io"
)

// Service represents a process that will be registered with the
// Consul catalog, including Consul components such as sidecars and gateways
type Service interface {
	// Export a service to the peering cluster
	Exec(ctx context.Context, cmd []string) (int, io.Reader, error)
	Export(partition, peer string, client *api.Client) error
	GetAddr() (string, int)
	GetLogs() (string, error)
	GetName() string
	GetServiceName() string
	Start() (err error)
	Terminate() error
	Restart() error
	GetStatus() (string, error)
}
