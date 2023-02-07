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
	"google.golang.org/grpc/status"

	pbacl "github.com/hashicorp/consul/proto-public/pbacl"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/agent/consul/rate"
)

func TestServerRateLimiterMiddleware_Integration(t *testing.T) {
	limiter := rate.NewMockRequestLimitsHandler(t)

	logger := hclog.NewNullLogger()
	server := grpc.NewServer(
		grpc.InTapHandle(ServerRateLimiterMiddleware(limiter, NewPanicHandler(logger), logger)),
	)
	pbacl.RegisterACLServiceServer(server, mockACLServer{})

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
	client := pbacl.NewACLServiceClient(conn)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	t.Run("ErrRetryElsewhere = ResourceExhausted", func(t *testing.T) {
		limiter.On("Allow", mock.Anything).
			Run(func(args mock.Arguments) {
				op := args.Get(0).(rate.Operation)
				require.Equal(t, "/hashicorp.consul.acl.ACLService/Login", op.Name)

				addr := op.SourceAddr.(*net.TCPAddr)
				require.True(t, addr.IP.IsLoopback())
			}).
			Return(rate.ErrRetryElsewhere).
			Once()

		_, err = client.Login(ctx, &pbacl.LoginRequest{})
		require.Error(t, err)
		require.Equal(t, codes.ResourceExhausted.String(), status.Code(err).String())
	})

	t.Run("ErrRetryLater = Unavailable", func(t *testing.T) {
		limiter.On("Allow", mock.Anything).
			Return(rate.ErrRetryLater).
			Once()

		_, err = client.Login(ctx, &pbacl.LoginRequest{})
		require.Error(t, err)
		require.Equal(t, codes.Unavailable.String(), status.Code(err).String())
	})

	t.Run("unexpected error", func(t *testing.T) {
		limiter.On("Allow", mock.Anything).
			Return(errors.New("uh oh")).
			Once()

		_, err = client.Login(ctx, &pbacl.LoginRequest{})
		require.Error(t, err)
		require.Equal(t, codes.Internal.String(), status.Code(err).String())
	})

	t.Run("operation allowed", func(t *testing.T) {
		limiter.On("Allow", mock.Anything).
			Return(nil).
			Once()

		_, err = client.Login(ctx, &pbacl.LoginRequest{})
		require.NoError(t, err)
	})

	t.Run("Allow panics", func(t *testing.T) {
		limiter.On("Allow", mock.Anything).
			Panic("uh oh").
			Once()

		_, err = client.Login(ctx, &pbacl.LoginRequest{})
		require.Error(t, err)
		require.Equal(t, codes.Internal.String(), status.Code(err).String())
	})
}

type mockACLServer struct {
	pbacl.ACLServiceServer
}

func (mockACLServer) Login(context.Context, *pbacl.LoginRequest) (*pbacl.LoginResponse, error) {
	return &pbacl.LoginResponse{}, nil
}
