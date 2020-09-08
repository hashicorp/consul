package consul

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/stream"
	agentgrpc "github.com/hashicorp/consul/agent/grpc"
	"github.com/hashicorp/consul/agent/subscribe"
	"google.golang.org/grpc"
)

type subscribeBackend struct {
	srv      *Server
	connPool *agentgrpc.ClientConnPool
}

// TODO: refactor Resolve methods to an ACLBackend that can be used by all
// the endpoints.
func (s subscribeBackend) ResolveToken(token string) (acl.Authorizer, error) {
	return s.srv.ResolveToken(token)
}

var _ subscribe.Backend = (*subscribeBackend)(nil)

// Forward requests to a remote datacenter by calling f if the target dc does not
// match the config. Does nothing but return handled=false if dc is not specified,
// or if it matches the Datacenter in config.
//
// TODO: extract this so that it can be used with other grpc services.
func (s subscribeBackend) Forward(dc string, f func(*grpc.ClientConn) error) (handled bool, err error) {
	if dc == "" || dc == s.srv.config.Datacenter {
		return false, nil
	}
	conn, err := s.connPool.ClientConn(dc)
	if err != nil {
		return false, err
	}
	return true, f(conn)
}

func (s subscribeBackend) Subscribe(req *stream.SubscribeRequest) (*stream.Subscription, error) {
	return s.srv.fsm.State().EventPublisher().Subscribe(req)
}
