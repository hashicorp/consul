package agent

import (
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"net"

	"github.com/pkg/errors"

	"github.com/hashicorp/consul/tlsutil"
)

func newSerfEncryptionKey() (string, error) {
	key := make([]byte, 32)
	n, err := rand.Reader.Read(key)
	if err != nil {
		return "", errors.Wrap(err, "error reading random data")
	}
	if n != 32 {
		return "", errors.Wrap(err, "couldn't read enough entropy. Generate more entropy!")
	}

	return base64.StdEncoding.EncodeToString(key), nil
}

func newServerTLSKeyPair(dc string, ctx *BuildContext) (string, string, string, string) {
	// Generate agent-specific key pair. Borrowed from 'consul tls cert create -server -dc <dc_name>'
	name := fmt.Sprintf("server.%s.%s", dc, "consul")

	dnsNames := []string{
		name,
		"localhost",
	}
	ipAddresses := []net.IP{net.ParseIP("127.0.0.1")}
	extKeyUsage := []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}

	signer, err := tlsutil.ParseSigner(ctx.caKey)
	if err != nil {
		panic("could not parse signer from CA key")
	}

	pub, priv, err := tlsutil.GenerateCert(tlsutil.CertOpts{
		Signer: signer, CA: ctx.caCert, Name: name, Days: 365,
		DNSNames: dnsNames, IPAddresses: ipAddresses, ExtKeyUsage: extKeyUsage,
	})

	prefix := fmt.Sprintf("%s-server-%s", dc, "consul")
	certFileName := fmt.Sprintf("%s-%d.pem", prefix, ctx.index)
	keyFileName := fmt.Sprintf("%s-%d-key.pem", prefix, ctx.index)

	if err = tlsutil.Verify(ctx.caCert, pub, name); err != nil {
		panic(fmt.Sprintf("could not verify keypair for %s and %s", certFileName, keyFileName))
	}

	return keyFileName, priv, certFileName, pub
}
