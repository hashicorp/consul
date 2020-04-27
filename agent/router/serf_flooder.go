package router

import (
	"fmt"
	"strings"

	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/serf/serf"
)

// FloodAddrPortFn gets the address and port to use for a given server when
// flood-joining. This will return false if it doesn't have one.
type FloodAddrFn func(*metadata.Server) (string, error)

// FloodJoins attempts to make sure all Consul servers in the src Serf
// instance are joined in the dst Serf instance. It assumes names in the
// src area are of the form <node> and those in the dst area are of the
// form <node>.<dc> as is done for WAN and general network areas in Consul
// Enterprise.
func FloodJoins(logger hclog.Logger, addrFn FloodAddrFn,
	localDatacenter string, srcSerf *serf.Serf, dstSerf *serf.Serf) {

	// Names in the dst Serf have the datacenter suffixed.
	suffix := fmt.Sprintf(".%s", localDatacenter)

	// Index the dst side so we can do one pass through the src side
	// with cheap lookups.
	index := make(map[string]*metadata.Server)
	for _, m := range dstSerf.Members() {
		ok, server := metadata.IsConsulServer(m)
		if !ok {
			continue
		}

		if server.Datacenter != localDatacenter {
			continue
		}

		srcName := strings.TrimSuffix(server.Name, suffix)
		index[srcName] = server
	}

	// Now run through the src side and look for joins.
	for _, m := range srcSerf.Members() {
		if m.Status != serf.StatusAlive {
			continue
		}

		ok, server := metadata.IsConsulServer(m)
		if !ok {
			continue
		}

		if _, ok := index[server.Name]; ok {
			continue
		}

		// TODO make RPC

		// TODO refactor into one function:

		addr, err := addrFn(server)
		if err != nil {
			logger.Debug("Failed to flood-join server", "server",
				server.Name, "address", server.Addr.String(),
				"error", err,
			)
			continue
		}
		// // We can't use the port number from the src Serf, so we just
		// // get the host part.
		// addr, _, err := net.SplitHostPort(server.Addr.String())
		// if err != nil {
		// 	logger.Debug("Failed to flood-join server (bad address)",
		// 		"server", server.Name,
		// 		"address", server.Addr.String(),
		// 		"error", err,
		// 	)
		// }
		// if addrFn != nil {
		// 	if a, ok := addrFn(server); ok {
		// 		addr = a
		// 	}
		// }

		// // Let the callback see if it can get the port number, otherwise
		// // leave it blank to behave as if we just supplied an address.
		// if port, ok := portFn(server); ok {
		// 	addr = net.JoinHostPort(addr, fmt.Sprintf("%d", port))
		// } else {
		// 	// If we have an IPv6 address, we should add brackets,
		// 	// single dstSerf.Join expects that.
		// 	if ip := net.ParseIP(addr); ip != nil {
		// 		if ip.To4() == nil {
		// 			addr = fmt.Sprintf("[%s]", addr)
		// 		}
		// 	} else {
		// 		logger.Debug("Failed to parse IP", "ip", addr)
		// 	}
		// }

		// end refactor

		dstServerName := fmt.Sprintf("%s.%s", server.Name, server.Datacenter)

		// Do the join!
		n, err := dstSerf.Join([]string{dstServerName + "/" + addr}, true)
		if err != nil {
			logger.Debug("Failed to flood-join server at address",
				"server", dstServerName,
				"address", addr,
				"error", err,
			)
		} else if n > 0 {
			logger.Debug("Successfully performed flood-join for server at address",
				"server", dstServerName,
				"address", addr,
			)
		}
	}
}
