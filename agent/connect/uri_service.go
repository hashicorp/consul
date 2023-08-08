// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package connect

import (
	"fmt"
	"net/url"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// SpiffeIDService is the structure to represent the SPIFFE ID for a service.
type SpiffeIDService struct {
	Host       string
	Partition  string
	Namespace  string
	Datacenter string
	Service    string
}

func (id SpiffeIDService) NamespaceOrDefault() string {
	return acl.NamespaceOrDefault(id.Namespace)
}

func (id SpiffeIDService) MatchesPartition(partition string) bool {
	return id.PartitionOrDefault() == acl.PartitionOrDefault(partition)
}

// URI returns the *url.URL for this SPIFFE ID.
func (id SpiffeIDService) URI() *url.URL {
	var result url.URL
	result.Scheme = "spiffe"
	result.Host = id.Host
	result.Path = id.uriPath()
	return &result
}

func (id SpiffeIDService) uriPath() string {
	path := fmt.Sprintf("/ns/%s/dc/%s/svc/%s",
		id.NamespaceOrDefault(),
		id.Datacenter,
		id.Service,
	)

	// Although OSS has no support for partitions, it still needs to be able to
	// handle exportedPartition from peered Consul Enterprise clusters in order
	// to generate the correct SpiffeID.
	// We intentionally avoid using pbpartition.DefaultName here to be OSS friendly.
	if ap := id.PartitionOrDefault(); ap != "" && ap != "default" {
		return "/ap/" + ap + path
	}
	return path
}

// SpiffeIDWorkloadIdentity is the structure to represent the SPIFFE ID for a workload identity.
type SpiffeIDWorkloadIdentity struct {
	Host      string
	Partition string
	Namespace string
	Identity  string
}

func (id SpiffeIDWorkloadIdentity) URI() *url.URL {
	var result url.URL
	result.Scheme = "spiffe"
	result.Host = id.Host
	result.Path = fmt.Sprintf("/ap/%s/ns/%s/identity/%s",
		id.Partition,
		id.Namespace,
		id.Identity,
	)
	return &result
}

// SpiffeIDFromIdentityRef creates the SPIFFE ID from a workload identity.
// TODO (ishustava): make sure ref type is workload identity.
func SpiffeIDFromIdentityRef(trustDomain string, ref *pbresource.Reference) string {
	return SpiffeIDWorkloadIdentity{
		Host:      trustDomain,
		Partition: ref.Tenancy.Partition,
		Namespace: ref.Tenancy.Namespace,
		Identity:  ref.Name,
	}.URI().String()
}
