package proxy

import (
	"crypto/x509"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/connect"
	"github.com/hashicorp/consul/lib"
)

// Proxy implements the built-in connect proxy.
type Proxy struct {
	client     *api.Client
	cfgWatcher ConfigWatcher
	stopChan   chan struct{}
	logger     hclog.Logger
	service    *connect.Service
}

// New returns a proxy with the given configuration source.
//
// The ConfigWatcher can be used to update the configuration of the proxy.
// Whenever a new configuration is detected, the proxy will reconfigure itself.
func New(client *api.Client, cw ConfigWatcher, logger hclog.Logger) (*Proxy, error) {
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
			p.logger.Debug("got new config")

			if cfg == nil {
				// Initial setup

				// Setup telemetry if configured
				// NOTE(kit): As far as I can tell, all of the metrics in the proxy are generated at runtime, so we
				//  don't have any static metrics we initialize at start.
				_, err := lib.InitTelemetry(newCfg.Telemetry, p.logger)
				if err != nil {
					p.logger.Error("proxy telemetry config error", "error", err)
				}

				// Setup Service instance now we know target ID etc
				service, err := newCfg.Service(p.client, p.logger)
				if err != nil {
					return err
				}
				p.service = service

				go func() {
					<-service.ReadyWait()
					p.logger.Info("Proxy loaded config and ready to serve")
					tcfg := service.ServerTLSConfig()
					cert, _ := tcfg.GetCertificate(nil)
					leaf, _ := x509.ParseCertificate(cert.Certificate[0])
					p.logger.Info("Parsed TLS identity", "uri", leaf.URIs[0])

					// Only start a listener if we have a port set. This allows
					// the configuration to disable our public listener.
					if newCfg.PublicListener.BindPort != 0 {
						newCfg.PublicListener.applyDefaults()
						l := NewPublicListener(p.service, newCfg.PublicListener, p.logger)
						err = p.startListener("public listener", l)
						if err != nil {
							// This should probably be fatal.
							p.logger.Error("failed to start public listener", "error", err)
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

				if uc.LocalBindSocketPath != "" {
					p.logger.Error("local_bind_socket_path is not supported with this proxy implementation. "+
						"Can't start upstream.", "upstream", uc.String())
					continue
				}

				if uc.LocalBindPort < 1 {
					p.logger.Error("upstream has no local_bind_port. "+
						"Can't start upstream.", "upstream", uc.String())
					continue
				}

				l := NewUpstreamListener(p.service, p.client, uc, p.logger)
				err := p.startListener(uc.String(), l)
				if err != nil {
					p.logger.Error("failed to start upstream",
						"upstream", uc.String(),
						"error", err,
					)
				}
			}
			cfg = newCfg

		case <-p.stopChan:
			if p.service != nil {
				p.service.Close()
			}
			return nil
		}
	}
}

// startPublicListener is run from the internal state machine loop
func (p *Proxy) startListener(name string, l *Listener) error {
	p.logger.Info("Starting listener", "listener", name, "bind_addr", l.BindAddr())
	go func() {
		err := l.Serve()
		if err != nil {
			p.logger.Error("listener stopped with error", "listener", name, "error", err)
			return
		}
		p.logger.Info("listener stopped", "listener", name)
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
