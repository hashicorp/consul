// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package middleware

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/consul/tlsutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

const AllowedPeerEndpointPrefix = "/hashicorp.consul.internal.peerstream.PeerStreamService/"

// AuthInterceptor provides gRPC interceptors for restricting endpoint access based
// on SNI. If the connection is plaintext, this filter will not activate, and the
// connection will be allowed to proceed.
type AuthInterceptor struct {
	TLS    *tlsutil.Configurator
	Logger Logger
}

// InterceptUnary prevents non-streaming gRPC calls from calling certain endpoints,
// based on the SNI information.
func (a *AuthInterceptor) InterceptUnary(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("unable to fetch peer info from grpc context")
	}
	err := restrictPeeringEndpoints(p.AuthInfo, a.TLS.PeeringServerName(), info.FullMethod)
	if err != nil {
		return nil, err
	}
	return handler(ctx, req)
}

// InterceptUnary prevents streaming gRPC calls from calling certain endpoints,
// based on the SNI information.
func (a *AuthInterceptor) InterceptStream(
	srv interface{},
	ss grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	p, ok := peer.FromContext(ss.Context())
	if !ok {
		return fmt.Errorf("unable to fetch peer info from grpc context")
	}
	err := restrictPeeringEndpoints(p.AuthInfo, a.TLS.PeeringServerName(), info.FullMethod)
	if err != nil {
		return err
	}
	return handler(srv, ss)
}

// restrictPeeringEndpoints will return an error if a peering TLS connection attempts to call
// a non-peering endpoint. This is necessary, because the peer streaming workflow does not
// present a mutual TLS certificate, and is allowed to bypass the `tls.grpc.verify_incoming`
// check as a special case. See the `tlsutil.Configurator` for this bypass.
func restrictPeeringEndpoints(authInfo credentials.AuthInfo, peeringSNI string, endpoint string) error {
	// No peering connection has been configured
	if peeringSNI == "" {
		return nil
	}
	// This indicates a plaintext connection.
	if authInfo == nil {
		return nil
	}
	// Otherwise attempt to check the AuthInfo for TLS credentials.
	tlsAuth, ok := authInfo.(credentials.TLSInfo)
	if !ok {
		return status.Error(codes.Unauthenticated, "invalid transport credentials")
	}

	if tlsAuth.State.ServerName == peeringSNI {
		// Prevent any calls, except those in the PeerStreamService
		if !strings.HasPrefix(endpoint, AllowedPeerEndpointPrefix) {
			return status.Error(codes.PermissionDenied, "invalid permissions to the specified endpoint")
		}
	}
	return nil
}
