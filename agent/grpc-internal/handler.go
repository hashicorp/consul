package internal

import (
	"fmt"
	"net"
	"time"

	"github.com/armon/go-metrics"

	agentmiddleware "github.com/hashicorp/consul/agent/grpc-middleware"

	middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"github.com/hashicorp/consul/agent/consul/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

var (
	metricsLabels = []metrics.Label{{
		Name:  "server_type",
		Value: "internal",
	}}
)

// NewHandler returns a gRPC server that accepts connections from Handle(conn).
// The register function will be called with the grpc.Server to register
// gRPC services with the server.
func NewHandler(logger Logger, addr net.Addr, register func(server *grpc.Server), metricsObj *metrics.Metrics, rateLimiter rate.RequestLimitsHandler) *Handler {
	if metricsObj == nil {
		metricsObj = metrics.Default()
	}

	// We don't need to pass tls.Config to the server since it's multiplexed
	// behind the RPC listener, which already has TLS configured.
	recoveryOpts := agentmiddleware.PanicHandlerMiddlewareOpts(logger)

	opts := []grpc.ServerOption{
		grpc.InTapHandle(agentmiddleware.ServerRateLimiterMiddleware(rateLimiter, agentmiddleware.NewPanicHandler(logger), logger)),
		grpc.StatsHandler(agentmiddleware.NewStatsHandler(metricsObj, metricsLabels)),
		middleware.WithUnaryServerChain(
			// Add middlware interceptors to recover in case of panics.
			recovery.UnaryServerInterceptor(recoveryOpts...),
		),
		middleware.WithStreamServerChain(
			// Add middlware interceptors to recover in case of panics.
			recovery.StreamServerInterceptor(recoveryOpts...),
			agentmiddleware.NewActiveStreamCounter(metricsObj, metricsLabels).Intercept,
		),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime: 15 * time.Second,
		}),
	}

	// We don't need to pass tls.Config to the server since it's multiplexed
	// behind the RPC listener, which already has TLS configured.
	srv := grpc.NewServer(opts...)
	register(srv)

	lis := &chanListener{addr: addr, conns: make(chan net.Conn), done: make(chan struct{})}
	return &Handler{srv: srv, listener: lis}
}

// Handler implements a handler for the rpc server listener, and the
// agent.Component interface for managing the lifecycle of the grpc.Server.
type Handler struct {
	srv      *grpc.Server
	listener *chanListener
}

// Handle the connection by sending it to a channel for the grpc.Server to receive.
func (h *Handler) Handle(conn net.Conn) {
	h.listener.conns <- conn
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
	done  chan struct{}
}

// Accept blocks until a connection is received from Handle, and then returns the
// connection. Accept implements part of the net.Listener interface for grpc.Server.
func (l *chanListener) Accept() (net.Conn, error) {
	select {
	case c := <-l.conns:
		return c, nil
	case <-l.done:
		return nil, &net.OpError{
			Op:   "accept",
			Net:  l.addr.Network(),
			Addr: l.addr,
			Err:  fmt.Errorf("listener closed"),
		}
	}
}

func (l *chanListener) Addr() net.Addr {
	return l.addr
}

func (l *chanListener) Close() error {
	close(l.done)
	return nil
}

// NoOpHandler implements the same methods as Handler, but performs no handling.
// It may be used in place of Handler to disable the grpc server.
type NoOpHandler struct {
	Logger Logger
}

type Logger interface {
	Error(string, ...interface{})
	Warn(string, ...interface{})
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
