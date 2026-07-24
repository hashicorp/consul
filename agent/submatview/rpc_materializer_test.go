// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package submatview

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/proto/private/pbservice"
	"github.com/hashicorp/consul/proto/private/pbsubscribe"
)

type aclNotFoundStreamClient struct{}

func (aclNotFoundStreamClient) Subscribe(
	_ context.Context,
	_ *pbsubscribe.SubscribeRequest,
	_ ...grpc.CallOption,
) (pbsubscribe.StateChangeSubscription_SubscribeClient, error) {
	return nil, status.Error(codes.Unknown, acl.ErrNotFound.Error())
}

func TestRPCMaterializer_TerminalACLNotFound(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	m := NewRPCMaterializer(aclNotFoundStreamClient{}, Deps{
		Logger: hclog.NewNullLogger(),
		View:   &fakeView{srvs: make(map[string]*pbservice.CheckServiceNode)},
		Request: func(uint64) *pbsubscribe.SubscribeRequest {
			return &pbsubscribe.SubscribeRequest{
				Topic: pbsubscribe.Topic_ServiceHealth,
				Subject: &pbsubscribe.SubscribeRequest_NamedSubject{
					NamedSubject: &pbsubscribe.NamedSubject{Key: "web"},
				},
				Token: "deleted-token",
			}
		},
	})

	runDone := make(chan struct{})
	go func() {
		m.Run(ctx)
		close(runDone)
	}()

	select {
	case <-runDone:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Run should exit immediately on ACL not found without retrying")
	}
}

func TestIsTerminalError(t *testing.T) {
	require.True(t, isTerminalError(status.Error(codes.Unknown, "ACL not found")))
	require.False(t, isTerminalError(status.Error(codes.Unavailable, "server unavailable")))
	require.False(t, isTerminalError(resetErr("stream reset requested")))
}
