// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package apigateways

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/apigateways/fetcher"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"google.golang.org/protobuf/types/known/anypb"
)

const (
	ControllerName = "consul.io/api-gateway"
	GatewayKind    = "api-gateway"
)

func Controller() *controller.Controller {
	r := &reconciler{}

	return controller.NewController(ControllerName, pbmesh.APIGatewayType).
		WithReconciler(r)
}

type reconciler struct{}

// Reconcile is responsible for creating a Service w/ a APIGateway owner,
// in addition to other things discussed in the RFC.
func (r *reconciler) Reconcile(ctx context.Context, rt controller.Runtime, req controller.Request) error {
	rt.Logger = rt.Logger.With("resource-id", req.ID)
	rt.Logger.Trace("reconciling api gateway")

	dataFetcher := fetcher.New(rt.Client)

	decodedAPIGateway, err := dataFetcher.FetchAPIGateway(ctx, req.ID)
	if err != nil {
		rt.Logger.Trace("error reading the apigateway", "apigatewayID", req.ID, "error", err)
		return err
	} else if decodedAPIGateway == nil {
		rt.Logger.Trace("apigateway not found", "apigatewayID", req.ID)
		return nil
	}

	apigw := decodedAPIGateway.Data

	ports := make([]*pbcatalog.ServicePort, 0, len(apigw.Listeners))

	for _, listener := range apigw.Listeners {
		ports = append(ports, &pbcatalog.ServicePort{
			Protocol:    listenerProtocolToCatalogProtocol(listener.Protocol),
			TargetPort:  listener.Name,
			VirtualPort: listener.Port,
		})
	}

	service := &pbcatalog.Service{
		Workloads: &pbcatalog.WorkloadSelector{
			Prefixes: []string{req.ID.Name},
		},
		Ports: ports,
	}

	serviceData, err := anypb.New(service)
	if err != nil {
		rt.Logger.Trace("error creating the serviceData", "apigatewayID", req.ID, "error", err)
		return err
	}

	// TODO NET-7378
	_, err = rt.Client.Write(ctx, &pbresource.WriteRequest{
		Resource: &pbresource.Resource{
			Data:     serviceData,
			Id:       resource.ReplaceType(pbcatalog.ServiceType, req.ID),
			Metadata: map[string]string{"gateway-kind": GatewayKind},
			Owner:    req.ID,
		},
	})

	if err != nil {
		rt.Logger.Trace("error writing the service", "apigatewayID", req.ID, "error", err)
		return err
	}

	rt.Logger.Trace("successfully reconciled APIGateway", "apigatewayID", req.ID)
	return nil
}

func listenerProtocolToCatalogProtocol(listenerProtocol string) pbcatalog.Protocol {
	switch strings.ToLower(listenerProtocol) {
	case "http":
		return pbcatalog.Protocol_PROTOCOL_HTTP
	case "tcp":
		return pbcatalog.Protocol_PROTOCOL_TCP
	case "grpc":
		return pbcatalog.Protocol_PROTOCOL_GRPC
	default:
		panic(fmt.Sprintf("this is a programmer error, the only available protocols are tcp/http/grpc. You provided: %q", listenerProtocol))
	}
}
