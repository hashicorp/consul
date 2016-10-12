// The snapshot endpoint is a special non-RPC endpoint that supports streaming
// for taking and restoring snapshots for disaster recovery. This gets wired
// directly into Consul's stream handler, and a new TCP connection is made for
// each request.
//
// This also includes a SnapshotRPC() function, which acts as a lightweight
// client that knows the details of the stream protocol.
package consul

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
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
		return SnapshotRPC(s.connPool, dc, server.Addr, args, in)
	} else if isLeader, server := s.getLeader(); !isLeader {
		if server == nil {
			return nil, structs.ErrNoLeader
		}
		return SnapshotRPC(s.connPool, dc, server.Addr, args, in)
	}

	// Verify token is allowed to operate on snapshots. There's only a
	// single ACL sense here (not read and write) since reading gets you
	// all the ACLs and you could escalate from there.
	if acl, err := s.resolveToken(args.Token); err != nil {
		return nil, err
	} else if acl != nil && !acl.Snapshot() {
		return nil, permissionDeniedErr
	}

	// Dispatch the operation.
	switch args.Op {
	case structs.SnapshotSave:
		if err := s.consistentRead(); err != nil {
			return nil, err
		}
		return snapshot.New(s.logger, s.raft)

	case structs.SnapshotRestore:
		if err := snapshot.Restore(s.logger, in, s.raft); err != nil {
			return nil, err
		}
		return ioutil.NopCloser(bytes.NewReader([]byte(""))), nil

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
		goto RESPOND
	}
	defer func() {
		if err := snap.Close(); err != nil {
			s.logger.Printf("[ERR] consul: Failed to close snapshot: %v", err)
		}
	}()

RESPOND:
	enc := codec.NewEncoder(conn, &codec.MsgpackHandle{})
	if err := enc.Encode(&resp); err != nil {
		return fmt.Errorf("failed to encode response: %v", err)
	}
	if snap != nil {
		if _, err := io.Copy(conn, snap); err != nil {
			return fmt.Errorf("failed to stream snapshot: %v", err)
		}
	}

	return nil
}

// SnapshotRPC is a streaming client function for performing a snapshot RPC
// request to a remote server. It will create a fresh connection for each
// request, send the request header, and then stream in any data from the
// reader (for a restore). It will then parse the received response header, and
// if there's no error will return an io.ReadCloser (that you must close) with
// the streaming output (for a snapshot).
func SnapshotRPC(pool *ConnPool, dc string, addr net.Addr,
	args *structs.SnapshotRequest, in io.Reader) (io.ReadCloser, error) {

	conn, hc, err := pool.Dial(dc, addr)
	if err != nil {
		return nil, err
	}

	// keep will disarm the defer on success if we are returning the caller
	// our connection to stream the output.
	var keep bool
	defer func() {
		if !keep {
			conn.Close()
		}
	}()

	// Write the snapshot RPC byte to set the mode, then perform the
	// request.
	if _, err := conn.Write([]byte{byte(rpcSnapshot)}); err != nil {
		return nil, fmt.Errorf("failed to write stream type: %v", err)
	}

	// Push the header encoded as msgpack, then stream the input.
	enc := codec.NewEncoder(conn, &codec.MsgpackHandle{})
	if err := enc.Encode(&args); err != nil {
		return nil, fmt.Errorf("failed to encode request: %v", err)
	}
	if _, err := io.Copy(conn, in); err != nil {
		return nil, fmt.Errorf("failed to copy snapshot in: %v", err)
	}

	// Our RPC protocol requires support for a half-close in order to signal
	// the other side that they are done reading the stream, since we don't
	// know the size in advance. This saves us from having to buffer just to
	// calculate the size.
	if hc != nil {
		if err := hc.CloseWrite(); err != nil {
			return nil, fmt.Errorf("failed to half close snapshot connection: %v", err)
		}
	} else {
		return nil, fmt.Errorf("snapshot connection requires half-close support")
	}

	// Pull the header decoded as msgpack. The caller can continue to read
	// the conn to stream the remaining data.
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
