package proxy

import (
	"log"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/connect"
)

// Proxy implements the built-in connect proxy.
type Proxy struct {
	proxyID    string
	client     *api.Client
	cfgWatcher ConfigWatcher
	stopChan   chan struct{}
	logger     *log.Logger
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

	service, err := connect.NewDevServiceFromCertFiles(cfg.ProxiedServiceID,
		client, logger, cfg.DevCAFile, cfg.DevServiceCertFile,
		cfg.DevServiceKeyFile)
	if err != nil {
		return nil, err
	}
	cfg.service = service

	p := &Proxy{
		proxyID:    cfg.ProxyID,
		client:     client,
		cfgWatcher: NewStaticConfigWatcher(cfg),
		stopChan:   make(chan struct{}),
		logger:     logger,
	}
	return p, nil
}

// New returns a Proxy with the given id, consuming the provided (configured)
// agent. It is ready to Run().
func New(client *api.Client, proxyID string, logger *log.Logger) (*Proxy, error) {
	p := &Proxy{
		proxyID: proxyID,
		client:  client,
		cfgWatcher: &AgentConfigWatcher{
			client:  client,
			proxyID: proxyID,
			logger:  logger,
		},
		stopChan: make(chan struct{}),
		logger:   logger,
	}
	return p, nil
}

// Serve the proxy instance until a fatal error occurs or proxy is closed.
func (p *Proxy) Serve() error {

	var cfg *Config

	// Watch for config changes (initial setup happens on first "change")
	for {
		select {
		case newCfg := <-p.cfgWatcher.Watch():
			p.logger.Printf("[DEBUG] got new config")
			if newCfg.service == nil {
				p.logger.Printf("[ERR] new config has nil service")
				continue
			}
			if cfg == nil {
				// Initial setup

				newCfg.PublicListener.applyDefaults()
				l := NewPublicListener(newCfg.service, newCfg.PublicListener, p.logger)
				err := p.startListener("public listener", l)
				if err != nil {
					return err
				}
			}

			// TODO(banks) update/remove upstreams properly based on a diff with current. Can
			// store a map of uc.String() to Listener here and then use it to only
			// start one of each and stop/modify if changes occur.
			for _, uc := range newCfg.Upstreams {
				uc.applyDefaults()
				uc.resolver = UpstreamResolverFromClient(p.client, uc)

				l := NewUpstreamListener(newCfg.service, uc, p.logger)
				err := p.startListener(uc.String(), l)
				if err != nil {
					p.logger.Printf("[ERR] failed to start upstream %s: %s", uc.String(),
						err)
				}
			}
			cfg = newCfg

		case <-p.stopChan:
			return nil
		}
	}
}

// startPublicListener is run from the internal state machine loop
func (p *Proxy) startListener(name string, l *Listener) error {
	go func() {
		err := l.Serve()
		if err != nil {
			p.logger.Printf("[ERR] %s stopped with error: %s", name, err)
			return
		}
		p.logger.Printf("[INFO] %s stopped", name)
	}()

	go func() {
		<-p.stopChan
		l.Close()
	}()

	return nil
}

// Close stops the proxy and terminates all active connections. It must be
// called only once.
func (p *Proxy) Close() {
	close(p.stopChan)
}
