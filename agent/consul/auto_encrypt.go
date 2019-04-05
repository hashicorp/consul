package consul

import (
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/tlsutil"
	"github.com/hashicorp/go-msgpack/codec"
)

func (c *Client) AutoEncrypt(servers []string, token string) {
	c.logger.Println("IN AutoEncrypt", servers, token)
	DNSNames := []string{"client.dc1.consul", "localhost"}
	IPAddresses := []net.IP{net.ParseIP("127.0.0.1")}
	// TODO (Hans): spiffeid
	uri, err := url.Parse("")
	if err != nil {
		return
	}

	autoEncryptTLSConfigurator, err := tlsutil.NewConfigurator(c.config.ToTLSUtilConfig(), c.logger)
	if err != nil {
		c.logger.Printf("[DEBUG] consul: AutoEncrypt error: %v", err)
		return
	}
	snCA, err := tlsutil.GenerateSerialNumber()
	if err != nil {
		c.logger.Printf("[DEBUG] consul: AutoEncrypt error: %v", err)
		return
	}
	s, _, err := tlsutil.GeneratePrivateKey()
	if err != nil {
		c.logger.Printf("[DEBUG] consul: AutoEncrypt error: %v", err)
		return
	}
	ca, err := tlsutil.GenerateCA(s, snCA, 1, nil)

	snCert, err := tlsutil.GenerateSerialNumber()
	if err != nil {
		c.logger.Printf("[DEBUG] consul: AutoEncrypt error: %v", err)
		return
	}

	pub, priv, err := tlsutil.GenerateCert(s, ca, snCert, "", 1, nil, nil, nil)
	if err != nil {
		c.logger.Printf("[DEBUG] consul: AutoEncrypt error: %v", err)
		return
	}
	autoEncryptTLSConfigurator.UpdateConnectCA(ca)
	autoEncryptTLSConfigurator.UpdateConnectCert(pub, priv)

	csr, _, err := tlsutil.GenerateCSR(uri, DNSNames, IPAddresses)
	var reply structs.SignResponse
	args := structs.CASignRequest{
		WriteRequest: structs.WriteRequest{Token: token},
		Datacenter:   c.config.Datacenter,
		CSR:          csr,
	}
	for _, server := range servers {
		parts := strings.Split(server, ":")
		// port, err := strconv.Atoi(parts[1])
		// if err != nil {
		// 	c.logger.Printf("[DEBUG] consul: AutoEncrypt error: %v", err)
		// 	continue
		// }
		addr := &net.TCPAddr{IP: net.ParseIP(parts[0]), Port: 8300}
		// Make the request.
		conn, hc, err := c.connPool.DialTimeoutInsecure(c.config.Datacenter, addr, 1*time.Second, autoEncryptTLSConfigurator.OutgoingRPCWrapper())
		if err != nil {
			c.logger.Printf("[DEBUG] consul: AutoEncrypt error: %v", err)
			continue
		}

		// // Write the snapshot RPC byte to set the mode, then perform the
		// // request.
		// if _, err := conn.Write([]byte{byte(pool.RPCConsul)}); err != nil {
		// 	c.logger.Printf("[DEBUG] consul: AutoEncrypt error: %v", fmt.Errorf("failed to write stream type: %v", err))
		// 	continue
		// }

		// Push the header encoded as msgpack, then stream the input.
		enc := codec.NewEncoder(conn, &codec.MsgpackHandle{})
		if err := enc.Encode(&args); err != nil {
			c.logger.Printf("[DEBUG] consul: AutoEncrypt error: %v", fmt.Errorf("failed to encode request: %v", err))
			continue
		}

		// Our RPC protocol requires support for a half-close in order to signal
		// the other side that they are done reading the stream, since we don't
		// know the size in advance. This saves us from having to buffer just to
		// calculate the size.
		if hc != nil {
			if err := hc.CloseWrite(); err != nil {
				c.logger.Printf("[DEBUG] consul: AutoEncrypt error: %v", fmt.Errorf("failed to half close connection: %v", err))
				continue
			}
		} else {
			c.logger.Printf("[DEBUG] consul: AutoEncrypt error: %v", fmt.Errorf("connection requires half-close support"))
			continue
		}

		// Pull the header decoded as msgpack. The caller can continue to read
		// the conn to stream the remaining data.
		dec := codec.NewDecoder(conn, &codec.MsgpackHandle{})
		if err := dec.Decode(reply); err != nil {
			c.logger.Printf("[DEBUG] consul: AutoEncrypt error: %v", fmt.Errorf("failed to decode response: %v", err))
			continue
		}
		fmt.Printf("EHEHEHEHE: %+v\n", reply)
		return
	}
}
