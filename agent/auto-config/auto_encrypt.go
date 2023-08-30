package autoconf

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/hashicorp/consul/agent/structs"
)

func (ac *AutoConfig) autoEncryptInitialCerts(ctx context.Context) (*structs.SignedResponse, error) {
	// generate a CSR
	csr, key, err := ac.generateCSR()
	if err != nil {
		return nil, err
	}

	ac.acConfig.Waiter.Reset()
	for {
		resp, err := ac.autoEncryptInitialCertsOnce(ctx, csr, key)
		switch {
		case err == nil && resp != nil:
			return resp, nil
		case err != nil:
			ac.logger.Error(err.Error())
		default:
			ac.logger.Error("No error returned when fetching certificates from the servers but no response was either")
		}

		if err := ac.acConfig.Waiter.Wait(ctx); err != nil {
			ac.logger.Info("interrupted during retrieval of auto-encrypt certificates", "err", err)
			return nil, err
		}
	}
}

func (ac *AutoConfig) autoEncryptInitialCertsOnce(ctx context.Context, csr, key string) (*structs.SignedResponse, error) {
	request := structs.CASignRequest{
		WriteRequest: structs.WriteRequest{Token: ac.acConfig.Tokens.AgentToken()},
		Datacenter:   ac.config.Datacenter,
		CSR:          csr,
	}
	var resp structs.SignedResponse

	servers, err := ac.joinHosts()
	if err != nil {
		return nil, err
	}

	for _, s := range servers {
		// try each IP to see if we can successfully make the request
		for _, addr := range ac.resolveHost(s) {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}

			ac.logger.Debug("making AutoEncrypt.Sign RPC", "addr", addr.String())
			err = ac.acConfig.DirectRPC.RPC(ac.config.Datacenter, ac.config.NodeName, &addr, "AutoEncrypt.Sign", &request, &resp)
			if err != nil {
				ac.logger.Error("AutoEncrypt.Sign RPC failed", "addr", addr.String(), "error", err)
				continue
			}

			resp.IssuedCert.PrivateKeyPEM = key
			return &resp, nil
		}
	}
	return nil, fmt.Errorf("No servers successfully responded to the auto-encrypt request")
}

func (ac *AutoConfig) joinHosts() ([]string, error) {
	// use servers known to gossip if there are any
	if ac.acConfig.ServerProvider != nil {
		if srv := ac.acConfig.ServerProvider.FindLANServer(); srv != nil {
			return []string{srv.Addr.String()}, nil
		}
	}

	hosts, err := ac.discoverServers(ac.config.RetryJoinLAN)
	if err != nil {
		return nil, err
	}

	var addrs []string

	// The addresses we use for auto-encrypt are the retry join and start join
	// addresses. These are for joining serf and therefore we cannot rely on the
	// ports for these. This loop strips any port that may have been specified and
	// will let subsequent resolveAddr calls add on the default RPC port.
	for _, addr := range append(ac.config.StartJoinAddrsLAN, hosts...) {
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			if strings.Contains(err.Error(), "missing port in address") {
				host = addr
			} else {
				ac.logger.Warn("error splitting host address into IP and port", "address", addr, "error", err)
				continue
			}
		}
		addrs = append(addrs, host)
	}

	if len(addrs) == 0 {
		return nil, fmt.Errorf("no auto-encrypt server addresses available for use")
	}

	return addrs, nil
}
