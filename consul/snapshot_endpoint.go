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

// dispatchSnapshotRequest takes an incoming request structure with possibly some
// streaming data (for a restore) and returns possibly some streaming data (for
// a snapshot save). We can't use the normal RPC mechanism in a streaming manner
// like this, so we have to dispatch these by hand.
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

	// Verify token is allowed to operate on snapshots. There's only a
	// single ACL sense here (not read and write) since reading gets you
	// all the ACLs and you could escalate from there.
	if acl, err := s.resolveToken(args.Token); err != nil {
		return nil, err
	} else if acl == nil || !acl.Snapshot() {
		return nil, permissionDeniedErr
	}

	// Dispatch the operation.
	switch args.Op {
	case structs.SnapshotSave:
		return snapshot.New(s.logger, s.raft)

	case structs.SnapshotRestore:
		if err := snapshot.Restore(s.logger, in, s.raft); err != nil {
			return nil, err
		}
		return nil, nil

	default:
		return nil, fmt.Errorf("unrecognized snapshot op %q", args.Op)
	}
}

// handleSnapshotRequest reads the request from the conn and dispatches it. This
// will be called from a goroutine after an incoming stream is determined to be
// a snapshot request.
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

// SnapshotRPC is a streaming client function for performing a snapshot RPC
// request to a remote server. It will dial up a fresh connection for each
// request, send the request header, and then stream in any data from the given
// reader (for a restore). It will then parse the received response header, and
// if there's no error will return a reader for the streaming part of the
// response (for a snapshot save). If this doesn't return an error, you must
// close the returned reader.
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
