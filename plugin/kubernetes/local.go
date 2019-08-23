package kubernetes

import (
	"net"
)

func localPodIP() net.IP {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}

	for _, addr := range addrs {
		ip, _, _ := net.ParseCIDR(addr.String())
		ip4 := ip.To4()
		if ip4 != nil && !ip4.IsLoopback() {
			return ip4
		}
		ip6 := ip.To16()
		if ip6 != nil && !ip6.IsLoopback() {
			return ip6
		}
	}
	return nil
}

// LocalNodeName is exclusively used in federation plugin, will be deprecated later.
func (k *Kubernetes) LocalNodeName() string {
	localIP := k.interfaceAddrsFunc()
	if localIP == nil {
		return ""
	}

	// Find endpoint matching localIP
	for _, ep := range k.APIConn.EpIndexReverse(localIP.String()) {
		for _, eps := range ep.Subsets {
			for _, addr := range eps.Addresses {
				if localIP.Equal(net.ParseIP(addr.IP)) {
					return addr.NodeName
				}
			}
		}
	}
	return ""
}
