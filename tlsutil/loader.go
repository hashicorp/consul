package tlsutil

import (
	"crypto/tls"
	"sync"
)

type Loader struct {
	// serverConfig is the currently loaded TLS configuration for incoming connections
	serverConfig *tls.Config

	// serverCert is the currently loaded TLS certificate for incoming connections
	serverCert *tls.Certificate

	// clientConfig is the currently loaded TLS configuration for outgoing connections
	clientConfig *tls.Config

	// clientCert is the currently loaded TLS certificate for outgoing connections
	clientCert *tls.Certificate

	loaderLock sync.RWMutex
}


// GetConfigForClient returns the currently-loaded server TLS configuration when
// the Server accepts an incoming connection. This currently does not consider
// information in the ClientHello and only returns the last loaded configuration.
func (l *Loader) GetConfigForClient(*tls.ClientHelloInfo) (*tls.Config, error) {
	l.loaderLock.RLock()
	defer l.loaderLock.RUnlock()

	// it's acceptable for this callback to return nil, so this is safe even when
	// server configuration has not been loaded (or TLS is disabled)
	return l.serverConfig, nil
}

// Reload triggers a configuration update using the Loader fields.
// TODO(ag) : Allow TLS configuration errors related to cert loading.
//            In that case, continue using the previously loaded cert.
func (l *Loader) Reload(config *Config) error {
	l.loaderLock.Lock()
	defer l.loaderLock.Unlock()

	var err error

	l.serverConfig, err = config.IncomingTLSConfig()
	if err != nil {
		return err
	}

	l.clientConfig, err = config.OutgoingTLSConfig()
	if err != nil {
		return err
	}

	return nil
}

// IncomingTLSConfig generates a TLS configuration for incoming connections.
// Provides a callback to provide per-connection configuration, allowing for
// reloading on the fly.
func (l *Loader) IncomingTLSConfig() (*tls.Config, error) {
	tlsConfig := &tls.Config{}
	tlsConfig.GetConfigForClient = l.GetConfigForClient
	return tlsConfig, nil
}

func (l *Loader) OutgoingTLSConfig() (*tls.Config, error) {
	l.loaderLock.RLock()
	defer l.loaderLock.RUnlock()

	// this may return nil, which indicates that TLS is disabled
	return l.clientConfig, nil
}
