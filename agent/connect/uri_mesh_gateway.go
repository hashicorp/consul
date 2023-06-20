// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package connect

import (
	"net/url"

	"github.com/hashicorp/consul/acl"
)

type SpiffeIDMeshGateway struct {
	Host       string
	Partition  string
	Datacenter string
}

func (id SpiffeIDMeshGateway) MatchesPartition(partition string) bool {
	return id.PartitionOrDefault() == acl.PartitionOrDefault(partition)
}

func (id SpiffeIDMeshGateway) PartitionOrDefault() string {
	return acl.PartitionOrDefault(id.Partition)
}

// URI returns the *url.URL for this SPIFFE ID.
func (id SpiffeIDMeshGateway) URI() *url.URL {
	var result url.URL
	result.Scheme = "spiffe"
	result.Host = id.Host
	result.Path = id.uriPath()
	return &result
}
