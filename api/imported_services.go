// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"encoding/json"
	"io"
)

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

type importedServicesResponse struct {
	Partition        string            `json:"Partition"`
	ImportedServices []ImportedService `json:"ImportedServices"`
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

	// Read the body so we can try different decode formats
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}

	// Try to decode as wrapped response (ENT format)
	// ENT returns: {Partition: "...", ImportedServices: [...]}
	var result importedServicesResponse
	if err := json.Unmarshal(body, &result); err == nil && result.Partition != "" {
		return result.ImportedServices, qm, nil
	}

	// If that fails, try to decode as raw array (CE format)
	// CE returns: [...]
	var services []ImportedService
	if err := json.Unmarshal(body, &services); err != nil {
		return nil, nil, err
	}

	return services, qm, nil
}
