// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"context"
	"errors"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/hashicorp/consul/agent/pool"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/internal/storage/raft"
)

// raftHandle is the glue layer between the Raft resource storage backend and
// the exising Raft logic in Server.
type raftHandle struct{ s *Server }

func (h *raftHandle) IsLeader() bool {
	return h.s.IsLeader()
}

func (h *raftHandle) EnsureStrongConsistency(ctx context.Context) error {
	return h.s.consistentReadWithContext(ctx)
}

func (h *raftHandle) Apply(msg []byte) (any, error) {
	return h.s.raftApplyEncoded(
		structs.ResourceOperationType,
		append([]byte{uint8(structs.ResourceOperationType)}, msg...),
	)
}

func (h *raftHandle) DialLeader() (*grpc.ClientConn, error) {
	leaderAddr, _ := h.s.raft.LeaderWithID()
	if leaderAddr == "" {
		return nil, errors.New("leader unknown")
	}

	dc := h.s.config.Datacenter
	tlsCfg := h.s.tlsConfigurator

	return grpc.Dial(string(leaderAddr),
		// TLS is handled in the dialer below.
		grpc.WithTransportCredentials(insecure.NewCredentials()),

		// This dialer negotiates a connection on the multiplexed server port using
		// our type-byte prefix scheme (see Server.handleConn for other side of it).
		grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
			var d net.Dialer
			conn, err := d.DialContext(ctx, "tcp", addr)
			if err != nil {
				return nil, err
			}

			if tlsCfg.UseTLS(dc) {
				if _, err := conn.Write([]byte{byte(pool.RPCTLS)}); err != nil {
					conn.Close()
					return nil, err
				}

				tc, err := tlsCfg.OutgoingRPCWrapper()(dc, conn)
				if err != nil {
					conn.Close()
					return nil, err
				}
				conn = tc
			}

			if _, err := conn.Write([]byte{byte(pool.RPCRaftForwarding)}); err != nil {
				conn.Close()
				return nil, err
			}
			return conn, nil
		}),
	)
}

var _ raft.Handle = (*raftHandle)(nil)
