// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dataplane

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	external "github.com/hashicorp/consul/agent/grpc-external"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto-public/pbdataplane"
	"github.com/hashicorp/consul/version"
)

func (s *Server) GetSupportedDataplaneFeatures(ctx context.Context, args *pbdataplane.GetSupportedDataplaneFeaturesRequest) (*pbdataplane.GetSupportedDataplaneFeaturesResponse, error) {
	logger := s.Logger.Named("get-supported-dataplane-features").With("request_id", external.TraceID())

	logger.Trace("Started processing request")
	defer logger.Trace("Finished processing request")

	options, err := external.QueryOptionsFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	// external.RequireAnyValidACLToken(s.ACLResolver, options.Token)
	fmt.Println("forward 1", args)
	fmt.Println("forward 1.0", args.Token)
	fmt.Println("forward 1.1.", args.QueryOptions)
	fmt.Println("forward 1.2.", options)
	fmt.Println("forward 1.3.", options.Token)

	// if err := external.RequireAnyValidACLToken(s.ACLResolver, options.Token); err != nil {
	fmt.Println("forward 2", err)
	rcpReq := &structs.ACLTokenGetRequest{
		TokenID:     options.Token,
		TokenIDType: structs.ACLTokenAccessor,
		Expanded:    false,
		Datacenter:  s.Datacenter,
		// EnterpriseMeta: args.EnterpriseMeta,
		QueryOptions: options,
	}

	rcpReply := &structs.ACLTokenResponse{}
	done, rpcErr := s.ConsulServer.ForwardRPC("ACL.RequireAnyValidACLToken", rcpReq, rcpReply)
	if done && rpcErr != nil {
		fmt.Println("forward 3", rpcErr)
		return nil, rpcErr
	}
	fmt.Println("forward 4 done done done wihtout error", err, rpcErr)

	if !done {
		fmt.Println("forward 5", err, rpcErr)
		// return nil, err
	}
	fmt.Println("forward 6", err, rpcErr)

	// }
	fmt.Println("forward 7")

	supportedFeatures := []*pbdataplane.DataplaneFeatureSupport{
		{
			FeatureName: pbdataplane.DataplaneFeatures_DATAPLANE_FEATURES_WATCH_SERVERS,
			Supported:   true,
		},
		{
			FeatureName: pbdataplane.DataplaneFeatures_DATAPLANE_FEATURES_EDGE_CERTIFICATE_MANAGEMENT,
			Supported:   true,
		},
		{
			FeatureName: pbdataplane.DataplaneFeatures_DATAPLANE_FEATURES_ENVOY_BOOTSTRAP_CONFIGURATION,
			Supported:   true,
		},
		{
			FeatureName: pbdataplane.DataplaneFeatures_DATAPLANE_FEATURES_FIPS,
			Supported:   version.IsFIPS(),
		},
	}

	return &pbdataplane.GetSupportedDataplaneFeaturesResponse{SupportedDataplaneFeatures: supportedFeatures}, nil
}
