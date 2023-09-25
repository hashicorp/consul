// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package pbpeerstream

import (
	"fmt"

	"github.com/hashicorp/consul/agent/structs"
	pbservice "github.com/hashicorp/consul/proto/private/pbservice"
)

// CheckServiceNodesToStruct converts the contained CheckServiceNodes to their structs equivalent.
func (s *ExportedService) CheckServiceNodesToStruct() ([]structs.CheckServiceNode, error) {
	if s == nil {
		return nil, nil
	}

	resp := make([]structs.CheckServiceNode, 0, len(s.Nodes))
	for _, pb := range s.Nodes {
		instance, err := pbservice.CheckServiceNodeToStructs(pb)
		if err != nil {
			return resp, fmt.Errorf("failed to convert instance: %w", err)
		}
		resp = append(resp, *instance)
	}
	return resp, nil
}

func ExportedServiceListFromStruct(e *structs.ExportedServiceList) *ExportedServiceList {
	services := make([]string, 0, len(e.Services))

	for _, s := range e.Services {
		services = append(services, s.String())
	}

	return &ExportedServiceList{
		Services: services,
	}
}
