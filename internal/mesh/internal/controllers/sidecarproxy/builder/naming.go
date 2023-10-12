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

func DestinationListenerName(destinationRef *pbresource.Reference, datacenter, portName string, address string, port uint32) string {
	if port != 0 {
		return fmt.Sprintf("%s:%s:%s:%d", DestinationResourceID(destinationRef, datacenter), portName, address, port)
	}

	return fmt.Sprintf("%s:%s:%s", DestinationResourceID(destinationRef, datacenter), portName, address)
}

// XDSResourceID returns a string representation that uniquely identifies the
// upstream in a canonical but human readable way.
func DestinationResourceID(destinationRef *pbresource.Reference, datacenter string) string {
	tenancyPrefix := fmt.Sprintf("%s/%s/%s/%s", destinationRef.Tenancy.Partition,
		destinationRef.Tenancy.PeerName, destinationRef.Tenancy.Namespace,
		datacenter)
	return fmt.Sprintf("%s/%s", tenancyPrefix, destinationRef.Name)
}
