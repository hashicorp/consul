package local

import (
	"context"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/agent/local"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/token"
)

const source proxycfg.ProxySource = "local"

// SyncConfig contains the dependencies required by Sync.
type SyncConfig struct {
	// Manager is the proxycfg Manager with which proxy services will be registered.
	Manager ConfigManager

	// State is the agent's local state that will be watched for proxy registrations.
	State *local.State

	// Tokens is used to retrieve a fallback ACL token if a service is registered
	// without one.
	Tokens *token.Store

	// NodeName is the name of the local agent node.
	NodeName string

	// Logger will be used to write log messages.
	Logger hclog.Logger
}

// Sync watches the agent's local state and registers/deregisters services with
// the proxycfg Manager ahead-of-time so they're ready immediately when a proxy
// begins an xDS stream.
//
// It runs until the given context is canceled, so should be called it its own
// goroutine.
//
// Note: proxy service definitions from the agent's local state will always
// overwrite definitions of the same service from other sources (e.g. the
// catalog).
func Sync(ctx context.Context, cfg SyncConfig) {
	// Single item buffer is enough since there is no data transferred so this is
	// "level triggering" and we can't miss actual data.
	stateCh := make(chan struct{}, 1)

	cfg.State.Notify(stateCh)
	defer cfg.State.StopNotify(stateCh)

	for {
		sync(cfg)

		select {
		case <-stateCh:
			// Wait for a state change.
		case <-ctx.Done():
			return
		}
	}
}

func sync(cfg SyncConfig) {
	cfg.Logger.Trace("syncing proxy services from local state")

	services := cfg.State.AllServices()

	// Traverse the local state and ensure all proxy services are registered
	for sid, svc := range services {
		if !svc.Kind.IsProxy() {
			continue
		}

		// Retrieve the token used to register the service, or fallback to the
		// default user token. This token is expected to match the token used in
		// the xDS request for this data.
		token := cfg.State.ServiceToken(sid)
		if token == "" {
			token = cfg.Tokens.UserToken()
		}

		id := proxycfg.ProxyID{
			ServiceID: sid,
			NodeName:  cfg.NodeName,

			// Note: we *intentionally* don't set Token here. All watches on local
			// services use the same ACL token, regardless of whatever token is
			// presented in the xDS stream (the token presented to the xDS server
			// is checked before the watch is created).
			Token: "",
		}

		// TODO(banks): need to work out when to default some stuff. For example
		// Proxy.LocalServicePort is practically necessary for any sidecar and can
		// default to the port of the sidecar service, but only if it's already
		// registered and once we get past here, we don't have enough context to
		// know that so we'd need to set it here if not during registration of the
		// proxy service. Sidecar Service in the interim can do that, but we should
		// validate more generally that that is always true.
		err := cfg.Manager.Register(id, svc, source, token, true)
		if err != nil {
			cfg.Logger.Error("failed to watch proxy service",
				"service", sid.String(),
				"error", err,
			)
		}
	}

	// Now see if any proxies were removed
	for _, proxyID := range cfg.Manager.RegisteredProxies(source) {
		if _, ok := services[proxyID.ServiceID]; !ok {
			cfg.Manager.Deregister(proxyID, source)
		}
	}
}

//go:generate mockery --name ConfigManager --inpackage
type ConfigManager interface {
	Watch(id proxycfg.ProxyID) (<-chan *proxycfg.ConfigSnapshot, proxycfg.CancelFunc)
	Register(proxyID proxycfg.ProxyID, service *structs.NodeService, source proxycfg.ProxySource, token string, overwrite bool) error
	Deregister(proxyID proxycfg.ProxyID, source proxycfg.ProxySource)
	RegisteredProxies(source proxycfg.ProxySource) []proxycfg.ProxyID
}
