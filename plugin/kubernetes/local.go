package kubernetes

import (
	"net"

	"github.com/caddyserver/caddy"
	"github.com/coredns/coredns/core/dnsserver"
)

// boundIPs returns the list of non-loopback IPs that CoreDNS is bound to
func boundIPs(c *caddy.Controller) (ips []net.IP) {
	conf := dnsserver.GetConfig(c)
	hosts := conf.ListenHosts
	if hosts == nil || hosts[0] == "" {
		hosts = nil
		addrs, err := net.InterfaceAddrs()
		if err != nil {
			return nil
		}
		for _, addr := range addrs {
			hosts = append(hosts, addr.String())
		}
	}
	for _, host := range hosts {
		ip, _, _ := net.ParseCIDR(host)
		ip4 := ip.To4()
		if ip4 != nil && !ip4.IsLoopback() {
			ips = append(ips, ip4)
			continue
		}
		ip6 := ip.To16()
		if ip6 != nil && !ip6.IsLoopback() {
			ips = append(ips, ip6)
		}
	}
	return ips
}

// LocalNodeName is exclusively used in federation plugin, will be deprecated later.
func (k *Kubernetes) LocalNodeName() string {
	if len(k.localIPs) == 0 {
		return ""
	}

	// Find fist endpoint matching any localIP
	for _, localIP := range k.localIPs {
		for _, ep := range k.APIConn.EpIndexReverse(localIP.String()) {
			for _, eps := range ep.Subsets {
				for _, addr := range eps.Addresses {
					if localIP.Equal(net.ParseIP(addr.IP)) {
						return addr.NodeName
					}
				}
			}
		}
	}
	return ""
}
