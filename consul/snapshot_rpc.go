package consul

import (
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/hashicorp/consul/consul/snapshot"
	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/go-msgpack/codec"
)

func (s *Server) dispatchSnapshotRequest(args *structs.SnapshotRequest, in io.Reader) (io.ReadCloser, error) {
	// We may need to forward if this is not our datacenter or we are not
	// the leader.
	if dc := args.Datacenter; dc != s.config.Datacenter {
		server, ok := s.getRemoteServer(dc)
		if !ok {
			return nil, structs.ErrNoDCPath
		}
		return SnapshotRPC(dc, server.Addr, s.connPool, args, in)
	} else if isLeader, server := s.getLeader(); !isLeader {
		if server == nil {
			return nil, structs.ErrNoLeader
		}
		return SnapshotRPC(dc, server.Addr, s.connPool, args, in)
	}

	// TODO - ACLs - need a management token

	switch args.Op {
	case structs.SnapshotSave:
		return snapshot.New(s.logger, s.raft)

	case structs.SnapshotRestore:
		if err := snapshot.Restore(s.logger, in, s.raft); err != nil {
			return nil, fmt.Errorf("failed to restore snapshot: %v", err)
		}
		return nil, nil

	default:
		return nil, fmt.Errorf("unrecognized snapshot op %q", args.Op)
	}
}

// handleSnapshotRequest reads the request from the conn and dispatches it.
func (s *Server) handleSnapshotRequest(conn net.Conn) error {
	var args structs.SnapshotRequest
	dec := codec.NewDecoder(conn, &codec.MsgpackHandle{})
	if err := dec.Decode(&args); err != nil {
		return fmt.Errorf("failed to decode request: %v", err)
	}

	var resp structs.SnapshotResponse
	snap, err := s.dispatchSnapshotRequest(&args, conn)
	if err != nil {
		resp.Error = err.Error()
	}
	defer func() {
		if err := snap.Close(); err != nil {
			s.logger.Printf("[ERR] consul: Failed to close snapshot: %v", err)
		}
	}()

	enc := codec.NewEncoder(conn, &codec.MsgpackHandle{})
	if err := enc.Encode(&resp); err != nil {
		return fmt.Errorf("failed to encode response: %v", err)
	}
	if _, err := io.Copy(conn, snap); err != nil {
		return fmt.Errorf("failed to stream snapshot: %v", err)
	}

	return nil
}

// XXX - TODO
func SnapshotRPC(dc string, addr net.Addr, pool *ConnPool,
	args *structs.SnapshotRequest, in io.Reader) (io.ReadCloser, error) {

	// Dial since we make a fresh connection for each RPC.
	conn, err := pool.Dial(dc, addr)
	if err != nil {
		return nil, err
	}

	var keep bool
	defer func() {
		if keep {
			return
		}

		conn.Close()
	}()

	// Write the snapshot RPC byte to set the mode, then perform the
	// request.
	if _, err := conn.Write([]byte{byte(rpcSnapshot)}); err != nil {
		return nil, err
	}

	enc := codec.NewEncoder(conn, &codec.MsgpackHandle{})
	if err := enc.Encode(&args); err != nil {
		return nil, fmt.Errorf("failed to encode request: %v", err)
	}
	if _, err := io.Copy(conn, in); err != nil {
		return nil, fmt.Errorf("failed to copy snapshot in: %v", err)
	}

	var resp structs.SnapshotResponse
	dec := codec.NewDecoder(conn, &codec.MsgpackHandle{})
	if err := dec.Decode(&resp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}
	if resp.Error != "" {
		return nil, errors.New(resp.Error)
	}

	keep = true
	return conn, nil
}
