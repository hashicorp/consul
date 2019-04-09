package consul

import (
	"fmt"
	"net"
	"net/rpc"
	"net/url"
	"strings"
	"time"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/tlsutil"
	"github.com/hashicorp/go-msgpack/codec"
)

func (c *Client) AutoEncrypt(servers []string, port int, token string) (*structs.SignResponse, string, error) {
	if len(servers) == 0 {
		return nil, "", fmt.Errorf("No servers to request AutoEncrypt.Sign")
	}

	DNSNames := []string{"client.dc1.consul", "localhost"}
	IPAddresses := []net.IP{net.ParseIP("127.0.0.1")}
	uri, err := url.Parse(fmt.Sprintf("spiffe://%s/agent/%s", c.config.NodeName, c.config.NodeID))
	if err != nil {
		return nil, "", err
	}

	autoEncryptTLSConfigurator, err := tlsutil.NewConfigurator(c.config.ToTLSUtilConfig(), c.logger)
	autoEncryptTLSConfigurator.EnableAutoEncryptModeStartup()
	if err != nil {
		return nil, "", err
	}

	csr, priv, err := tlsutil.GenerateCSR(uri, DNSNames, IPAddresses)

	reply := structs.SignResponse{}
	args := structs.CASignRequest{
		WriteRequest: structs.WriteRequest{Token: token},
		Datacenter:   c.config.Datacenter,
		CSR:          csr,
	}
	var lastError error
	for _, server := range servers {
		host := strings.SplitN(server, ":", 2)[0]
		addr := &net.TCPAddr{IP: net.ParseIP(host), Port: port}

		// Make the request.
		conn, hc, err := c.connPool.DialTimeoutInsecure(c.config.Datacenter, addr, 1*time.Second, autoEncryptTLSConfigurator.OutgoingRPCWrapper())
		if err != nil {
			c.logger.Printf("[DEBUG] consul: AutoEncrypt error: %v", err)
			lastError = err
			continue
		}

		// Push the header encoded as msgpack, then stream the input.
		enc := codec.NewEncoder(conn, &codec.MsgpackHandle{})
		request := rpc.Request{
			Seq:           1,
			ServiceMethod: "AutoEncrypt.Sign",
		}
		if err := enc.Encode(&request); err != nil {
			err := fmt.Errorf("failed to encode request: %v", err)
			c.logger.Printf("[DEBUG] consul: AutoEncrypt error: %v", err)
			lastError = err
			continue
		}

		if err := enc.Encode(&args); err != nil {
			err := fmt.Errorf("failed to encode args: %v", err)
			c.logger.Printf("[DEBUG] consul: AutoEncrypt error: %v", err)
			lastError = err
			continue
		}

		// Our RPC protocol requires support for a half-close in order to signal
		// the other side that they are done reading the stream, since we don't
		// know the size in advance. This saves us from having to buffer just to
		// calculate the size.
		if hc != nil {
			if err := hc.CloseWrite(); err != nil {
				err := fmt.Errorf("failed to half close snapshot connection: %v", err)
				c.logger.Printf("[DEBUG] consul: AutoEncrypt error: %v", err)
				lastError = err
				continue
			}
		} else {
			err := fmt.Errorf("snapshot connection requires half-close support")
			c.logger.Printf("[DEBUG] consul: AutoEncrypt error: %v", err)
			lastError = err
			continue
		}

		// Pull the header decoded as msgpack.
		dec := codec.NewDecoder(conn, &codec.MsgpackHandle{})
		response := rpc.Response{}
		if err := dec.Decode(&response); err != nil {
			fmt.Errorf("failed to decode response: %v", err)
			c.logger.Printf("[DEBUG] consul: AutoEncrypt error: %v", err)
			lastError = err
			continue
		}
		if response.Error != "" {
			err := fmt.Errorf("error in response: %v", response.Error)
			c.logger.Printf("[DEBUG] consul: AutoEncrypt error: %v", err)
			lastError = err
			continue
		}

		if err := dec.Decode(&reply); err != nil {
			err := fmt.Errorf("failed to decode reply: %v", err)
			c.logger.Printf("[DEBUG] consul: AutoEncrypt error: %v", err)
			lastError = err
			continue
		}
		return &reply, priv, nil
	}
	return nil, "", lastError
}
