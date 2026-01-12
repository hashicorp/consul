// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

type ImportedService struct {
	// Service is the name of the service which is imported.
	Service string

	// Partition of the service
	Partition string `json:",omitempty"`

	// Namespace of the service
	Namespace string `json:",omitempty"`

	// SourcePeer is the peer from which the service is imported.
	SourcePeer string `json:",omitempty"`

	// SourcePartition is the partition from which the service is imported.
	SourcePartition string `json:",omitempty"`
}

func (c *Client) ImportedServices(q *QueryOptions) ([]ImportedService, *QueryMeta, error) {

	r := c.newRequest("GET", "/v1/imported-services")
	r.setQueryOptions(q)
	rtt, resp, err := c.doRequest(r)
	if err != nil {
		return nil, nil, err
	}
	defer closeResponseBody(resp)
	if err := requireOK(resp); err != nil {
		return nil, nil, err
	}

	qm := &QueryMeta{}
	parseQueryMeta(resp, qm)
	qm.RequestTime = rtt

	var impSvcs []ImportedService

	if err := decodeBody(resp, &impSvcs); err != nil {
		return nil, nil, err
	}

	return impSvcs, qm, nil
}
