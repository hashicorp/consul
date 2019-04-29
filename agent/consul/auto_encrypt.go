package consul

import (
	"fmt"
	"net"
	"strings"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
)

const DummyTrustDomain = "dummy.trustdomain"

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
	// DummyTrustDomain the first time. Subsequent CSRs will have the
	// correct TrustDomain.
	id := &connect.SpiffeIDAgent{
		Host:       DummyTrustDomain,
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

	args := structs.CASignRequest{
		WriteRequest: structs.WriteRequest{Token: token},
		Datacenter:   c.config.Datacenter,
		CSR:          csr,
	}
	var reply structs.SignResponse
	addrs := []*net.TCPAddr{}

	for _, s := range servers {
		host := strings.SplitN(s, ":", 2)[0]
		addrs = append(addrs, &net.TCPAddr{IP: net.ParseIP(host), Port: port})
	}

	if err = c.RPCInsecure("AutoEncrypt.Sign", &args, &reply, addrs); err != nil {
		return errFn(err)
	}

	return &reply, pkPEM, nil
}
