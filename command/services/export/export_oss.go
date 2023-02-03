//go:build !consulent
// +build !consulent

package export

import (
	"errors"
)

func (c *cmd) validateFlags() error {
	if c.serviceName == "" {
		return errors.New("Missing the required -name flag")
	}

	if c.peerNames == "" {
		return errors.New("Missing the required -consumer-peers flag")
	}

	return nil
}

func (c *cmd) getPartitionNames() ([]string, error) {
	return []string{}, nil
}

const (
	synopsis = "Export a service"
	help     = `
Usage: consul services export [options] -name <service name> -consumer-peers <other cluster name>

  Export a service. The peers provided will be used locally by
  this cluster to refer to the other cluster where the services will be exported. 

  Example:

  $ consul services export -name=web -consumer-peers=other-cluster
`
)