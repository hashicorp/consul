package dnsserver

import (
	"flag"
	"fmt"
	"net"
	"time"

	"github.com/miekg/coredns/middleware"

	"github.com/mholt/caddy"
	"github.com/mholt/caddy/caddyfile"
)

const serverType = "dns"

// Any flags defined here, need to be namespaced to the serverType other
// wise they potentially clash with other server types.
func init() {
	flag.StringVar(&Port, serverType+".port", DefaultPort, "Default port")

	caddy.RegisterServerType(serverType, caddy.ServerType{
		Directives: func() []string { return directives },
		DefaultInput: func() caddy.Input {
			return caddy.CaddyfileInput{
				Filepath:       "Corefile",
				Contents:       []byte(".:" + Port + " {\nwhoami\n}\n"),
				ServerTypeName: serverType,
			}
		},
		NewContext: newContext,
	})
}

func newContext() caddy.Context {
	return &dnsContext{keysToConfigs: make(map[string]*Config)}
}

type dnsContext struct {
	keysToConfigs map[string]*Config

	// configs is the master list of all site configs.
	configs []*Config
}

func (h *dnsContext) saveConfig(key string, cfg *Config) {
	h.configs = append(h.configs, cfg)
	h.keysToConfigs[key] = cfg
}

// InspectServerBlocks make sure that everything checks out before
// executing directives and otherwise prepares the directives to
// be parsed and executed.
func (h *dnsContext) InspectServerBlocks(sourceFile string, serverBlocks []caddyfile.ServerBlock) ([]caddyfile.ServerBlock, error) {
	// Normalize and check all the zone names and check for duplicates
	dups := map[string]string{}
	for _, s := range serverBlocks {
		for i, k := range s.Keys {
			za, err := normalizeZone(k)
			if err != nil {
				return nil, err
			}
			s.Keys[i] = za.String()
			if v, ok := dups[za.Zone]; ok {
				return nil, fmt.Errorf("cannot serve %s - zone already defined for %v", za, v)
			}
			dups[za.Zone] = za.String()

			// Save the config to our master list, and key it for lookups
			cfg := &Config{
				Zone: za.Zone,
				Port: za.Port,
			}
			h.saveConfig(za.String(), cfg)
		}
	}
	return serverBlocks, nil
}

// MakeServers uses the newly-created siteConfigs to create and return a list of server instances.
func (h *dnsContext) MakeServers() ([]caddy.Server, error) {

	// we must map (group) each config to a bind address
	groups, err := groupConfigsByListenAddr(h.configs)
	if err != nil {
		return nil, err
	}
	// then we create a server for each group
	var servers []caddy.Server
	for addr, group := range groups {
		s, err := NewServer(addr, group)
		if err != nil {
			return nil, err
		}
		servers = append(servers, s)
	}

	return servers, nil
}

// AddMiddleware adds a middleware to a site's middleware stack.
func (c *Config) AddMiddleware(m middleware.Middleware) {
	c.Middleware = append(c.Middleware, m)
}

// groupSiteConfigsByListenAddr groups site configs by their listen
// (bind) address, so sites that use the same listener can be served
// on the same server instance. The return value maps the listen
// address (what you pass into net.Listen) to the list of site configs.
// This function does NOT vet the configs to ensure they are compatible.
func groupConfigsByListenAddr(configs []*Config) (map[string][]*Config, error) {
	groups := make(map[string][]*Config)

	for _, conf := range configs {
		if conf.Port == "" {
			conf.Port = Port
		}
		addr, err := net.ResolveTCPAddr("tcp", net.JoinHostPort(conf.ListenHost, conf.Port))
		if err != nil {
			return nil, err
		}
		addrstr := addr.String()
		groups[addrstr] = append(groups[addrstr], conf)
	}

	return groups, nil
}

const (
	// DefaultPort is the default port.
	DefaultPort = "53"
)

// These "soft defaults" are configurable by
// command line flags, etc.
var (
	// Port is the port we listen on by default.
	Port = DefaultPort

	// GracefulTimeout is the maximum duration of a graceful shutdown.
	GracefulTimeout time.Duration
)
