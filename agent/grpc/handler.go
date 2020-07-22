/*
Package grpc provides a Handler and client for agent gRPC connections.
*/
package grpc

import (
	"fmt"
	"net"

	"google.golang.org/grpc"
)

// NewHandler returns a Handler for addr.
func NewHandler(addr net.Addr) *Handler {
	conns := make(chan net.Conn)

	// We don't need to pass tls.Config to the server since it's multiplexed
	// behind the RPC listener, which already has TLS configured.
	srv := grpc.NewServer(
		grpc.StatsHandler(&statsHandler{}),
		grpc.StreamInterceptor((&activeStreamCounter{}).Intercept),
	)

	// TODO(streaming): add gRPC services to srv here

	return &Handler{
		conns:    conns,
		srv:      srv,
		listener: &chanListener{addr: addr, conns: conns},
	}
}

// Handler implements a handler for the rpc server listener, and the
// agent.Component interface for managing the lifecycle of the grpc.Server.
type Handler struct {
	conns    chan net.Conn
	srv      *grpc.Server
	listener *chanListener
}

// Handle the connection by sending it to a channel for the grpc.Server to receive.
func (h *Handler) Handle(conn net.Conn) {
	h.conns <- conn
}

func (h *Handler) Run() error {
	return h.srv.Serve(h.listener)
}

func (h *Handler) Shutdown() error {
	h.srv.Stop()
	return nil
}

// chanListener implements net.Listener for grpc.Server.
type chanListener struct {
	conns chan net.Conn
	addr  net.Addr
}

// Accept blocks until a connection is received from Handle, and then returns the
// connection. Accept implements part of the net.Listener interface for grpc.Server.
func (l *chanListener) Accept() (net.Conn, error) {
	return <-l.conns, nil
}

func (l *chanListener) Addr() net.Addr {
	return l.addr
}

// Close does nothing. The connections are managed by the caller.
func (l *chanListener) Close() error {
	return nil
}

// NoOpHandler implements the same methods as Handler, but performs no handling.
// It may be used in place of Handler to disable the grpc server.
type NoOpHandler struct {
	Logger Logger
}

type Logger interface {
	Error(string, ...interface{})
}

func (h NoOpHandler) Handle(conn net.Conn) {
	h.Logger.Error("gRPC conn opened but gRPC RPC is disabled, closing",
		"conn", logConn(conn))
	_ = conn.Close()
}

func (h NoOpHandler) Run() error {
	return nil
}

func (h NoOpHandler) Shutdown() error {
	return nil
}

// logConn is a local copy of github.com/hashicorp/memberlist.LogConn, to avoid
// a large dependency for a minor formatting function.
// logConn is used to keep log formatting consistent.
func logConn(conn net.Conn) string {
	if conn == nil {
		return "from=<unknown address>"
	}
	addr := conn.RemoteAddr()
	if addr == nil {
		return "from=<unknown address>"
	}

	return fmt.Sprintf("from=%s", addr.String())
}
