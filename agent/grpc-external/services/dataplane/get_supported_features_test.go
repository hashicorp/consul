package dataplane

import (
	"context"
	"testing"

	"github.com/hashicorp/go-hclog"
	mock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/acl"
	resolver "github.com/hashicorp/consul/acl/resolver"
	external "github.com/hashicorp/consul/agent/grpc-external"
	"github.com/hashicorp/consul/agent/grpc-external/testutils"
	"github.com/hashicorp/consul/proto-public/pbdataplane"
)

const testACLToken = "acl-token"

func TestSupportedDataplaneFeatures_Success(t *testing.T) {
	// Mock the ACL Resolver to return an authorizer with `service:write`.
	aclResolver := &MockACLResolver{}
	aclResolver.On("ResolveTokenAndDefaultMeta", testACLToken, mock.Anything, mock.Anything).
		Return(testutils.TestAuthorizerServiceWriteAny(t), nil)
	ctx := external.ContextWithToken(context.Background(), testACLToken)
	server := NewServer(Config{
		Logger:      hclog.NewNullLogger(),
		ACLResolver: aclResolver,
	})
	client := testClient(t, server)
	resp, err := client.GetSupportedDataplaneFeatures(ctx, &pbdataplane.GetSupportedDataplaneFeaturesRequest{})
	require.NoError(t, err)
	require.Equal(t, 3, len(resp.SupportedDataplaneFeatures))

	for _, feature := range resp.SupportedDataplaneFeatures {
		switch feature.GetFeatureName() {
		case pbdataplane.DataplaneFeatures_DATAPLANE_FEATURES_EDGE_CERTIFICATE_MANAGEMENT:
			require.True(t, feature.GetSupported())
		case pbdataplane.DataplaneFeatures_DATAPLANE_FEATURES_WATCH_SERVERS:
			require.True(t, feature.GetSupported())
		case pbdataplane.DataplaneFeatures_DATAPLANE_FEATURES_ENVOY_BOOTSTRAP_CONFIGURATION:
			require.True(t, feature.GetSupported())
		default:
			require.False(t, feature.GetSupported())
		}
	}
}

func TestSupportedDataplaneFeatures_Unauthenticated(t *testing.T) {
	// Mock the ACL resolver to return ErrNotFound.
	aclResolver := &MockACLResolver{}
	aclResolver.On("ResolveTokenAndDefaultMeta", mock.Anything, mock.Anything, mock.Anything).
		Return(resolver.Result{}, acl.ErrNotFound)
	ctx := external.ContextWithToken(context.Background(), testACLToken)
	server := NewServer(Config{
		Logger:      hclog.NewNullLogger(),
		ACLResolver: aclResolver,
	})
	client := testClient(t, server)
	resp, err := client.GetSupportedDataplaneFeatures(ctx, &pbdataplane.GetSupportedDataplaneFeaturesRequest{})
	require.Error(t, err)
	require.Equal(t, codes.Unauthenticated.String(), status.Code(err).String())
	require.Nil(t, resp)
}

func TestSupportedDataplaneFeatures_PermissionDenied(t *testing.T) {
	// Mock the ACL resolver to return a deny all authorizer
	aclResolver := &MockACLResolver{}
	aclResolver.On("ResolveTokenAndDefaultMeta", testACLToken, mock.Anything, mock.Anything).
		Return(testutils.TestAuthorizerDenyAll(t), nil)
	ctx := external.ContextWithToken(context.Background(), testACLToken)
	server := NewServer(Config{
		Logger:      hclog.NewNullLogger(),
		ACLResolver: aclResolver,
	})
	client := testClient(t, server)
	resp, err := client.GetSupportedDataplaneFeatures(ctx, &pbdataplane.GetSupportedDataplaneFeaturesRequest{})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied.String(), status.Code(err).String())
	require.Nil(t, resp)
}
