// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package builder

import (
	"fmt"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func DestinationSNI(serviceRef *pbresource.Reference, datacenter, trustDomain string) string {
	return connect.ServiceSNI(serviceRef.Name,
		"",
		serviceRef.Tenancy.Namespace,
		serviceRef.Tenancy.Partition,
		datacenter,
		trustDomain)
}

func DestinationStatPrefix(serviceRef *pbresource.Reference, portName, datacenter string) string {
	return fmt.Sprintf("upstream.%s.%s.%s.%s.%s",
		portName,
		serviceRef.Name,
		serviceRef.Tenancy.Namespace,
		serviceRef.Tenancy.Partition,
		datacenter)
}

func DestinationListenerName(name, portName string, address string, port uint32) string {
	if port != 0 {
		return fmt.Sprintf("%s:%s:%s:%d", name, portName, address, port)
	}

	return fmt.Sprintf("%s:%s:%s", name, portName, address)
}
