// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package connect

import (
	"fmt"
	"net/url"
)

// SpiffeIDWorkloadIdentity is the structure to represent the SPIFFE ID for a workload.
type SpiffeIDWorkloadIdentity struct {
	TrustDomain      string
	Partition        string
	Namespace        string
	WorkloadIdentity string
}

// URI returns the *url.URL for this SPIFFE ID.
func (id SpiffeIDWorkloadIdentity) URI() *url.URL {
	var result url.URL
	result.Scheme = "spiffe"
	result.Host = id.TrustDomain
	result.Path = id.uriPath()
	return &result
}

func (id SpiffeIDWorkloadIdentity) uriPath() string {
	// Although CE has no support for partitions, it still needs to be able to
	// handle exportedPartition from peered Consul Enterprise clusters in order
	// to generate the correct SpiffeID.
	// We intentionally avoid using pbpartition.DefaultName here to be CE friendly.
	path := fmt.Sprintf("/ap/%s/ns/%s/identity/%s",
		id.Partition,
		id.Namespace,
		id.WorkloadIdentity,
	)

	return path
}
