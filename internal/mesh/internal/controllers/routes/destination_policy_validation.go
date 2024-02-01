// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package routes

import (
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/routes/loader"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// ValidateDestinationPolicyPorts examines the ported configs of the policies provided
// and issues status conditions if anything is unacceptable.
func ValidateDestinationPolicyPorts(related *loader.RelatedResources, pending PendingStatuses) {
	for rk, destPolicy := range related.DestinationPolicies {
		conditions := computeNewDestPolicyPortConditions(related, rk, destPolicy)
		pending.AddConditions(rk, destPolicy.Resource, conditions)
	}
}

func computeNewDestPolicyPortConditions(
	related serviceGetter,
	rk resource.ReferenceKey,
	destPolicy *types.DecodedDestinationPolicy,
) []*pbresource.Condition {
	var conditions []*pbresource.Condition

	// Since this is name-aligned, just switch the type to fetch the service.
	service := related.GetService(resource.ReplaceType(pbcatalog.ServiceType, rk.ToID()))
	if service != nil {
		allowedPortProtocols := make(map[string]pbcatalog.Protocol)
		for _, port := range service.Data.Ports {
			if port.Protocol == pbcatalog.Protocol_PROTOCOL_MESH {
				continue // skip
			}
			allowedPortProtocols[port.TargetPort] = port.Protocol
		}

		usedTargetPorts := make(map[string]any)
		for port := range destPolicy.Data.PortConfigs {
			svcPort := service.Data.FindPortByID(port)
			targetPort := svcPort.GetTargetPort() // svcPort could be nil

			serviceRef := resource.NewReferenceKey(service.Id)
			if svcPort == nil {
				conditions = append(conditions, ConditionUnknownDestinationPort(serviceRef.ToReference(), port))
			} else if _, ok := usedTargetPorts[targetPort]; ok {
				conditions = append(conditions, ConditionConflictDestinationPort(serviceRef.ToReference(), svcPort))
			} else {
				usedTargetPorts[targetPort] = struct{}{}
			}
		}
	} else {
		conditions = append(conditions, ConditionDestinationServiceNotFound(rk.ToReference()))
	}

	return conditions
}
