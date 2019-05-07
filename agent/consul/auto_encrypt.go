package consul

import (
	"fmt"
	"net"
	"time"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
)

const (
	dummyTrustDomain  = "dummy.trustdomain"
	retryJitterWindow = 30 * time.Second
)

func (c *Client) AutoEncrypt(servers []string, port int, token string) (*structs.SignResponse, string, error) {
	errFn := func(err error) (*structs.SignResponse, string, error) {
		return nil, "", err
	}

	if len(servers) == 0 {
		return errFn(fmt.Errorf("No servers to request AutoEncrypt.Sign"))
	}

	// We don't provide the correct host here, because we don't know any
	// better at this point. Apart from the domain, we would need the
	// ClusterID, which we don't have. This is why we go with
	// dummyTrustDomain the first time. Subsequent CSRs will have the
	// correct TrustDomain.
	id := &connect.SpiffeIDAgent{
		Host:       dummyTrustDomain,
		Datacenter: c.config.Datacenter,
		Agent:      string(c.config.NodeName),
	}

	// Create a new private key
	pk, pkPEM, err := connect.GeneratePrivateKey()
	if err != nil {
		return errFn(err)
	}

	// Create a CSR.
	csr, err := connect.CreateCSR(id, pk)
	if err != nil {
		return errFn(err)
	}

	// Prepare request and response so that it can be passed to
	// RPCInsecure.
	args := structs.CASignRequest{
		WriteRequest: structs.WriteRequest{Token: token},
		Datacenter:   c.config.Datacenter,
		CSR:          csr,
	}
	var reply structs.SignResponse

	// Translate host:port to net.TCPAddr to make life easier for
	// RPCInsecure.
	addrs := []*net.TCPAddr{}
	for _, s := range servers {
		addr, err := net.ResolveTCPAddr("tcp", s)
		if err != nil {
			continue
		}
		addr.Port = port
		addrs = append(addrs, addr)
	}

	// Retry implementation modelled after https://github.com/hashicorp/consul/pull/5228.
	// TLDR; there is a 30s window from which a random time is picked.
	// Repeat until the call is successful.
	attempts := 0
	for {
		if err = c.RPCInsecure("AutoEncrypt.Sign", &args, &reply, addrs); err == nil {
			return &reply, pkPEM, nil
		}

		delay := lib.RandomStagger(retryJitterWindow)
		interval := (time.Duration(attempts) * delay) + delay
		c.logger.Printf("[WARN] agent: AutoEncrypt failed: %v, retrying in %v", err, interval)
		select {
		case <-time.After(interval):
			continue
		case <-c.shutdownCh:
			return errFn(fmt.Errorf("aborting AutoEncrypt because shutting down"))
		}
	}
}
