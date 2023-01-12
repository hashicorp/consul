package assert

import (
	"context"
	"testing"

	"fortio.org/fortio/fgrpc"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func GRPCPing(t *testing.T, addr string) {
	pingConn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	pingCl := fgrpc.NewPingServerClient(pingConn)
	payload := addr
	// TODO: not sure why, but this doesn't work the first time, needs some time to become ready
	var msg *fgrpc.PingMessage
	retry.Run(t, func(r *retry.R) {
		msg, err = pingCl.Ping(context.Background(), &fgrpc.PingMessage{
			Payload: payload,
		})
		if err != nil {
			r.Error(err)
		}
	})
	assert.Equal(t, payload, msg.Payload)
}
