package balancer

import (
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/resolver"
)

func init() {
	balancer.Register(newCustomPickfirstBuilder())
}

// logger is referenced in pickfirst.go.
// The gRPC library uses the same component name.
var logger = grpclog.Component("balancer")

func newCustomPickfirstBuilder() balancer.Builder {
	return &customPickfirstBuilder{}
}

type customPickfirstBuilder struct{}

func (*customPickfirstBuilder) Build(cc balancer.ClientConn, opt balancer.BuildOptions) balancer.Balancer {
	return &customPickfirstBalancer{
		pickfirstBalancer: pickfirstBalancer{cc: cc},
	}
}

func (*customPickfirstBuilder) Name() string {
	return "pick_first_custom"
}

// customPickfirstBalancer overrides UpdateClientConnState of pickfirstBalancer.
type customPickfirstBalancer struct {
	pickfirstBalancer

	activeAddr resolver.Address
}

// shim since resolver.Address.Equal method does not exist in older gRPC versions.
func equalAddr(a, o resolver.Address) bool {
	//nolint:staticcheck
	return a.Addr == o.Addr && a.ServerName == o.ServerName &&
		a.Type == o.Type && a.Metadata == o.Metadata
}

func (b *customPickfirstBalancer) UpdateClientConnState(state balancer.ClientConnState) error {
	for _, a := range state.ResolverState.Addresses {
		// This hack preserves an existing behavior in our client-side
		// load balancing where if the first address in a shuffled list
		// of addresses matched the currently connected address, it would
		// be an effective no-op.
		if equalAddr(b.activeAddr, a) {
			break
		}

		// Attempt to make a new SubConn with a single address so we can
		// track a successful connection explicitly. If we were to pass
		// a list of addresses, we cannot assume the first address was
		// successful and there is no way to extract the connected address.
		sc, err := b.cc.NewSubConn([]resolver.Address{a}, balancer.NewSubConnOptions{})
		if err != nil {
			logger.Warningf("balancer.customPickfirstBalancer: failed to create new SubConn: %v", err)
			continue
		}

		if b.subConn != nil {
			b.cc.RemoveSubConn(b.subConn)
		}

		// Copy-pasted from pickfirstBalancer.UpdateClientConnState.
		{
			b.subConn = sc
			b.state = connectivity.Idle
			b.cc.UpdateState(balancer.State{
				ConnectivityState: connectivity.Idle,
				Picker:            &picker{result: balancer.PickResult{SubConn: b.subConn}},
			})
			b.subConn.Connect()
		}

		b.activeAddr = a

		// We now have a new subConn with one address.
		// Break the loop and call UpdateClientConnState
		// with the full set of addresses.
		break
	}

	// This will load the full set of addresses but leave the
	// newly created subConn alone.
	return b.pickfirstBalancer.UpdateClientConnState(state)
}
