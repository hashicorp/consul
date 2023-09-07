// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dataplane

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	external "github.com/hashicorp/consul/agent/grpc-external"
	"github.com/hashicorp/consul/proto-public/pbdataplane"
	"github.com/hashicorp/consul/version"
)

func (s *Server) GetSupportedDataplaneFeatures(ctx context.Context, req *pbdataplane.GetSupportedDataplaneFeaturesRequest) (*pbdataplane.GetSupportedDataplaneFeaturesResponse, error) {
	logger := s.Logger.Named("get-supported-dataplane-features").With("request_id", external.TraceID())

	logger.Trace("Started processing request")
	defer logger.Trace("Finished processing request")

	options, err := external.QueryOptionsFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if err := external.RequireAnyValidACLToken(s.ACLResolver, options.Token); err != nil {
		return nil, err
	}

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
