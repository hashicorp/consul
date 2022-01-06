package autoconf

import (
	"fmt"
	"github.com/hashicorp/consul/ipaddr"
	"net"
	"strings"

	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/go-discover"
	discoverk8s "github.com/hashicorp/go-discover/provider/k8s"

	"github.com/hashicorp/go-hclog"
)

// Returns ip addresses (without port) of discovered servers. These may be IPv6 addresses.
// This is assumed from the contract of (*Discover) Addrs. It would be nice to validate this somehow
// but not sure how to do this without false positives from IPv6 addresses.
func (ac *AutoConfig) discoverServers(servers []string) ([]string, error) {
	providers := make(map[string]discover.Provider)
	for k, v := range discover.Providers {
		providers[k] = v
	}
	providers["k8s"] = &discoverk8s.Provider{}

	disco, err := discover.New(
		discover.WithUserAgent(lib.UserAgent()),
		discover.WithProviders(providers),
	)

	if err != nil {
		return nil, fmt.Errorf("Failed to create go-discover resolver: %w", err)
	}

	var addrs []string
	for _, addr := range servers {
		switch {
		case strings.Contains(addr, "provider="):
			resolved, err := disco.Addrs(addr, ac.logger.StandardLogger(&hclog.StandardLoggerOptions{InferLevels: true}))
			if err != nil {
				ac.logger.Error("failed to resolve go-discover auto-config servers", "configuration", addr, "err", err)
				continue
			}

			addrs = append(addrs, resolved...)
			ac.logger.Debug("discovered auto-config servers", "servers", resolved)
		default:
			addrs = append(addrs, addr)
		}
	}

	return addrs, nil
}

// autoConfigHosts is responsible for taking the list of server addresses
// and resolving any go-discover provider invocations. It will then return
// a list of hosts. These might be hostnames and is expected that DNS resolution
// may be performed after this function runs. Additionally these may contain
// ports so SplitHostPort could also be necessary.
func (ac *AutoConfig) autoConfigHosts() ([]string, error) {
	// use servers known to gossip if there are any
	if ac.acConfig.ServerProvider != nil {
		if srv := ac.acConfig.ServerProvider.FindLANServer(); srv != nil {
			return []string{srv.Addr.String()}, nil
		}
	}

	addrs, err := ac.discoverServers(ac.config.AutoConfig.ServerAddresses)
	if err != nil {
		return nil, err
	}

	if len(addrs) == 0 {
		return nil, fmt.Errorf("no auto-config server addresses available for use")
	}

	return addrs, nil
}

// resolveHost will take a single host string and convert it to a list of TCPAddrs
// This will process any port in the input as well as looking up the hostname using
// normal DNS resolution.
func (ac *AutoConfig) resolveHost(hostPort string) []net.TCPAddr {
	host, port, err := ipaddr.SplitHostPort(hostPort)
	if err != nil {
		ac.logger.Warn("error splitting host address into IP and port", "address", hostPort, "error", err)
		return nil
	} else {
		if port == -1 {
			port = ac.config.ServerPort
		}
	}

	// resolve the host to a list of IPs
	ips, err := net.LookupIP(host)
	if err != nil {
		ac.logger.Warn("IP resolution failed", "host", host, "error", err)
		return nil
	}

	var addrs []net.TCPAddr
	for _, ip := range ips {
		addrs = append(addrs, net.TCPAddr{IP: ip, Port: port})
	}

	return addrs
}
