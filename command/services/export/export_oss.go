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
