package connect

import (
	"context"
	"crypto/tls"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/hashicorp/consul/api"
	"golang.org/x/net/http2"
)

// Service represents a Consul service that accepts and/or connects via Connect.
// This can represent a service that only is a server, only is a client, or
// both.
//
// TODO(banks): API for monitoring status of certs from app
//
// TODO(banks): Agent implicit health checks based on knowing which certs are
// available should prevent clients being routed until the agent knows the
// service has been delivered valid certificates. Once built, document that here
// too.
type Service struct {
	// serviceID is the unique ID for this service in the agent-local catalog.
	// This is often but not always the service name. This is used to request
	// Connect metadata. If the service with this ID doesn't exist on the local
	// agent no error will be returned and the Service will retry periodically.
	// This allows service startup and registration to happen in either order
	// without coordination since they might be performed by separate processes.
	serviceID string

	// client is the Consul API client. It must be configured with an appropriate
	// Token that has `service:write` policy on the provided ServiceID. If an
	// insufficient token is provided, the Service will abort further attempts to
	// fetch certificates and print a loud error message. It will not Close() or
	// kill the process since that could lead to a crash loop in every service if
	// ACL token was revoked. All attempts to dial will error and any incoming
	// connections will fail to verify.
	client *api.Client

	// serverTLSCfg is the (reloadable) TLS config we use for serving.
	serverTLSCfg *reloadableTLSConfig

	// clientTLSCfg is the (reloadable) TLS config we use for dialling.
	clientTLSCfg *reloadableTLSConfig

	// httpResolverFromAddr is a function that returns a Resolver from a string
	// address for HTTP clients. It's privately pluggable to make testing easier
	// but will default to a simple method to parse the host as a Consul DNS host.
	//
	// TODO(banks): write the proper implementation
	httpResolverFromAddr func(addr string) (Resolver, error)

	logger *log.Logger
}

// NewService creates and starts a Service. The caller must close the returned
// service to free resources and allow the program to exit normally. This is
// typically called in a signal handler.
func NewService(serviceID string, client *api.Client) (*Service, error) {
	return NewServiceWithLogger(serviceID, client,
		log.New(os.Stderr, "", log.LstdFlags))
}

// NewServiceWithLogger starts the service with a specified log.Logger.
func NewServiceWithLogger(serviceID string, client *api.Client,
	logger *log.Logger) (*Service, error) {
	s := &Service{
		serviceID: serviceID,
		client:    client,
		logger:    logger,
	}
	s.serverTLSCfg = newReloadableTLSConfig(defaultTLSConfig(newServerSideVerifier(client, serviceID)))
	s.clientTLSCfg = newReloadableTLSConfig(defaultTLSConfig(clientSideVerifier))

	// TODO(banks) run the background certificate sync
	return s, nil
}

// NewDevServiceFromCertFiles creates a Service using certificate and key files
// passed instead of fetching them from the client.
func NewDevServiceFromCertFiles(serviceID string, client *api.Client,
	logger *log.Logger, caFile, certFile, keyFile string) (*Service, error) {
	s := &Service{
		serviceID: serviceID,
		client:    client,
		logger:    logger,
	}
	tlsCfg, err := devTLSConfigFromFiles(caFile, certFile, keyFile)
	if err != nil {
		return nil, err
	}

	// Note that newReloadableTLSConfig makes a copy so we can re-use the same
	// base for both client and server with swapped verifiers.
	setVerifier(tlsCfg, newServerSideVerifier(client, serviceID))
	s.serverTLSCfg = newReloadableTLSConfig(tlsCfg)
	setVerifier(tlsCfg, clientSideVerifier)
	s.clientTLSCfg = newReloadableTLSConfig(tlsCfg)
	return s, nil
}

// ServerTLSConfig returns a *tls.Config that allows any TCP listener to accept
// and authorize incoming Connect clients. It will return a single static config
// with hooks to dynamically load certificates, and perform Connect
// authorization during verification. Service implementations do not need to
// reload this to get new certificates.
//
// At any time it may be possible that the Service instance does not have access
// to usable certificates due to not being initially setup yet or a prolonged
// error during renewal. The listener will be able to accept connections again
// once connectivity is restored provided the client's Token is valid.
func (s *Service) ServerTLSConfig() *tls.Config {
	return s.serverTLSCfg.TLSConfig()
}

// Dial connects to a remote Connect-enabled server. The passed Resolver is used
// to discover a single candidate instance which will be dialled and have it's
// TLS certificate verified against the expected identity. Failures are returned
// directly with no retries. Repeated dials may use different instances
// depending on the Resolver implementation.
//
// Timeout can be managed via the Context.
func (s *Service) Dial(ctx context.Context, resolver Resolver) (net.Conn, error) {
	addr, certURI, err := resolver.Resolve(ctx)
	if err != nil {
		return nil, err
	}
	s.logger.Printf("[DEBUG] resolved service instance: %s (%s)", addr,
		certURI.URI())
	var dialer net.Dialer
	tcpConn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}

	tlsConn := tls.Client(tcpConn, s.clientTLSCfg.TLSConfig())
	// Set deadline for Handshake to complete.
	deadline, ok := ctx.Deadline()
	if ok {
		tlsConn.SetDeadline(deadline)
	}
	// Perform handshake
	if err = tlsConn.Handshake(); err != nil {
		tlsConn.Close()
		return nil, err
	}
	// Clear deadline since that was only for connection. Caller can set their own
	// deadline later as necessary.
	tlsConn.SetDeadline(time.Time{})

	// Verify that the connect server's URI matches certURI
	err = verifyServerCertMatchesURI(tlsConn.ConnectionState().PeerCertificates,
		certURI)
	if err != nil {
		tlsConn.Close()
		return nil, err
	}
	s.logger.Printf("[DEBUG] successfully connected to %s (%s)", addr,
		certURI.URI())
	return tlsConn, nil
}

// HTTPDialTLS is compatible with http.Transport.DialTLS. It expects the addr
// hostname to be specified using Consul DNS query syntax, e.g.
// "web.service.consul". It converts that into the equivalent ConsulResolver and
// then call s.Dial with the resolver. This is low level, clients should
// typically use HTTPClient directly.
func (s *Service) HTTPDialTLS(network,
	addr string) (net.Conn, error) {
	if s.httpResolverFromAddr == nil {
		return nil, errors.New("no http resolver configured")
	}
	r, err := s.httpResolverFromAddr(addr)
	if err != nil {
		return nil, err
	}
	// TODO(banks): figure out how to do timeouts better.
	return s.Dial(context.Background(), r)
}

// HTTPClient returns an *http.Client configured to dial remote Consul Connect
// HTTP services. The client will return an error if attempting to make requests
// to a non HTTPS hostname. It resolves the domain of the request with the same
// syntax as Consul DNS queries although it performs discovery directly via the
// API rather than just relying on Consul DNS. Hostnames that are not valid
// Consul DNS queries will fail.
func (s *Service) HTTPClient() *http.Client {
	t := &http.Transport{
		// Sadly we can't use DialContext hook since that is expected to return a
		// plain TCP connection an http.Client tries to start a TLS handshake over
		// it. We need to control the handshake to be able to do our validation.
		// So we have to use the older DialTLS which means no context/timeout
		// support.
		//
		// TODO(banks): figure out how users can configure a timeout when using
		// this and/or compatibility with http.Request.WithContext.
		DialTLS: s.HTTPDialTLS,
	}
	// Need to manually re-enable http2 support since we set custom DialTLS.
	// See https://golang.org/src/net/http/transport.go?s=8692:9036#L228
	http2.ConfigureTransport(t)
	return &http.Client{
		Transport: t,
	}
}

// Close stops the service and frees resources.
func (s *Service) Close() error {
	// TODO(banks): stop background activity if started
	return nil
}
