package connect

// import (
// 	"context"
// 	"crypto/tls"
// 	"fmt"
// 	"math/rand"
// 	"net"

// 	"github.com/hashicorp/consul/api"
// )

// // CertStatus indicates whether the Client currently has valid certificates for
// // incoming and outgoing connections.
// type CertStatus int

// const (
// 	// CertStatusUnknown is the zero value for CertStatus which may be returned
// 	// when a watch channel is closed on shutdown. It has no other meaning.
// 	CertStatusUnknown CertStatus = iota

// 	// CertStatusOK indicates the client has valid certificates and trust roots to
// 	// Authenticate incoming and outgoing connections.
// 	CertStatusOK

// 	// CertStatusPending indicates the client is waiting to be issued initial
// 	// certificates, or that it's certificates have expired and it's waiting to be
// 	// issued new ones. In this state all incoming and outgoing connections will
// 	// fail.
// 	CertStatusPending
// )

// func (s CertStatus) String() string {
// 	switch s {
// 	case CertStatusOK:
// 		return "OK"
// 	case CertStatusPending:
// 		return "pending"
// 	case CertStatusUnknown:
// 		fallthrough
// 	default:
// 		return "unknown"
// 	}
// }

// // Client is the interface a basic client implementation must support.
// type Client interface {
// 	// TODO(banks): build this and test it
// 	// CertStatus returns the current status of the client's certificates. It can
// 	// be used to determine if the Client is able to service requests at the
// 	// current time.
// 	//CertStatus() CertStatus

// 	// TODO(banks): build this and test it
// 	// WatchCertStatus returns a channel that is notified on all status changes.
// 	// Note that a message on the channel isn't guaranteed to be different so it's
// 	// value should be inspected. During Client shutdown the channel will be
// 	// closed returning a zero type which is equivalent to CertStatusUnknown.
// 	//WatchCertStatus() <-chan CertStatus

// 	// ServerTLSConfig returns the *tls.Config to be used when creating a TCP
// 	// listener that should accept Connect connections. It is likely that at
// 	// startup the tlsCfg returned will not be immediately usable since
// 	// certificates are typically fetched from the agent asynchronously. In this
// 	// case it's still safe to listen with the provided config, but auth failures
// 	// will occur until initial certificate discovery is complete. In general at
// 	// any time it is possible for certificates to expire before new replacements
// 	// have been issued due to local network errors so the server may not actually
// 	// have a working certificate configuration at any time, however as soon as
// 	// valid certs can be issued it will automatically start working again so
// 	// should take no action.
// 	ServerTLSConfig() (*tls.Config, error)

// 	// DialService opens a new connection to the named service registered in
// 	// Consul. It will perform service discovery to find healthy instances. If
// 	// there is an error during connection it is returned and the caller may call
// 	// again. The client implementation makes a best effort to make consecutive
// 	// Dials against different instances either by randomising the list and/or
// 	// maintaining a local memory of which instances recently failed. If the
// 	// context passed times out before connection is established and verified an
// 	// error is returned.
// 	DialService(ctx context.Context, namespace, name string) (net.Conn, error)

// 	// DialPreparedQuery opens a new connection by executing the named Prepared
// 	// Query against the local Consul agent, and picking one of the returned
// 	// instances to connect to. It will perform service discovery with the same
// 	// semantics as DialService.
// 	DialPreparedQuery(ctx context.Context, namespace, name string) (net.Conn, error)
// }

// /*

// Maybe also convenience wrappers for:
// 	- listening TLS conn with right config
// 	- http.ListenAndServeTLS equivalent

// */

// // AgentClient is the primary implementation of a connect.Client which
// // communicates with the local Consul agent.
// type AgentClient struct {
// 	agent  *api.Client
// 	tlsCfg *ReloadableTLSConfig
// }

// // NewClient returns an AgentClient to allow consuming and providing
// // Connect-enabled network services.
// func NewClient(agent *api.Client) Client {
// 	// TODO(banks): hook up fetching certs from Agent and updating tlsCfg on cert
// 	// delivery/change. Perhaps need to make
// 	return &AgentClient{
// 		agent:  agent,
// 		tlsCfg: NewReloadableTLSConfig(defaultTLSConfig()),
// 	}
// }

// // NewInsecureDevClientWithLocalCerts returns an AgentClient that will still do
// // service discovery via the local agent but will use externally provided
// // certificates and skip authorization. This is intended just for development
// // and must not be used ever in production.
// func NewInsecureDevClientWithLocalCerts(agent *api.Client, caFile, certFile,
// 	keyFile string) (Client, error) {

// 	cfg, err := devTLSConfigFromFiles(caFile, certFile, keyFile)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return &AgentClient{
// 		agent:  agent,
// 		tlsCfg: NewReloadableTLSConfig(cfg),
// 	}, nil
// }

// // ServerTLSConfig implements Client
// func (c *AgentClient) ServerTLSConfig() (*tls.Config, error) {
// 	return c.tlsCfg.ServerTLSConfig(), nil
// }

// // DialService implements Client
// func (c *AgentClient) DialService(ctx context.Context, namespace,
// 	name string) (net.Conn, error) {
// 	return c.dial(ctx, "service", namespace, name)
// }

// // DialPreparedQuery implements Client
// func (c *AgentClient) DialPreparedQuery(ctx context.Context, namespace,
// 	name string) (net.Conn, error) {
// 	return c.dial(ctx, "prepared_query", namespace, name)
// }

// func (c *AgentClient) dial(ctx context.Context, discoveryType, namespace,
// 	name string) (net.Conn, error) {

// 	svcs, err := c.discoverInstances(ctx, discoveryType, namespace, name)
// 	if err != nil {
// 		return nil, err
// 	}

// 	svc, err := c.pickInstance(svcs)
// 	if err != nil {
// 		return nil, err
// 	}
// 	if svc == nil {
// 		return nil, fmt.Errorf("no healthy services discovered")
// 	}

// 	// OK we have a service we can dial! We need a ClientAuther that will validate
// 	// the connection is legit.

// 	// TODO(banks): implement ClientAuther properly to actually verify connected
// 	// cert matches the expected service/cluster etc. based on svc.
// 	auther := &ClientAuther{}
// 	tlsConfig := c.tlsCfg.TLSConfig(auther)

// 	// Resolve address TODO(banks): I expected this to happen magically in the
// 	// agent at registration time if I register with no explicit address but
// 	// apparently doesn't. This is a quick hack to make it work for now, need to
// 	// see if there is a better shared code path for doing this.
// 	addr := svc.Service.Address
// 	if addr == "" {
// 		addr = svc.Node.Address
// 	}
// 	var dialer net.Dialer
// 	tcpConn, err := dialer.DialContext(ctx, "tcp",
// 		fmt.Sprintf("%s:%d", addr, svc.Service.Port))
// 	if err != nil {
// 		return nil, err
// 	}

// 	tlsConn := tls.Client(tcpConn, tlsConfig)
// 	err = tlsConn.Handshake()
// 	if err != nil {
// 		tlsConn.Close()
// 		return nil, err
// 	}

// 	return tlsConn, nil
// }

// // pickInstance returns an instance from the given list to try to connect to. It
// // may be made pluggable later, for now it just picks a random one regardless of
// // whether the list is already shuffled.
// func (c *AgentClient) pickInstance(svcs []*api.ServiceEntry) (*api.ServiceEntry, error) {
// 	if len(svcs) < 1 {
// 		return nil, nil
// 	}
// 	idx := rand.Intn(len(svcs))
// 	return svcs[idx], nil
// }

// // discoverInstances returns all instances for the given discoveryType,
// // namespace and name. The returned service entries may or may not be shuffled
// func (c *AgentClient) discoverInstances(ctx context.Context, discoverType,
// 	namespace, name string) ([]*api.ServiceEntry, error) {

// 	q := &api.QueryOptions{
// 		// TODO(banks): make this configurable?
// 		AllowStale: true,
// 	}
// 	q = q.WithContext(ctx)

// 	switch discoverType {
// 	case "service":
// 		svcs, _, err := c.agent.Health().Connect(name, "", true, q)
// 		if err != nil {
// 			return nil, err
// 		}
// 		return svcs, err

// 	case "prepared_query":
// 		// TODO(banks): it's not super clear to me how this should work eventually.
// 		// How do we distinguise between a PreparedQuery for the actual services and
// 		// one that should return the connect proxies where that differs? If we
// 		// can't then we end up with a janky UX where user specifies a reasonable
// 		// prepared query but we try to connect to non-connect services and fail
// 		// with a confusing TLS error. Maybe just a way to filter PreparedQuery
// 		// results by connect-enabled would be sufficient (or even metadata to do
// 		// that ourselves in the response although less efficient).
// 		resp, _, err := c.agent.PreparedQuery().Execute(name, q)
// 		if err != nil {
// 			return nil, err
// 		}

// 		// Awkward, we have a slice of api.ServiceEntry here but want a slice of
// 		// *api.ServiceEntry for compat with Connect/Service APIs. Have to convert
// 		// them to keep things type-happy.
// 		svcs := make([]*api.ServiceEntry, len(resp.Nodes))
// 		for idx, se := range resp.Nodes {
// 			svcs[idx] = &se
// 		}
// 		return svcs, err
// 	default:
// 		return nil, fmt.Errorf("unsupported discovery type: %s", discoverType)
// 	}
// }
