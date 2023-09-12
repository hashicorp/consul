package resolver

import (
	"fmt"
	"sync"

	"google.golang.org/grpc/resolver"
)

// registry of ServerResolverBuilder. This type exists because grpc requires that
// resolvers are registered globally before any requests are made. This is
// incompatible with our resolver implementation and testing strategy, which
// requires a different Resolver for each test.
type registry struct {
	lock        sync.RWMutex
	byAuthority map[string]*ServerResolverBuilder
}

func (r *registry) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()
	//nolint:staticcheck
	res, ok := r.byAuthority[target.Authority]
	if !ok {
		//nolint:staticcheck
		return nil, fmt.Errorf("no resolver registered for %v", target.Authority)
	}
	return res.Build(target, cc, opts)
}

func (r *registry) Scheme() string {
	return "consul"
}

var _ resolver.Builder = (*registry)(nil)

var reg = &registry{byAuthority: make(map[string]*ServerResolverBuilder)}

func init() {
	resolver.Register(reg)
}

// Register a ServerResolverBuilder with the global registry.
func Register(res *ServerResolverBuilder) {
	reg.lock.Lock()
	defer reg.lock.Unlock()
	reg.byAuthority[res.Authority()] = res
}

// Deregister the ServerResolverBuilder associated with the authority. Only used
// for testing.
func Deregister(authority string) {
	reg.lock.Lock()
	defer reg.lock.Unlock()
	delete(reg.byAuthority, authority)
}
