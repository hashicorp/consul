package rate

import (
	"bytes"
	"context"
	"net"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/agent/consul/multilimiter"
)

func TestHandler(t *testing.T) {
	var (
		rpcName    = "Foo.Bar"
		sourceAddr = net.TCPAddrFromAddrPort(netip.MustParseAddrPort("1.2.3.4:5678"))
	)

	type limitCheck struct {
		limit multilimiter.LimitedEntity
		allow bool
	}
	testCases := map[string]struct {
		op         Operation
		globalMode Mode
		checks     []limitCheck
		isLeader   bool
		expectErr  error
		expectLog  bool
	}{
		"operation exempt from limiting": {
			op: Operation{
				Type:       OperationTypeExempt,
				Name:       rpcName,
				SourceAddr: sourceAddr,
			},
			globalMode: ModeEnforcing,
			checks:     []limitCheck{},
			expectErr:  nil,
			expectLog:  false,
		},
		"global write limit disabled": {
			op: Operation{
				Type:       OperationTypeWrite,
				Name:       rpcName,
				SourceAddr: sourceAddr,
			},
			globalMode: ModeDisabled,
			checks:     []limitCheck{},
			expectErr:  nil,
			expectLog:  false,
		},
		"global write limit within allowance": {
			op: Operation{
				Type:       OperationTypeWrite,
				Name:       rpcName,
				SourceAddr: sourceAddr,
			},
			globalMode: ModeEnforcing,
			checks: []limitCheck{
				{limit: globalWrite, allow: true},
			},
			expectErr: nil,
			expectLog: false,
		},
		"global write limit exceeded (permissive)": {
			op: Operation{
				Type:       OperationTypeWrite,
				Name:       rpcName,
				SourceAddr: sourceAddr,
			},
			globalMode: ModePermissive,
			checks: []limitCheck{
				{limit: globalWrite, allow: false},
			},
			expectErr: nil,
			expectLog: true,
		},
		"global write limit exceeded (enforcing, leader)": {
			op: Operation{
				Type:       OperationTypeWrite,
				Name:       rpcName,
				SourceAddr: sourceAddr,
			},
			globalMode: ModeEnforcing,
			checks: []limitCheck{
				{limit: globalWrite, allow: false},
			},
			isLeader:  true,
			expectErr: ErrRetryLater,
			expectLog: true,
		},
		"global write limit exceeded (enforcing, follower)": {
			op: Operation{
				Type:       OperationTypeWrite,
				Name:       rpcName,
				SourceAddr: sourceAddr,
			},
			globalMode: ModeEnforcing,
			checks: []limitCheck{
				{limit: globalWrite, allow: false},
			},
			isLeader:  false,
			expectErr: ErrRetryElsewhere,
			expectLog: true,
		},
		"global read limit disabled": {
			op: Operation{
				Type:       OperationTypeRead,
				Name:       rpcName,
				SourceAddr: sourceAddr,
			},
			globalMode: ModeDisabled,
			checks:     []limitCheck{},
			expectErr:  nil,
			expectLog:  false,
		},
		"global read limit within allowance": {
			op: Operation{
				Type:       OperationTypeRead,
				Name:       rpcName,
				SourceAddr: sourceAddr,
			},
			globalMode: ModeEnforcing,
			checks: []limitCheck{
				{limit: globalRead, allow: true},
			},
			expectErr: nil,
			expectLog: false,
		},
		"global read limit exceeded (permissive)": {
			op: Operation{
				Type:       OperationTypeRead,
				Name:       rpcName,
				SourceAddr: sourceAddr,
			},
			globalMode: ModePermissive,
			checks: []limitCheck{
				{limit: globalRead, allow: false},
			},
			expectErr: nil,
			expectLog: true,
		},
		"global read limit exceeded (enforcing, leader)": {
			op: Operation{
				Type:       OperationTypeRead,
				Name:       rpcName,
				SourceAddr: sourceAddr,
			},
			globalMode: ModeEnforcing,
			checks: []limitCheck{
				{limit: globalRead, allow: false},
			},
			isLeader:  true,
			expectErr: ErrRetryLater,
			expectLog: true,
		},
		"global read limit exceeded (enforcing, follower)": {
			op: Operation{
				Type:       OperationTypeRead,
				Name:       rpcName,
				SourceAddr: sourceAddr,
			},
			globalMode: ModeEnforcing,
			checks: []limitCheck{
				{limit: globalRead, allow: false},
			},
			isLeader:  false,
			expectErr: ErrRetryElsewhere,
			expectLog: true,
		},
	}
	for desc, tc := range testCases {
		t.Run(desc, func(t *testing.T) {
			limiter := newMockLimiter(t)
			limiter.On("UpdateConfig", mock.Anything, mock.Anything).Return()
			for _, c := range tc.checks {
				limiter.On("Allow", c.limit).Return(c.allow)
			}

			delegate := NewMockHandlerDelegate(t)
			delegate.On("IsLeader").Return(tc.isLeader).Maybe()

			var output bytes.Buffer
			logger := hclog.NewInterceptLogger(&hclog.LoggerOptions{
				Level:  hclog.Trace,
				Output: &output,
			})

			handler := NewHandler(
				HandlerConfig{
					GlobalMode: tc.globalMode,
					makeLimiter: func(multilimiter.Config) multilimiter.RateLimiter {
						return limiter
					},
				},
				logger,
			)
			handler.RegisterDelegate(delegate)

			require.Equal(t, tc.expectErr, handler.Allow(tc.op))

			if tc.expectLog {
				require.Contains(t, output.String(), "RPC exceeded allowed rate limit")
			} else {
				require.Zero(t, output.Len(), "expected no logs to be emitted")
			}
		})
	}
}

var _ multilimiter.RateLimiter = (*mockLimiter)(nil)

func newMockLimiter(t *testing.T) *mockLimiter {
	l := &mockLimiter{}
	l.Mock.Test(t)

	t.Cleanup(func() { l.AssertExpectations(t) })

	return l
}

type mockLimiter struct {
	mock.Mock
}

func (m *mockLimiter) Allow(v multilimiter.LimitedEntity) bool { return m.Called(v).Bool(0) }
func (m *mockLimiter) Run(ctx context.Context)                 { m.Called(ctx) }
func (m *mockLimiter) UpdateConfig(cfg multilimiter.LimiterConfig, prefix []byte) {
	m.Called(cfg, prefix)
}
