// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

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
	structs "github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto-public/pbdataplane"
	"github.com/hashicorp/consul/version"
)

const testACLToken = "acl-token"

func TestSupportedDataplaneFeatures_Success(t *testing.T) {
	// Mock the ACL Resolver to return an authorizer with `service:write`.
	aclResolver := &MockACLResolver{}
	aclResolver.On("ResolveTokenAndDefaultMeta", testACLToken, mock.Anything, mock.Anything).
		Return(testutils.ACLServiceWriteAny(t), nil)

	options := structs.QueryOptions{Token: testACLToken}
	ctx, err := external.ContextWithQueryOptions(context.Background(), options)
	require.NoError(t, err)

	server := NewServer(Config{
		Logger:      hclog.NewNullLogger(),
		ACLResolver: aclResolver,
	})
	client := testClient(t, server)
	resp, err := client.GetSupportedDataplaneFeatures(ctx, &pbdataplane.GetSupportedDataplaneFeaturesRequest{})
	require.NoError(t, err)
	require.Equal(t, 4, len(resp.SupportedDataplaneFeatures))

	for _, feature := range resp.SupportedDataplaneFeatures {
		switch feature.GetFeatureName() {
		case pbdataplane.DataplaneFeatures_DATAPLANE_FEATURES_EDGE_CERTIFICATE_MANAGEMENT:
			require.True(t, feature.GetSupported())
		case pbdataplane.DataplaneFeatures_DATAPLANE_FEATURES_WATCH_SERVERS:
			require.True(t, feature.GetSupported())
		case pbdataplane.DataplaneFeatures_DATAPLANE_FEATURES_ENVOY_BOOTSTRAP_CONFIGURATION:
			require.True(t, feature.GetSupported())
		case pbdataplane.DataplaneFeatures_DATAPLANE_FEATURES_FIPS:
			require.Equal(t, version.IsFIPS(), feature.GetSupported())
		default:
			require.False(t, feature.GetSupported())
		}
	}
}

func TestSupportedDataplaneFeatures_ACLsDisabled(t *testing.T) {
	aclResolver := &MockACLResolver{}
	aclResolver.On("ResolveTokenAndDefaultMeta", "", mock.Anything, mock.Anything).
		Return(testutils.ACLsDisabled(t), nil)

	options := structs.QueryOptions{Token: ""}
	ctx, err := external.ContextWithQueryOptions(context.Background(), options)
	require.NoError(t, err)

	server := NewServer(Config{
		Logger:      hclog.NewNullLogger(),
		ACLResolver: aclResolver,
	})
	client := testClient(t, server)
	resp, err := client.GetSupportedDataplaneFeatures(ctx, &pbdataplane.GetSupportedDataplaneFeaturesRequest{})
	require.NoError(t, err)
	require.Equal(t, 4, len(resp.SupportedDataplaneFeatures))
}

func TestSupportedDataplaneFeatures_InvalidACLToken(t *testing.T) {
	// Mock the ACL resolver to return ErrNotFound.
	aclResolver := &MockACLResolver{}
	aclResolver.On("ResolveTokenAndDefaultMeta", mock.Anything, mock.Anything, mock.Anything).
		Return(resolver.Result{}, acl.ErrNotFound)

	options := structs.QueryOptions{Token: testACLToken}
	ctx, err := external.ContextWithQueryOptions(context.Background(), options)
	require.NoError(t, err)

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

func TestSupportedDataplaneFeatures_AnonymousACLToken(t *testing.T) {
	// Mock the ACL resolver to return ErrNotFound.
	aclResolver := &MockACLResolver{}
	aclResolver.On("ResolveTokenAndDefaultMeta", mock.Anything, mock.Anything, mock.Anything).
		Return(testutils.ACLAnonymous(t), nil)

	options := structs.QueryOptions{Token: testACLToken}
	ctx, err := external.ContextWithQueryOptions(context.Background(), options)
	require.NoError(t, err)

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

func TestSupportedDataplaneFeatures_NoPermissions(t *testing.T) {
	// Mock the ACL resolver to return a deny all authorizer
	aclResolver := &MockACLResolver{}
	aclResolver.On("ResolveTokenAndDefaultMeta", testACLToken, mock.Anything, mock.Anything).
		Return(testutils.ACLNoPermissions(t), nil)

	options := structs.QueryOptions{Token: testACLToken}
	ctx, err := external.ContextWithQueryOptions(context.Background(), options)
	require.NoError(t, err)

	server := NewServer(Config{
		Logger:      hclog.NewNullLogger(),
		ACLResolver: aclResolver,
	})
	client := testClient(t, server)
	_, err = client.GetSupportedDataplaneFeatures(ctx, &pbdataplane.GetSupportedDataplaneFeaturesRequest{})
	require.NoError(t, err)
}
