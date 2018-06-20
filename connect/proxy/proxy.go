package proxy

import (
	"bytes"
	"crypto/x509"
	"log"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/connect"
	"github.com/hashicorp/consul/lib"
)

// Proxy implements the built-in connect proxy.
type Proxy struct {
	client     *api.Client
	cfgWatcher ConfigWatcher
	stopChan   chan struct{}
	logger     *log.Logger
	service    *connect.Service
}

// New returns a proxy with the given configuration source.
//
// The ConfigWatcher can be used to update the configuration of the proxy.
// Whenever a new configuration is detected, the proxy will reconfigure itself.
func New(client *api.Client, cw ConfigWatcher, logger *log.Logger) (*Proxy, error) {
	return &Proxy{
		client:     client,
		cfgWatcher: cw,
		stopChan:   make(chan struct{}),
		logger:     logger,
	}, nil
}

// Serve the proxy instance until a fatal error occurs or proxy is closed.
func (p *Proxy) Serve() error {
	var cfg *Config

	// failCh is used to stop Serve and return an error from another goroutine we
	// spawn.
	failCh := make(chan error, 1)

	// Watch for config changes (initial setup happens on first "change")
	for {
		select {
		case err := <-failCh:
			// don't log here, we can log with better context at the point where we
			// write the err to the chan
			return err

		case newCfg := <-p.cfgWatcher.Watch():
			p.logger.Printf("[DEBUG] got new config")

			if cfg == nil {
				// Initial setup

				// Setup telemetry if configured
				_, err := lib.InitTelemetry(newCfg.Telemetry)
				if err != nil {
					p.logger.Printf("[ERR] proxy telemetry config error: %s", err)
				}

				// Setup Service instance now we know target ID etc
				service, err := newCfg.Service(p.client, p.logger)
				if err != nil {
					return err
				}
				p.service = service

				go func() {
					<-service.ReadyWait()
					p.logger.Printf("[INFO] proxy loaded config and ready to serve")
					tcfg := service.ServerTLSConfig()
					cert, _ := tcfg.GetCertificate(nil)
					leaf, _ := x509.ParseCertificate(cert.Certificate[0])
					p.logger.Printf("[DEBUG] leaf: %s roots: %s", leaf.URIs[0],
						bytes.Join(tcfg.RootCAs.Subjects(), []byte(",")))

					// Only start a listener if we have a port set. This allows
					// the configuration to disable our public listener.
					if newCfg.PublicListener.BindPort != 0 {
						newCfg.PublicListener.applyDefaults()
						l := NewPublicListener(p.service, newCfg.PublicListener, p.logger)
						err = p.startListener("public listener", l)
						if err != nil {
							// This should probably be fatal.
							p.logger.Printf("[ERR] failed to start public listener: %s", err)
							failCh <- err
						}
					}
				}()
			}

			// TODO(banks) update/remove upstreams properly based on a diff with current. Can
			// store a map of uc.String() to Listener here and then use it to only
			// start one of each and stop/modify if changes occur.
			for _, uc := range newCfg.Upstreams {
				uc.applyDefaults()
				uc.resolver = UpstreamResolverFromClient(p.client, uc)

				if uc.LocalBindPort < 1 {
					p.logger.Printf("[ERR] upstream %s has no local_bind_port. "+
						"Can't start upstream.", uc.String())
					continue
				}

				l := NewUpstreamListener(p.service, uc, p.logger)
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
	p.logger.Printf("[INFO] %s starting on %s", name, l.BindAddr())
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
	if p.service != nil {
		p.service.Close()
	}
}
