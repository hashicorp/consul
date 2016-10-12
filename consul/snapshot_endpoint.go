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
	// forward is a helper to dial up another server using the pool and
	// relay the request.
	forward := func(dc string, addr net.Addr) (io.ReadCloser, error) {
		conn, err := s.connPool.Dial(dc, addr)
		if err != nil {
			return nil, err
		}
		if err := SnapshotRPC(conn, args, in); err != nil {
			conn.Close()
			return nil, err
		}
		return conn, nil
	}

	// We may need to forward if this is not our datacenter or we are not
	// the leader.
	if dc := args.Datacenter; dc != s.config.Datacenter {
		server, ok := s.getRemoteServer(dc)
		if !ok {
			return nil, structs.ErrNoDCPath
		}
		return forward(dc, server.Addr)
	} else if isLeader, server := s.getLeader(); !isLeader {
		if server == nil {
			return nil, structs.ErrNoLeader
		}
		return forward(dc, server.Addr)
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
// request to a remote server. It should be given a fresh connection for each
// request, sends the request header, and then streams in any data from the
// reader (for a restore). It will then parse the received response header, and
// if there's no error you should read the remainder of the data in the conn to
// get the streaming part of the response.
//
// If the given connection is a TCP connection, it will be closed for writes
// after the data has been streamed in from the reader. This propagates the EOF
// so the remote side knows to stop reading, since we don't have the size of the
// payload ahead of time.
func SnapshotRPC(conn net.Conn, args *structs.SnapshotRequest, in io.Reader) error {
	// Write the snapshot RPC byte to set the mode, then perform the
	// request.
	if _, err := conn.Write([]byte{byte(rpcSnapshot)}); err != nil {
		return err
	}

	// Push the header encoded as JSON, then stream the input.
	enc := codec.NewEncoder(conn, &codec.MsgpackHandle{})
	if err := enc.Encode(&args); err != nil {
		return fmt.Errorf("failed to encode request: %v", err)
	}
	if _, err := io.Copy(conn, in); err != nil {
		return fmt.Errorf("failed to copy snapshot in: %v", err)
	}

	// If this is a TCP connection, we close the write side since this is
	// the only way to get an EOF on the other side to signal that we are
	// done. This is a bit jank but it beats having to know the size in
	// advance on the receiving end.
	if tc, ok := conn.(*net.TCPConn); ok {
		if err := tc.CloseWrite(); err != nil {
			return fmt.Errorf("failed to half close snapshot TCP connection: %v", err)
		}
	}

	// Pull the header decoded as JSON. The caller can continue to read
	// the conn to stream the remaining data.
	var resp structs.SnapshotResponse
	dec := codec.NewDecoder(conn, &codec.MsgpackHandle{})
	if err := dec.Decode(&resp); err != nil {
		return fmt.Errorf("failed to decode response: %v", err)
	}
	if resp.Error != "" {
		return errors.New(resp.Error)
	}

	return nil
}
