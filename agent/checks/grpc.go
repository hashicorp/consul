// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package checks

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	hv1 "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/resolver"
)

// GrpcHealthProbe connects to gRPC application and queries health service for application/service status.
type GrpcHealthProbe struct {
	server      string
	request     *hv1.HealthCheckRequest
	timeout     time.Duration
	dialOptions []grpc.DialOption
}

// NewGrpcHealthProbe constructs GrpcHealthProbe from target string in format
// server[/service]
// If service is omitted, health of the entire application is probed
func NewGrpcHealthProbe(target string, timeout time.Duration, tlsConfig *tls.Config) *GrpcHealthProbe {
	serverAndService := strings.SplitN(target, "/", 2)

	request := hv1.HealthCheckRequest{}
	if len(serverAndService) > 1 {
		request.Service = serverAndService[1]
	}

	var dialOptions = []grpc.DialOption{}

	if tlsConfig != nil {
		dialOptions = append(dialOptions, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	} else {
		//nolint:staticcheck
		dialOptions = append(dialOptions, grpc.WithInsecure())
	}

	return &GrpcHealthProbe{
		request:     &request,
		timeout:     timeout,
		dialOptions: dialOptions,
	}
}

// Check if the target of this GrpcHealthProbe is healthy
// If nil is returned, target is healthy, otherwise target is not healthy
func (probe *GrpcHealthProbe) Check(target string) error {
	serverAndService := strings.SplitN(target, "/", 2)
	serverWithScheme := fmt.Sprintf("%s:///%s", resolver.GetDefaultScheme(), serverAndService[0])

	ctx, cancel := context.WithTimeout(context.Background(), probe.timeout)
	defer cancel()

	connection, err := grpc.DialContext(ctx, serverWithScheme, probe.dialOptions...)
	if err != nil {
		return err
	}
	defer connection.Close()

	client := hv1.NewHealthClient(connection)
	response, err := client.Check(ctx, probe.request)
	if err != nil {
		return err
	}
	if response.Status != hv1.HealthCheckResponse_SERVING {
		return fmt.Errorf("gRPC %s serving status: %s", target, response.Status)
	}

	return nil
}
