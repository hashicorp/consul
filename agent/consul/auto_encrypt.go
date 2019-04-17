package consul

import (
	"fmt"
	"net"
	"net/rpc"
	"strings"
	"time"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/tlsutil"
	"github.com/hashicorp/go-msgpack/codec"
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
	// ClusterID, which we don't have. This is why we go with the domain
	// the first time. Subsequent CSRs will have the correct Host.
	id := &connect.SpiffeIDAgent{
		Host:  strings.TrimSuffix(c.config.Domain, "."),
		Agent: string(c.config.NodeID),
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

	var lastError error
	for _, server := range servers {
		reply, err := c.queryServer(server, port, token, csr)
		if err != nil {
			c.logger.Printf("[DEBUG] consul: AutoEncrypt error: %v", err)
			lastError = err
			continue
		}
		return reply, pkPEM, nil
	}
	return errFn(lastError)
}

func (c *Client) queryServer(server string, port int, token, csr string) (*structs.SignResponse, error) {
	errFn := func(err error) (*structs.SignResponse, error) {
		return nil, err
	}

	autoEncryptTLSConfigurator, err := tlsutil.NewConfigurator(c.tlsConfigurator.Base(), c.logger)
	if err != nil {
		return errFn(err)
	}
	autoEncryptTLSConfigurator.EnableAutoEncryptModeClientStartup()

	host := strings.SplitN(server, ":", 2)[0]
	addr := &net.TCPAddr{IP: net.ParseIP(host), Port: port}

	// Make the request.
	conn, hc, err := c.connPool.DialTimeoutInsecure(c.config.Datacenter, addr, 1*time.Second, autoEncryptTLSConfigurator.OutgoingRPCWrapper())
	if err != nil {
		return errFn(err)
	}

	// Push the header encoded as msgpack, then stream the input.
	enc := codec.NewEncoder(conn, &codec.MsgpackHandle{})
	request := rpc.Request{
		Seq:           1,
		ServiceMethod: "AutoEncrypt.Sign",
	}
	if err := enc.Encode(&request); err != nil {
		return errFn(fmt.Errorf("failed to encode request: %v", err))
	}

	args := structs.CASignRequest{
		WriteRequest: structs.WriteRequest{Token: token},
		Datacenter:   c.config.Datacenter,
		CSR:          csr,
	}

	if err := enc.Encode(&args); err != nil {
		return errFn(fmt.Errorf("failed to encode args: %v", err))
	}

	// Our RPC protocol requires support for a half-close in order to signal
	// the other side that they are done reading the stream, since we don't
	// know the size in advance. This saves us from having to buffer just to
	// calculate the size.
	if hc != nil {
		if err := hc.CloseWrite(); err != nil {
			return errFn(fmt.Errorf("failed to half close connection: %v", err))
		}
	} else {
		return errFn(fmt.Errorf("connection requires half-close support"))
	}

	// Pull the header decoded as msgpack.
	dec := codec.NewDecoder(conn, &codec.MsgpackHandle{})
	response := rpc.Response{}
	if err := dec.Decode(&response); err != nil {
		return errFn(fmt.Errorf("failed to decode response: %v", err))
	}
	if response.Error != "" {
		return errFn(fmt.Errorf("error in response: %v", response.Error))
	}

	var reply structs.SignResponse
	if err := dec.Decode(&reply); err != nil {
		return errFn(fmt.Errorf("failed to decode reply: %v", err))
	}
	return &reply, nil
}
