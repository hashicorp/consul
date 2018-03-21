package proxy

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/hashicorp/consul/connect"
)

// Upstream provides an implementation of Proxier that listens for inbound TCP
// connections on the private network shared with the proxied application
// (typically localhost). For each accepted connection from the app, it uses the
// connect.Client to discover an instance and connect over mTLS.
type Upstream struct {
	cfg *UpstreamConfig
}

// UpstreamConfig configures the upstream
type UpstreamConfig struct {
	// Client is the connect client to perform discovery with
	Client connect.Client

	// LocalAddress is the host:port to listen on for local app connections.
	LocalBindAddress string `json:"local_bind_address" hcl:"local_bind_address,attr"`

	// DestinationName is the service name of the destination.
	DestinationName string `json:"destination_name" hcl:"destination_name,attr"`

	// DestinationNamespace is the namespace of the destination.
	DestinationNamespace string `json:"destination_namespace" hcl:"destination_namespace,attr"`

	// DestinationType determines which service discovery method is used to find a
	// candidate instance to connect to.
	DestinationType string `json:"destination_type" hcl:"destination_type,attr"`

	// ConnectTimeout is the timeout for establishing connections with the remote
	// service instance. Defaults to 10,000 (10s).
	ConnectTimeoutMs int `json:"connect_timeout_ms" hcl:"connect_timeout_ms,attr"`

	logger *log.Logger
}

func (uc *UpstreamConfig) applyDefaults() {
	if uc.ConnectTimeoutMs == 0 {
		uc.ConnectTimeoutMs = 10000
	}
	if uc.logger == nil {
		uc.logger = log.New(os.Stdout, "", log.LstdFlags)
	}
}

// String returns a string that uniquely identifies the Upstream. Used for
// identifying the upstream in log output and map keys.
func (uc *UpstreamConfig) String() string {
	return fmt.Sprintf("%s->%s:%s/%s", uc.LocalBindAddress, uc.DestinationType,
		uc.DestinationNamespace, uc.DestinationName)
}

// NewUpstream returns an outgoing proxy instance with the given config.
func NewUpstream(cfg UpstreamConfig) *Upstream {
	u := &Upstream{
		cfg: &cfg,
	}
	u.cfg.applyDefaults()
	return u
}

// String returns a string that uniquely identifies the Upstream. Used for
// identifying the upstream in log output and map keys.
func (u *Upstream) String() string {
	return u.cfg.String()
}

// Listener implements Proxier
func (u *Upstream) Listener() (net.Listener, error) {
	return net.Listen("tcp", u.cfg.LocalBindAddress)
}

// HandleConn implements Proxier
func (u *Upstream) HandleConn(conn net.Conn) error {
	defer conn.Close()

	// Discover destination instance
	dst, err := u.discoverAndDial()
	if err != nil {
		return err
	}

	// Hand conn and dst over to Conn to manage the byte copying.
	c := NewConn(conn, dst)
	return c.CopyBytes()
}

func (u *Upstream) discoverAndDial() (net.Conn, error) {
	to := time.Duration(u.cfg.ConnectTimeoutMs) * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), to)
	defer cancel()

	switch u.cfg.DestinationType {
	case "service":
		return u.cfg.Client.DialService(ctx, u.cfg.DestinationNamespace,
			u.cfg.DestinationName)

	case "prepared_query":
		return u.cfg.Client.DialPreparedQuery(ctx, u.cfg.DestinationNamespace,
			u.cfg.DestinationName)

	default:
		return nil, fmt.Errorf("invalid destination type %s", u.cfg.DestinationType)
	}
}

/*
// Upstream represents a service that the proxied application needs to connect
// out to. It provides a dedication local TCP listener (usually listening only
// on loopback) and forwards incoming connections to the proxy to handle.
type Upstream struct {
	cfg *UpstreamConfig
	wg  sync.WaitGroup

	proxy    *Proxy
	fatalErr error
}

// NewUpstream creates an upstream ready to attach to a proxy instance with
// Proxy.AddUpstream. An Upstream can only be attached to single Proxy instance
// at once.
func NewUpstream(p *Proxy, cfg *UpstreamConfig) *Upstream {
	return &Upstream{
		cfg:      cfg,
		proxy:    p,
		shutdown: make(chan struct{}),
	}
}

// UpstreamConfig configures the upstream
type UpstreamConfig struct {
	// LocalAddress is the host:port to listen on for local app connections.
	LocalAddress string

	// DestinationName is the service name of the destination.
	DestinationName string

	// DestinationNamespace is the namespace of the destination.
	DestinationNamespace string

	// DestinationType determines which service discovery method is used to find a
	// candidate instance to connect to.
	DestinationType string
}

// String returns a string representation for the upstream for debugging or
// use as a unique key.
func (uc *UpstreamConfig) String() string {
	return fmt.Sprintf("%s->%s:%s/%s", uc.LocalAddress, uc.DestinationType,
		uc.DestinationNamespace, uc.DestinationName)
}

func (u *Upstream) listen() error {
	l, err := net.Listen("tcp", u.cfg.LocalAddress)
	if err != nil {
		u.fatal(err)
		return
	}

	for {
		conn, err := l.Accept()
		if err != nil {
			return err
		}

		go u.discoverAndConnect(conn)
	}
}

func (u *Upstream) discoverAndConnect(src net.Conn) {
	// First, we need an upstream instance from Consul to connect to
	dstAddrs, err := u.discoverInstances()
	if err != nil {
		u.fatal(fmt.Errorf("failed to discover upstream instances: %s", err))
		return
	}

	if len(dstAddrs) < 1 {
		log.Printf("[INFO] no instances found for %s", len(dstAddrs), u)
	}

	// Attempt connection to first one that works
	// TODO: configurable number/deadline?
	for idx, addr := range dstAddrs {
		err := u.proxy.startProxyingConn(src, addr, false)
		if err != nil {
			log.Printf("[INFO] failed to connect to %s: %s (%d of %d)", addr, err,
				idx+1, len(dstAddrs))
			continue
		}
		return
	}

	log.Printf("[INFO] failed to connect to all %d instances for %s",
		len(dstAddrs), u)
}

func (u *Upstream) discoverInstances() ([]string, error) {
	switch u.cfg.DestinationType {
	case "service":
		svcs, _, err := u.cfg.Consul.Health().Service(u.cfg.DestinationName, "",
			true, nil)
		if err != nil {
			return nil, err
		}

		addrs := make([]string, len(svcs))

		// Shuffle order as we go since health endpoint doesn't
		perm := rand.Perm(len(addrs))
		for i, se := range svcs {
			// Pick location in output array based on next permutation position
			j := perm[i]
			addrs[j] = fmt.Sprintf("%s:%d", se.Service.Address, se.Service.Port)
		}

		return addrs, nil

	case "prepared_query":
		pqr, _, err := u.cfg.Consul.PreparedQuery().Execute(u.cfg.DestinationName,
			nil)
		if err != nil {
			return nil, err
		}

		addrs := make([]string, 0, len(svcs))
		for _, se := range pqr.Nodes {
			addrs = append(addrs, fmt.Sprintf("%s:%d", se.Service.Address,
				se.Service.Port))
		}

		// PreparedQuery execution already shuffles the result
		return addrs, nil

	default:
		u.fatal(fmt.Errorf("invalid destination type %s", u.cfg.DestinationType))
	}
}

func (u *Upstream) fatal(err Error) {
	log.Printf("[ERROR] upstream %s stopping on error: %s", u.cfg.LocalAddress,
		err)
	u.fatalErr = err
}

// String returns a string representation for the upstream for debugging or
// use as a unique key.
func (u *Upstream) String() string {
	return u.cfg.String()
}
*/
