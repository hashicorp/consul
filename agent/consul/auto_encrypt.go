package consul

import (
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/tlsutil"
	"github.com/hashicorp/net-rpc-msgpackrpc"
)

func (c *Client) AutoEncrypt(servers []string, token string) {
	c.logger.Println("IN AutoEncrypt", servers, token)
	DNSNames := []string{"client.dc1.consul", "localhost"}
	IPAddresses := []net.IP{net.ParseIP("127.0.0.1")}
	// TODO (Hans): spiffeid
	uri, err := url.Parse(fmt.Sprintf("spiffe://%s/agent/%s", c.config.NodeName, c.config.NodeID))
	if err != nil {
		c.logger.Printf("[DEBUG] consul: AutoEncrypt error: %v", err)
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
	reply := structs.SignResponse{}
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
		conn, _, err := c.connPool.DialTimeoutInsecure(c.config.Datacenter, addr, 1*time.Second, autoEncryptTLSConfigurator.OutgoingRPCWrapper())
		if err != nil {
			c.logger.Printf("[DEBUG] consul: AutoEncrypt error: %v", err)
			continue
		}

		codec := msgpackrpc.NewClientCodec(conn)
		err = msgpackrpc.CallWithCodec(codec, "AutoEncrypt.Sign", args, reply)
		if err != nil {
			c.logger.Printf("[DEBUG] consul: AutoEncrypt error: %v", err)
			continue
		}

		return
	}
}
