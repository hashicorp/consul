//go:build !consulent
// +build !consulent

package consul

import (
	"fmt"
	"strings"
)

func (b *peeringBackend) enterpriseCheckPartitions(partition string) error {
	if partition == "" || strings.EqualFold(partition, "default") {
		return nil
	}
	return fmt.Errorf("Partitions are a Consul Enterprise feature")
}

func (b *peeringBackend) enterpriseCheckNamespaces(namespace string) error {
	if namespace == "" || strings.EqualFold(namespace, "default") {
		return nil
	}
	return fmt.Errorf("Namespaces are a Consul Enterprise feature")
}
