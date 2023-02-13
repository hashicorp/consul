// package rate implements server-side RPC rate limiting.
package rate

import (
	"context"
	"errors"
	"fmt"
	"net"
	"reflect"
	"sync/atomic"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/agent/consul/multilimiter"
)

var (
	// ErrRetryElsewhere indicates that the operation was not allowed because the
	// rate limit was exhausted, but may succeed on a different server.
	//
	// Results in a RESOURCE_EXHAUSTED or "429 Too Many Requests" response.
	ErrRetryElsewhere = errors.New("rate limit exceeded, try again later or against a different server")

	// ErrRetryLater indicates that the operation was not allowed because the rate
	// limit was exhausted, and trying a different server won't help (e.g. because
	// the operation can only be performed on the leader).
	//
	// Results in an UNAVAILABLE or "503 Service Unavailable" response.
	ErrRetryLater = errors.New("rate limit exceeded for operation that can only be performed by the leader, try again later")
)

// Mode determines the action that will be taken when a rate limit has been
// exhausted (e.g. log and allow, or reject).
type Mode int

const (
	// ModeDisabled causes rate limiting to be bypassed.
	ModeDisabled Mode = iota

	// ModePermissive causes the handler to log the rate-limited operation but
	// still allow it to proceed.
	ModePermissive

	// ModeEnforcing causes the handler to reject the rate-limited operation.
	ModeEnforcing
)

var modeToName = map[Mode]string{
	ModeDisabled:   "disabled",
	ModeEnforcing:  "enforcing",
	ModePermissive: "permissive",
}
var modeFromName = func() map[string]Mode {
	vals := map[string]Mode{
		"": ModeDisabled,
	}
	for k, v := range modeToName {
		vals[v] = k
	}
	return vals
}()

func (m Mode) String() string {
	return modeToName[m]
}

// RequestLimitsModeFromName will unmarshal the string form of a configMode.
func RequestLimitsModeFromName(name string) (Mode, bool) {
	s, ok := modeFromName[name]
	return s, ok
}

// RequestLimitsModeFromNameWithDefault will unmarshal the string form of a configMode.
func RequestLimitsModeFromNameWithDefault(name string) Mode {
	s, ok := modeFromName[name]
	if !ok {
		return ModePermissive
	}
	return s
}

// OperationType is the type of operation the client is attempting to perform.
type OperationType int

const (
	// OperationTypeRead represents a read operation.
	OperationTypeRead OperationType = iota

	// OperationTypeWrite represents a write operation.
	OperationTypeWrite

	// OperationTypeExempt represents an operation that is exempt from rate-limiting.
	OperationTypeExempt
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

//go:generate mockery --name RequestLimitsHandler --inpackage
type RequestLimitsHandler interface {
	Run(ctx context.Context)
	Allow(op Operation) error
	UpdateConfig(cfg HandlerConfig)
	Register(leaderStatusProvider LeaderStatusProvider)
}

// Handler enforces rate limits for incoming RPCs.
type Handler struct {
	cfg                  *atomic.Pointer[HandlerConfig]
	leaderStatusProvider LeaderStatusProvider

	limiter multilimiter.RateLimiter

	logger hclog.Logger
}

type HandlerConfig struct {
	multilimiter.Config

	// GlobalMode configures the action that will be taken when a global rate-limit
	// has been exhausted.
	//
	// Note: in the future there'll be a separate Mode for IP-based limits.
	GlobalMode Mode

	// GlobalWriteConfig configures the global rate limiter for write operations.
	GlobalWriteConfig multilimiter.LimiterConfig

	// GlobalReadConfig configures the global rate limiter for read operations.
	GlobalReadConfig multilimiter.LimiterConfig
}

//go:generate mockery --name LeaderStatusProvider --inpackage --filename mock_LeaderStatusProvider_test.go
type LeaderStatusProvider interface {
	// IsLeader is used to determine whether the operation is being performed
	// against the cluster leader, such that if it can _only_ be performed by
	// the leader (e.g. write operations) we don't tell clients to retry against
	// a different server.
	IsLeader() bool
}

func NewHandlerWithLimiter(
	cfg HandlerConfig,
	limiter multilimiter.RateLimiter,
	logger hclog.Logger) *Handler {

	limiter.UpdateConfig(cfg.GlobalWriteConfig, globalWrite)
	limiter.UpdateConfig(cfg.GlobalReadConfig, globalRead)

	h := &Handler{
		cfg:     new(atomic.Pointer[HandlerConfig]),
		limiter: limiter,
		logger:  logger,
	}
	h.cfg.Store(&cfg)

	return h
}

// NewHandler creates a new RPC rate limit handler.
func NewHandler(cfg HandlerConfig, logger hclog.Logger) *Handler {
	limiter := multilimiter.NewMultiLimiter(cfg.Config)
	return NewHandlerWithLimiter(cfg, limiter, logger)
}

// Run the limiter cleanup routine until the given context is canceled.
//
// Note: this starts a goroutine.
func (h *Handler) Run(ctx context.Context) {
	h.limiter.Run(ctx)
}

// Allow returns an error if the given operation is not allowed to proceed
// because of an exhausted rate-limit.
func (h *Handler) Allow(op Operation) error {

	if h.leaderStatusProvider == nil {
		h.logger.Error("leaderStatusProvider required to be set via Register(). bailing on rate limiter")
		return nil
		// TODO: panic and make sure to use the server's recovery handler
		// panic("leaderStatusProvider required to be set via Register(..)")
	}

	cfg := h.cfg.Load()
	if cfg.GlobalMode == ModeDisabled {
		return nil
	}

	for _, l := range h.limits(op) {
		if l.mode == ModeDisabled {
			continue
		}

		if h.limiter.Allow(l.ent) {
			continue
		}

		// TODO(NET-1382): is this the correct log-level?

		enforced := l.mode == ModeEnforcing
		h.logger.Debug("RPC exceeded allowed rate limit",
			"rpc", op.Name,
			"source_addr", op.SourceAddr,
			"limit_type", l.desc,
			"limit_enforced", enforced,
		)

		metrics.IncrCounterWithLabels([]string{"rpc", "rate_limit", "exceeded"}, 1, []metrics.Label{
			{
				Name:  "limit_type",
				Value: l.desc,
			},
			{
				Name:  "op",
				Value: op.Name,
			},
			{
				Name:  "mode",
				Value: l.mode.String(),
			},
		})

		if enforced {
			if h.leaderStatusProvider.IsLeader() && op.Type == OperationTypeWrite {
				return ErrRetryLater
			}
			return ErrRetryElsewhere
		}
	}
	return nil
}

func (h *Handler) UpdateConfig(cfg HandlerConfig) {
	existingCfg := h.cfg.Load()
	h.cfg.Store(&cfg)
	if reflect.DeepEqual(existingCfg, cfg) {
		h.logger.Warn("UpdateConfig called but configuration has not changed.  Skipping updating the server rate limiter configuration.")
		return
	}
	if !reflect.DeepEqual(existingCfg.GlobalWriteConfig, cfg.GlobalWriteConfig) {
		h.limiter.UpdateConfig(cfg.GlobalWriteConfig, globalWrite)
	}
	if !reflect.DeepEqual(existingCfg.GlobalReadConfig, cfg.GlobalReadConfig) {
		h.limiter.UpdateConfig(cfg.GlobalReadConfig, globalRead)
	}
}

func (h *Handler) Register(leaderStatusProvider LeaderStatusProvider) {
	h.leaderStatusProvider = leaderStatusProvider
}

type limit struct {
	mode Mode
	ent  multilimiter.LimitedEntity
	desc string
}

// limits returns the limits to check for the given operation (e.g. global +
// ip-based + tenant-based).
func (h *Handler) limits(op Operation) []limit {
	limits := make([]limit, 0)

	if global := h.globalLimit(op); global != nil {
		limits = append(limits, *global)
	}

	return limits
}

func (h *Handler) globalLimit(op Operation) *limit {
	if op.Type == OperationTypeExempt {
		return nil
	}
	cfg := h.cfg.Load()

	lim := &limit{mode: cfg.GlobalMode}
	switch op.Type {
	case OperationTypeRead:
		lim.desc = "global/read"
		lim.ent = globalRead
	case OperationTypeWrite:
		lim.desc = "global/write"
		lim.ent = globalWrite
	default:
		panic(fmt.Sprintf("unknown operation type %d", op.Type))
	}
	return lim
}

var (
	// globalWrite identifies the global rate limit applied to write operations.
	globalWrite = globalLimit("global.write")

	// globalRead identifies the global rate limit applied to read operations.
	globalRead = globalLimit("global.read")
)

// globalLimit represents a limit that applies to all writes or reads.
type globalLimit []byte

// Key satisfies the multilimiter.LimitedEntity interface.
func (prefix globalLimit) Key() multilimiter.KeyType {
	return multilimiter.Key(prefix, nil)
}

// NullRequestLimitsHandler returns a RequestLimitsHandler that allows every operation.
func NullRequestLimitsHandler() RequestLimitsHandler {
	return nullRequestLimitsHandler{}
}

type nullRequestLimitsHandler struct{}

func (nullRequestLimitsHandler) Allow(Operation) error { return nil }

func (nullRequestLimitsHandler) Run(ctx context.Context) {}

func (nullRequestLimitsHandler) UpdateConfig(cfg HandlerConfig) {}

func (nullRequestLimitsHandler) Register(leaderStatusProvider LeaderStatusProvider) {}
