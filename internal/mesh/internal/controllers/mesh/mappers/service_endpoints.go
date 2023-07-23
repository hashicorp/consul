package mappers

import (
	"context"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// MapServiceEndpointsToProxyStateTemplate maps catalog.ServiceEndpoints objects to the IDs of
// ProxyStateTemplate.
// For a downstream proxy, we only need to generate requests from workloads this endpoints points to
// If this service endpoints is an upstream for some proxies, we need to generate requests for those proxies as well.
// so we need to have a map from service endpoints to downstream proxy Ids
func MapServiceEndpointsToProxyStateTemplate(_ context.Context, _ controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	// This mapper needs to look up workload IDs from service endpoints and replace them with proxystatetemplatetype.
	var serviceEndpoints pbcatalog.ServiceEndpoints
	err := res.Data.UnmarshalTo(&serviceEndpoints)
	if err != nil {
		return nil, err
	}

	var result []controller.Request

	for _, endpoint := range serviceEndpoints.Endpoints {
		// Convert the reference to a workload to a ProxyStateTemplate ID.
		// Because these resources are name and tenancy aligned, we only need to change the type.
		result = append(result, controller.Request{
			ID: &pbresource.ID{
				Name:    endpoint.TargetRef.Name,
				Tenancy: endpoint.TargetRef.Tenancy,
				Type:    types.ProxyStateTemplateType,
			},
		})
	}

	return result, err
}
