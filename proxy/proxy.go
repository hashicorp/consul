package proxy

import (
	"context"
	"log"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/connect"
)

// Proxy implements the built-in connect proxy.
type Proxy struct {
	proxyID, token string

	connect  connect.Client
	manager  *Manager
	cfgWatch ConfigWatcher
	cfg      *Config

	logger *log.Logger
}

// NewFromConfigFile returns a Proxy instance configured just from a local file.
// This is intended mostly for development and bypasses the normal mechanisms
// for fetching config and certificates from the local agent.
func NewFromConfigFile(client *api.Client, filename string,
	logger *log.Logger) (*Proxy, error) {
	cfg, err := ParseConfigFile(filename)
	if err != nil {
		return nil, err
	}

	connect, err := connect.NewInsecureDevClientWithLocalCerts(client,
		cfg.DevCAFile, cfg.DevServiceCertFile, cfg.DevServiceKeyFile)
	if err != nil {
		return nil, err
	}

	p := &Proxy{
		proxyID:  cfg.ProxyID,
		connect:  connect,
		manager:  NewManagerWithLogger(logger),
		cfgWatch: NewStaticConfigWatcher(cfg),
		logger:   logger,
	}
	return p, nil
}

// New returns a Proxy with the given id, consuming the provided (configured)
// agent. It is ready to Run().
func New(client *api.Client, proxyID string, logger *log.Logger) (*Proxy, error) {
	p := &Proxy{
		proxyID:  proxyID,
		connect:  connect.NewClient(client),
		manager:  NewManagerWithLogger(logger),
		cfgWatch: &AgentConfigWatcher{client: client},
		logger:   logger,
	}
	return p, nil
}

// Run the proxy instance until a fatal error occurs or ctx is cancelled.
func (p *Proxy) Run(ctx context.Context) error {
	defer p.manager.StopAll()

	// Watch for config changes (initial setup happens on first "change")
	for {
		select {
		case newCfg := <-p.cfgWatch.Watch():
			p.logger.Printf("[DEBUG] got new config")
			if p.cfg == nil {
				// Initial setup
				err := p.startPublicListener(ctx, newCfg.PublicListener)
				if err != nil {
					return err
				}
			}

			// TODO add/remove upstreams properly based on a diff with current
			for _, uc := range newCfg.Upstreams {
				uc.Client = p.connect
				uc.logger = p.logger
				err := p.manager.RunProxier(uc.String(), NewUpstream(uc))
				if err == ErrExists {
					continue
				}
				if err != nil {
					p.logger.Printf("[ERR] failed to start upstream %s: %s", uc.String(),
						err)
				}
			}
			p.cfg = newCfg

		case <-ctx.Done():
			return nil
		}
	}
}

func (p *Proxy) startPublicListener(ctx context.Context,
	cfg PublicListenerConfig) error {

	// Get TLS creds
	tlsCfg, err := p.connect.ServerTLSConfig()
	if err != nil {
		return err
	}
	cfg.TLSConfig = tlsCfg

	cfg.logger = p.logger
	return p.manager.RunProxier("public_listener", NewPublicListener(cfg))
}
