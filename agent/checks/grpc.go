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
	ctx    context.Context
	cancel context.CancelFunc

	connection *grpc.ClientConn
	client     hv1.HealthClient
	request    *hv1.HealthCheckRequest
	timeout    time.Duration
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

	var dialOptions []grpc.DialOption
	if tlsConfig != nil {
		dialOptions = append(dialOptions, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	} else {
		dialOptions = append(dialOptions, grpc.WithInsecure())
	}

	serverWithScheme := fmt.Sprintf("%s:///%s", resolver.GetDefaultScheme(), serverAndService[0])
	connection, err := grpc.DialContext(context.Background(), serverWithScheme, dialOptions...)
	if err != nil {
		// dial may return an error only when opts are invalid or grpc.WithBlock is used
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &GrpcHealthProbe{
		ctx:    ctx,
		cancel: cancel,

		connection: connection,
		client:     hv1.NewHealthClient(connection),
		request:    &request,
		timeout:    timeout,
	}
}

// Check if the target of this GrpcHealthProbe is healthy
// If nil is returned, target is healthy, otherwise target is not healthy
func (probe *GrpcHealthProbe) Check(target string) error {
	ctx, cancel := context.WithTimeout(probe.ctx, probe.timeout)
	defer cancel()

	response, err := probe.client.Check(ctx, probe.request)
	if err != nil {
		return err
	}
	if response.Status != hv1.HealthCheckResponse_SERVING {
		return fmt.Errorf("gRPC %s serving status: %s", target, response.Status)
	}

	return nil
}

func (probe *GrpcHealthProbe) Stop() {
	probe.cancel()
	probe.connection.Close()
}
