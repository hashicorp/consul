// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package connect

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/api/watch"
	"github.com/hashicorp/consul/logging"
	"github.com/hashicorp/go-hclog"
	"golang.org/x/net/http2"
)

// Service represents a Consul service that accepts and/or connects via Connect.
// This can represent a service that only is a server, only is a client, or
// both.
//
// TODO(banks): Agent implicit health checks based on knowing which certs are
// available should prevent clients being routed until the agent knows the
// service has been delivered valid certificates. Once built, document that here
// too.
type Service struct {
	// service is the name (not ID) for the Consul service. This is used to request
	// Connect metadata.
	service string

	// client is the Consul API client. It must be configured with an appropriate
	// Token that has `service:write` policy on the provided service. If an
	// insufficient token is provided, the Service will abort further attempts to
	// fetch certificates and print a loud error message. It will not Close() or
	// kill the process since that could lead to a crash loop in every service if
	// ACL token was revoked. All attempts to dial will error and any incoming
	// connections will fail to verify. It may be nil if the Service is being
	// configured from local files for development or testing.
	client *api.Client

	// tlsCfg is the dynamic TLS config
	tlsCfg *dynamicTLSConfig

	// httpResolverFromAddr is a function that returns a Resolver from a string
	// address for HTTP clients. It's privately pluggable to make testing easier
	// but will default to a simple method to parse the host as a Consul DNS host.
	httpResolverFromAddr func(addr string) (Resolver, error)

	rootsWatch *watch.Plan
	leafWatch  *watch.Plan

	logger hclog.Logger
}

// Config represents the configuration options for a service.
type Config struct {
	// client is the mandatory Consul API client. Will panic if not set.
	Client *api.Client
	// Logger is the logger to use. If nil a default logger will be used.
	Logger hclog.Logger
	// ServerNextProtos are the protocols advertised via ALPN. If nil, defaults to
	// ["h2"] for backwards compatibility. Usually there is no need to change this,
	// see https://github.com/hashicorp/consul/issues/4466 for some discussion on why
	// this can be useful.
	ServerNextProtos []string
}

// NewServiceWithConfig starts a service with the specified Config.
func NewServiceWithConfig(serviceName string, config Config) (*Service, error) {
	if config.Logger == nil {
		config.Logger = hclog.New(&hclog.LoggerOptions{})
	}
	tlsCfg := defaultTLSConfig()
	if config.ServerNextProtos != nil {
		tlsCfg.NextProtos = config.ServerNextProtos
	}
	s := &Service{
		service:              serviceName,
		client:               config.Client,
		logger:               config.Logger.Named(logging.Connect).With("service", serviceName),
		tlsCfg:               newDynamicTLSConfig(tlsCfg, config.Logger),
		httpResolverFromAddr: ConsulResolverFromAddrFunc(config.Client),
	}

	// Set up root and leaf watches
	p, err := watch.Parse(map[string]interface{}{
		"type": "connect_roots",
	})
	if err != nil {
		return nil, err
	}
	s.rootsWatch = p
	s.rootsWatch.HybridHandler = s.rootsWatchHandler

	p, err = watch.Parse(map[string]interface{}{
		"type":    "connect_leaf",
		"service": s.service,
	})
	if err != nil {
		return nil, err
	}
	s.leafWatch = p
	s.leafWatch.HybridHandler = s.leafWatchHandler

	go s.rootsWatch.RunWithClientAndHclog(config.Client, s.logger)
	go s.leafWatch.RunWithClientAndHclog(config.Client, s.logger)

	return s, nil
}

// NewService creates and starts a Service. The caller must close the returned
// service to free resources and allow the program to exit normally. This is
// typically called in a signal handler.
//
// Caller must provide client which is already configured to speak to the local
// Consul agent, and with an ACL token that has `service:write` privileges for
// the service specified.
func NewService(serviceName string, client *api.Client) (*Service, error) {
	return NewServiceWithConfig(serviceName, Config{Client: client})
}

// NewServiceWithLogger starts the service with a specified log.Logger.
func NewServiceWithLogger(serviceName string, client *api.Client,
	logger hclog.Logger) (*Service, error) {
	return NewServiceWithConfig(serviceName, Config{Client: client, Logger: logger})
}

// NewDevServiceFromCertFiles creates a Service using certificate and key files
// passed instead of fetching them from the client.
func NewDevServiceFromCertFiles(serviceID string, logger hclog.Logger,
	caFile, certFile, keyFile string) (*Service, error) {

	tlsCfg, err := devTLSConfigFromFiles(caFile, certFile, keyFile)
	if err != nil {
		return nil, err
	}
	return NewDevServiceWithTLSConfig(serviceID, logger, tlsCfg)
}

// NewDevServiceWithTLSConfig creates a Service using static TLS config passed.
// It's mostly useful for testing.
func NewDevServiceWithTLSConfig(serviceName string, logger hclog.Logger,
	tlsCfg *tls.Config) (*Service, error) {
	s := &Service{
		service: serviceName,
		logger:  logger,
		tlsCfg:  newDynamicTLSConfig(tlsCfg, logger),
	}
	return s, nil
}

// Name returns the name of the service this object represents. Note it is the
// service _name_ as used during discovery, not the ID used to uniquely identify
// an instance of the service with an agent.
func (s *Service) Name() string {
	return s.service
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
//
// To prevent routing traffic to the app instance while it's certificates are
// invalid or not populated yet you may use Ready in a health check endpoint
// and/or ReadyWait during startup before starting the TLS listener. The latter
// only prevents connections during initial bootstrap (including permission
// issues where certs can never be issued due to bad credentials) but won't
// handle the case that certificates expire and an error prevents timely
// renewal.
func (s *Service) ServerTLSConfig() *tls.Config {
	return s.tlsCfg.Get(newServerSideVerifier(s.logger, s.client, s.service))
}

// Dial connects to a remote Connect-enabled server. The passed Resolver is used
// to discover a single candidate instance which will be dialed and have it's
// TLS certificate verified against the expected identity. Failures are returned
// directly with no retries. Repeated dials may use different instances
// depending on the Resolver implementation.
//
// Timeout can be managed via the Context.
//
// Calls to Dial made before the Service has loaded certificates from the agent
// will fail. You can prevent this by using Ready or ReadyWait in app during
// startup.
func (s *Service) Dial(ctx context.Context, resolver Resolver) (net.Conn, error) {
	addr, certURI, err := resolver.Resolve(ctx)
	if err != nil {
		return nil, err
	}
	s.logger.Debug("resolved service instance",
		"address", addr,
		"identity", certURI.URI(),
	)
	var dialer net.Dialer
	tcpConn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}

	tlsConn := tls.Client(tcpConn, s.tlsCfg.Get(clientSideVerifier))
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
	s.logger.Debug("successfully connected to service instance",
		"address", addr,
		"identity", certURI.URI(),
	)
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
		// plain TCP connection and http.Client tries to start a TLS handshake over
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
	if s.rootsWatch != nil {
		s.rootsWatch.Stop()
	}
	if s.leafWatch != nil {
		s.leafWatch.Stop()
	}
	return nil
}

func (s *Service) rootsWatchHandler(blockParam watch.BlockingParamVal, raw interface{}) {
	if raw == nil {
		return
	}
	v, ok := raw.(*api.CARootList)
	if !ok || v == nil {
		s.logger.Error("got invalid response from root watch")
		return
	}

	// Got new root certificates, update the tls.Configs.
	roots := x509.NewCertPool()
	for _, root := range v.Roots {
		roots.AppendCertsFromPEM([]byte(root.RootCertPEM))
	}

	s.tlsCfg.SetRoots(roots)
}

func (s *Service) leafWatchHandler(blockParam watch.BlockingParamVal, raw interface{}) {
	if raw == nil {
		return // ignore
	}
	v, ok := raw.(*api.LeafCert)
	if !ok || v == nil {
		s.logger.Error("got invalid response from leaf watch")
		return
	}

	// Got new leaf, update the tls.Configs
	cert, err := tls.X509KeyPair([]byte(v.CertPEM), []byte(v.PrivateKeyPEM))
	if err != nil {
		s.logger.Error("failed to parse new leaf cert", "error", err)
		return
	}

	s.tlsCfg.SetLeaf(&cert)
}

// Ready returns whether or not both roots and a leaf certificate are
// configured. If both are non-nil, they are assumed to be valid and usable.
func (s *Service) Ready() bool {
	return s.tlsCfg.Ready()
}

// ReadyWait returns a chan that is closed when the Service becomes ready
// for use for the first time. Note that if the Service is ready when it is
// called it returns a nil chan. Ready means that it has root and leaf
// certificates configured which we assume are valid. The service may
// subsequently stop being "ready" if it's certificates expire or are revoked
// and an error prevents new ones being loaded but this method will not stop
// returning a nil chan in that case. It is only useful for initial startup. For
// ongoing health Ready() should be used.
func (s *Service) ReadyWait() <-chan struct{} {
	return s.tlsCfg.ReadyWait()
}
