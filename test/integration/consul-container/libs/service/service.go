// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package service

import (
	"context"

	"github.com/hashicorp/consul/api"
)

// Service represents a process that will be registered with the
// Consul catalog, including Consul components such as sidecars and gateways
type Service interface {
	Exec(ctx context.Context, cmd []string) (string, error)
	// Export a service to the peering cluster
	Export(partition, peer string, client *api.Client) error
	GetAddr() (string, int)
	GetAddrs() (string, []int)
	GetPort(port int) (int, error)
	// GetAdminAddr returns the external admin address
	GetAdminAddr() (string, int)
	GetLogs() (string, error)
	GetName() string
	GetServiceName() string
	Start() (err error)
	Stop() (err error)
	Terminate() error
	Restart() error
	GetStatus() (string, error)
}
