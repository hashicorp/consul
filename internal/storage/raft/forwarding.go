package raft

import (
	"context"
	"errors"
	"net"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/go-hclog"

	grpcinternal "github.com/hashicorp/consul/agent/grpc-internal"
	"github.com/hashicorp/consul/internal/storage"
	pbstorage "github.com/hashicorp/consul/proto/private/pbstorage"
)

// forwardingServer implements the gRPC forwarding service.
type forwardingServer struct {
	backend  *Backend
	listener *grpcinternal.Listener
}

var _ pbstorage.ForwardingServer = (*forwardingServer)(nil)

func newForwardingServer(backend *Backend) *forwardingServer {
	return &forwardingServer{
		backend: backend,

		// The address here doesn't actually matter. gRPC uses it as an identifier
		// internally, but we only bind the server to a single listener.
		listener: grpcinternal.NewListener(&net.TCPAddr{
			IP:   net.ParseIP("0.0.0.0"),
			Port: 0,
		}),
	}
}

// RoundTrip is the RPC endpoint called to forward an operation to the leader.
func (s *forwardingServer) RoundTrip(ctx context.Context, req *pbstorage.Request) (*pbstorage.Response, error) {
	switch req.Type {
	case pbstorage.RequestType_REQUEST_TYPE_READ:
		return s.handleRead(ctx, req)
	case pbstorage.RequestType_REQUEST_TYPE_LIST:
		return s.handleList(ctx, req)
	default:
		return s.raftApply(ctx, req)
	}
}

func (s *forwardingServer) handleRead(ctx context.Context, req *pbstorage.Request) (*pbstorage.Response, error) {
	res, err := s.backend.leaderRead(ctx, req.GetRead().Id)
	if err != nil {
		return nil, wrapError(err)
	}
	return &pbstorage.Response{
		Response: &pbstorage.Response_Read{
			Read: &pbstorage.ReadResponse{
				Resource: res,
			},
		},
	}, nil
}

func (s *forwardingServer) handleList(ctx context.Context, req *pbstorage.Request) (*pbstorage.Response, error) {
	listReq := req.GetList()
	res, err := s.backend.leaderList(ctx, storage.UnversionedTypeFrom(listReq.Type), listReq.Tenancy, listReq.NamePrefix)
	if err != nil {
		return nil, wrapError(err)
	}
	return &pbstorage.Response{
		Response: &pbstorage.Response_List{
			List: &pbstorage.ListResponse{
				Resources: res,
			},
		},
	}, nil
}

func (s *forwardingServer) raftApply(ctx context.Context, req *pbstorage.Request) (*pbstorage.Response, error) {
	msg, err := req.MarshalBinary()
	if err != nil {
		return nil, wrapError(err)
	}

	rsp, err := s.backend.handle.Apply(msg)
	if err != nil {
		return nil, wrapError(err)
	}

	switch t := rsp.(type) {
	case *pbstorage.Response:
		return t, nil
	default:
		return nil, status.Errorf(codes.Internal, "unexpected response from Raft apply: %T", rsp)
	}
}

func (s *forwardingServer) run(ctx context.Context) error {
	server := grpc.NewServer()
	pbstorage.RegisterForwardingServer(server, s)

	go func() {
		<-ctx.Done()
		server.Stop()
	}()

	return server.Serve(s.listener)
}

// forwardingClient is used to forward operations to the leader.
type forwardingClient struct {
	handle Handle
	logger hclog.Logger

	mu   sync.RWMutex
	conn *grpc.ClientConn
}

func newForwardingClient(h Handle, l hclog.Logger) *forwardingClient {
	return &forwardingClient{
		handle: h,
		logger: l,
	}
}

func (c *forwardingClient) leaderChanged() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return
	}

	if err := c.conn.Close(); err != nil {
		c.logger.Error("failed to close connection to previous leader", "error", err)
	}
	c.conn = nil
}

func (c *forwardingClient) getConn() (*grpc.ClientConn, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		return c.conn, nil
	}

	conn, err := c.handle.DialLeader()
	if err != nil {
		c.logger.Error("failed to dial leader", "error", err)
		return nil, err
	}
	c.conn = conn

	return conn, nil
}

func (c *forwardingClient) roundTrip(ctx context.Context, req *pbstorage.Request) (*pbstorage.Response, error) {
	conn, err := c.getConn()
	if err != nil {
		return nil, err
	}

	rsp, err := pbstorage.NewForwardingClient(conn).RoundTrip(ctx, req)
	return rsp, unwrapError(err)
}

var (
	errorToCode = map[error]codes.Code{
		// Note: OutOfRange is used to represent GroupVersionMismatchError, but is
		// handled specially in wrapError and unwrapError because it has extra details.
		storage.ErrNotFound:     codes.NotFound,
		storage.ErrCASFailure:   codes.Aborted,
		storage.ErrWrongUid:     codes.AlreadyExists,
		storage.ErrInconsistent: codes.FailedPrecondition,
	}

	codeToError = func() map[codes.Code]error {
		inverted := make(map[codes.Code]error, len(errorToCode))
		for k, v := range errorToCode {
			inverted[v] = k
		}
		return inverted
	}()
)

// wrapError converts the given error to a gRPC status to send over the wire.
func wrapError(err error) error {
	var gvm storage.GroupVersionMismatchError
	if errors.As(err, &gvm) {
		s, err := status.New(codes.OutOfRange, err.Error()).
			WithDetails(&pbstorage.GroupVersionMismatchErrorDetails{
				RequestedType: gvm.RequestedType,
				Stored:        gvm.Stored,
			})
		if err == nil {
			return s.Err()
		}
	}

	code, ok := errorToCode[err]
	if !ok {
		code = codes.Internal
	}
	return status.Error(code, err.Error())
}

// unwrapError converts the given gRPC status error back to a storage package
// error.
func unwrapError(err error) error {
	s, ok := status.FromError(err)
	if !ok {
		return err
	}

	for _, d := range s.Details() {
		if gvm, ok := d.(*pbstorage.GroupVersionMismatchErrorDetails); ok {
			return storage.GroupVersionMismatchError{
				RequestedType: gvm.RequestedType,
				Stored:        gvm.Stored,
			}
		}
	}

	unwrapped, ok := codeToError[s.Code()]
	if !ok {
		return err
	}
	return unwrapped
}
