//go:build !consulent
// +build !consulent

package consul

import (
	"fmt"
)

func (b *peeringBackend) enterpriseCheckPartitions(partition string) error {
	if partition != "" {
		return fmt.Errorf("Partitions are a Consul Enterprise feature")
	}
	return nil
}
