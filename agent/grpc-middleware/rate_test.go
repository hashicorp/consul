package middleware

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/agent/consul/rate"
)

func TestServerRateLimiterMiddleware_Integration(t *testing.T) {
	limiter := rate.NewMockRequestLimitsHandler(t)

	server := grpc.NewServer(
		grpc.InTapHandle(ServerRateLimiterMiddleware(limiter, NewPanicHandler(hclog.NewNullLogger()))),
	)
	server.RegisterService(&healthpb.Health_ServiceDesc, health.NewServer())

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := lis.Close(); err != nil {
			t.Logf("failed to close listener: %v", err)
		}
	})
	go server.Serve(lis)
	t.Cleanup(server.Stop)

	conn, err := grpc.Dial(
		lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Logf("failed to close client connection: %v", err)
		}
	})
	client := healthpb.NewHealthClient(conn)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	t.Run("ErrRetryElsewhere = ResourceExhausted", func(t *testing.T) {
		limiter.On("Allow", mock.Anything).
			Run(func(args mock.Arguments) {
				op := args.Get(0).(rate.Operation)
				require.Equal(t, "/grpc.health.v1.Health/Check", op.Name)

				addr := op.SourceAddr.(*net.TCPAddr)
				require.True(t, addr.IP.IsLoopback())
			}).
			Return(rate.ErrRetryElsewhere).
			Once()

		_, err = client.Check(ctx, &healthpb.HealthCheckRequest{})
		require.Error(t, err)
		require.Equal(t, codes.ResourceExhausted.String(), status.Code(err).String())
	})

	t.Run("ErrRetryLater = Unavailable", func(t *testing.T) {
		limiter.On("Allow", mock.Anything).
			Return(rate.ErrRetryLater).
			Once()

		_, err = client.Check(ctx, &healthpb.HealthCheckRequest{})
		require.Error(t, err)
		require.Equal(t, codes.Unavailable.String(), status.Code(err).String())
	})

	t.Run("unexpected error", func(t *testing.T) {
		limiter.On("Allow", mock.Anything).
			Return(errors.New("uh oh")).
			Once()

		_, err = client.Check(ctx, &healthpb.HealthCheckRequest{})
		require.Error(t, err)
		require.Equal(t, codes.Internal.String(), status.Code(err).String())
	})

	t.Run("operation allowed", func(t *testing.T) {
		limiter.On("Allow", mock.Anything).
			Return(nil).
			Once()

		_, err = client.Check(ctx, &healthpb.HealthCheckRequest{})
		require.NoError(t, err)
	})

	t.Run("Allow panics", func(t *testing.T) {
		limiter.On("Allow", mock.Anything).
			Panic("uh oh").
			Once()

		_, err = client.Check(ctx, &healthpb.HealthCheckRequest{})
		require.Error(t, err)
		require.Equal(t, codes.Internal.String(), status.Code(err).String())
	})
}
