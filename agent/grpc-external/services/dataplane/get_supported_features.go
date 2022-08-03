package dataplane

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	acl "github.com/hashicorp/consul/acl"
	external "github.com/hashicorp/consul/agent/grpc-external"
	structs "github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto-public/pbdataplane"
)

func (s *Server) GetSupportedDataplaneFeatures(ctx context.Context, req *pbdataplane.GetSupportedDataplaneFeaturesRequest) (*pbdataplane.GetSupportedDataplaneFeaturesResponse, error) {
	logger := s.Logger.Named("get-supported-dataplane-features").With("request_id", external.TraceID())

	logger.Trace("Started processing request")
	defer logger.Trace("Finished processing request")

	// Require the given ACL token to have `service:write` on any service
	token := external.TokenFromContext(ctx)
	var authzContext acl.AuthorizerContext
	entMeta := structs.WildcardEnterpriseMetaInPartition(structs.WildcardSpecifier)
	authz, err := s.ACLResolver.ResolveTokenAndDefaultMeta(token, entMeta, &authzContext)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	if err := authz.ToAllowAuthorizer().ServiceWriteAnyAllowed(&authzContext); err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
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
	}

	return &pbdataplane.GetSupportedDataplaneFeaturesResponse{SupportedDataplaneFeatures: supportedFeatures}, nil
}
