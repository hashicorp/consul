package consul

import (
	"google.golang.org/grpc"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/rpc/subscribe"
	"github.com/hashicorp/consul/agent/structs"
)

type subscribeBackend struct {
	srv      *Server
	connPool GRPCClientConner
}

// TODO: refactor Resolve methods to an ACLBackend that can be used by all
// the endpoints.
func (s subscribeBackend) ResolveTokenAndDefaultMeta(
	token string,
	entMeta *structs.EnterpriseMeta,
	authzContext *acl.AuthorizerContext,
) (acl.Authorizer, error) {
	return s.srv.ResolveTokenAndDefaultMeta(token, entMeta, authzContext)
}

var _ subscribe.Backend = (*subscribeBackend)(nil)

// Forward requests to a remote datacenter by calling f if the target dc does not
// match the config. Does nothing but return handled=false if dc is not specified,
// or if it matches the Datacenter in config.
//
// TODO: extract this so that it can be used with other grpc services.
// TODO: rename to ForwardToDC
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
