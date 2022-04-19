package dataplane

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	acl "github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/grpc/public"
	structs "github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto-public/pbdataplane"
)

func (d *Server) SupportedDataplaneFeatures(ctx context.Context, req *pbdataplane.SupportedDataplaneFeaturesRequest) (*pbdataplane.SupportedDataplaneFeaturesResponse, error) {
	d.Logger.Trace("Received request for supported dataplane features")

	// Require the given ACL token to have `service:write` on any service
	token := public.TokenFromContext(ctx)
	var authzContext acl.AuthorizerContext
	entMeta := structs.WildcardEnterpriseMetaInPartition(structs.WildcardSpecifier)
	authz, err := d.ACLResolver.ResolveTokenAndDefaultMeta(token, entMeta, &authzContext)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	if err := authz.ToAllowAuthorizer().ServiceWriteAnyAllowed(&authzContext); err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}

	supportedFeatures := []*pbdataplane.DataplaneFeatureSupport{
		{
			FeatureName: pbdataplane.DataplaneFeatures_WATCH_SERVERS,
			Supported:   true,
		},
		{
			FeatureName: pbdataplane.DataplaneFeatures_EDGE_CERTIFICATE_MANAGEMENT,
			Supported:   true,
		},
		{
			FeatureName: pbdataplane.DataplaneFeatures_ENVOY_BOOTSTRAP_CONFIGURATION,
			Supported:   true,
		},
	}

	return &pbdataplane.SupportedDataplaneFeaturesResponse{SupportedDataplaneFeatures: supportedFeatures}, nil
}
