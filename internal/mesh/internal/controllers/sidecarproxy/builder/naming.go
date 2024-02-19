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

func DestinationListenerName(destinationRef *pbresource.Reference, portName string, address string, port uint32) string {
	name := fmt.Sprintf("%s:%s", DestinationResourceID(destinationRef, portName), address)
	if port != 0 {
		return fmt.Sprintf("%s:%d", name, port)
	}

	return name
}

// DestinationResourceID returns a string representation that uniquely identifies the
// upstream in a canonical but human readable way.
func DestinationResourceID(destinationRef *pbresource.Reference, port string) string {
	return fmt.Sprintf("%s/%s/%s:%s",
		destinationRef.Tenancy.Partition,
		destinationRef.Tenancy.Namespace,
		destinationRef.Name,
		port,
	)
}
