// package rate implements server-side RPC rate limiting.
package rate

import (
	"context"
	"errors"
	"net"
	"sync/atomic"

	"github.com/hashicorp/consul/agent/consul/rate/multilimiter"
)

var (
	// ErrRetryElsewhere indicates that the operation was not allowed because the
	// rate limit was exhausted, but may succeed on a different server.
	//
	// Results in a RESOURCE_EXHAUSTED or "429 Too Many Requests" response.
	ErrRetryElsewhere = errors.New("rate limit exceeded, try a different server")

	// ErrRetryLater indicates that the operation was not allowed because the rate
	// limit was exhausted, and trying a different server won't help (e.g. because
	// the operation can only be performed on the leader).
	//
	// Results in an UNAVAILABLE or "503 Service Unavailable" response.
	ErrRetryLater = errors.New("rate limit exceeded, try again later")
)

// Mode determines the action that will be taken when a rate limit has been
// exhausted (e.g. log and allow, or reject).
type Mode int

const (
	// ModePermissive causes the handler to log the rate-limited operation but
	// still allow it to proceed.
	ModePermissive Mode = iota

	// ModeEnforce causes the handler to reject the rate-limted operation.
	ModeEnforce
)

// OperationType is the type of operation the client is attempting to perform.
type OperationType int

const (
	// OperationTypeRead represents a read operation.
	OperationTypeRead OperationType = iota

	// OperationTypeWrite represents a write operation.
	OperationTypeWrite
)

// Operation the client is attempting to perform.
type Operation struct {
	// Name of the RPC endpoint (e.g. "Foo.Bar" for net/rpc and "/foo.service/Bar" for gRPC).
	Name string

	// SourceAddr is the client's (or forwarding server's) IP address.
	SourceAddr net.Addr

	// Type of operation to be performed (e.g. read or write).
	Type OperationType
}

// Handler enforces rate limits for incoming RPCs.
type Handler struct {
	cfg      *atomic.Pointer[HandlerConfig]
	delegate HandlerDelegate

	limiter multilimiter.RateLimiter
}

type HandlerConfig struct {
	multilimiter.Config

	// GlobalMode configures the action that will be taken when a global rate-limit
	// has been exhausted.
	//
	// Note: in the future there'll be a separate Mode for IP-based limits.
	GlobalMode Mode
}

type HandlerDelegate interface {
	// IsLeader is used to determine whether the operation is being performed
	// against the cluster leader, such that if it can _only_ be performed by
	// the leader (e.g. write operations) we don't tell clients to retry against
	// a different server.
	IsLeader() bool
}

// NewHandler creates a new RPC rate limit handler.
func NewHandler(cfg HandlerConfig, delegate HandlerDelegate) *Handler {
	h := &Handler{
		cfg:      new(atomic.Pointer[HandlerConfig]),
		delegate: delegate,
		limiter:  multilimiter.NewMultiLimiter(cfg.Config),
	}
	h.cfg.Store(&cfg)
	return h
}

// Run the limiter cleanup routine until the given context is canceled.
//
// Note: this starts a goroutine.
func (h *Handler) Run(ctx context.Context) {
	h.limiter.Run(ctx)
}

// Allow returns an error if the given operation is now allowed to proceed
// because of an exhausted rate-limit.
func (h *Handler) Allow(op Operation) error {
	// TODO(NET-1383): actually implement the rate limiting logic.
	return nil
}

// TODO(NET-1379): call this on `consul reload`.
func (h *Handler) UpdateConfig(cfg HandlerConfig) {
	h.cfg.Store(&cfg)
	h.limiter.UpdateConfig(cfg.Config)
}
