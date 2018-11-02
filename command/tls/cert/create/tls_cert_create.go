package create

import (
	"crypto/x509"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strings"

	"github.com/hashicorp/consul/command/flags"
	"github.com/hashicorp/consul/command/tls"
	"github.com/mitchellh/cli"
)

func New(ui cli.Ui) *cmd {
	c := &cmd{UI: ui}
	c.init()
	return c
}

type cmd struct {
	UI       cli.Ui
	flags    *flag.FlagSet
	ca       string
	key      string
	server   bool
	client   bool
	cli      bool
	dc       string
	days     int
	domain   string
	help     string
	dnsnames flags.AppendSliceValue
	prefix   string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.StringVar(&c.ca, "ca", "consul-agent-ca.pem", "Provide path to the ca. Defaults to consul-agent-ca.pem.")
	c.flags.StringVar(&c.key, "key", "consul-agent-ca-key.pem", "Provide path to the key. Defaults to consul-agent-ca-key.pem.")
	c.flags.BoolVar(&c.server, "server", false, "Generate server certificate.")
	c.flags.BoolVar(&c.client, "client", false, "Generate client certificate.")
	c.flags.BoolVar(&c.cli, "cli", false, "Generate cli certificate.")
	c.flags.IntVar(&c.days, "days", 365, "Provide number of days the certificate is valid for from now on. Defaults to 1 year.")
	c.flags.StringVar(&c.dc, "dc", "dc1", "Provide the datacenter. Matters only for -server certificates. Defaults to dc1.")
	c.flags.StringVar(&c.domain, "domain", "consul", "Provide the domain. Matters only for -server certificates.")
	c.flags.Var(&c.dnsnames, "additional-dnsname", "Provide an additional dnsname for Subject Alternative Names. "+
		"127.0.0.1 and localhost are always included. This flag may be provided multiple times.")
	c.flags.StringVar(&c.prefix, "prefix", "consul", "Prefix for the generated cert and key. Defaults to consul.")
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		c.UI.Error(fmt.Sprintf("Failed to parse args: %v", err))
		return 1
	}
	if c.ca == "" {
		c.UI.Error("Please provide the ca")
		return 1
	}
	if c.key == "" {
		c.UI.Error("Please provide the key")
		return 1
	}

	if !((c.server && !c.client && !c.cli) ||
		(!c.server && c.client && !c.cli) ||
		(!c.server && !c.client && c.cli)) {
		c.UI.Error("Please provide either -server, -client, or -cli")
		return 1
	}

	var DNSNames []string
	var IPAddresses []net.IP
	var extKeyUsage []x509.ExtKeyUsage
	var name, prefix string

	for _, d := range c.dnsnames {
		if len(d) > 0 {
			DNSNames = append(DNSNames, strings.TrimSpace(d))
		}
	}

	if c.server {
		DNSNames = append(DNSNames, []string{fmt.Sprintf("server.%s.%s", c.dc, c.domain), "localhost"}...)
		IPAddresses = []net.IP{net.ParseIP("127.0.0.1")}
		extKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}
		prefix = fmt.Sprintf("%s-server-%s", c.prefix, c.dc)
		name = "Consul Server Certificate"
	} else if c.client {
		DNSNames = append(DNSNames, "localhost")
		IPAddresses = []net.IP{net.ParseIP("127.0.0.1")}
		extKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth}
		prefix = fmt.Sprintf("%s-client", c.prefix)
		name = "Consul Client Certificate"
	} else if c.cli {
		prefix = fmt.Sprintf("%s-cli", prefix)
		name = "Consul CLI Certificate"
	} else {
		c.UI.Error("Neither client, cli nor server - should not happen")
		return 1
	}

	var pkFileName, certFileName string
	max := 10000
	for i := 0; i <= max; i++ {
		tmpCert := fmt.Sprintf("%s-%d.pem", prefix, i)
		tmpPk := fmt.Sprintf("%s-%d-key.pem", prefix, i)
		if tls.FileDoesNotExist(tmpCert) && tls.FileDoesNotExist(tmpPk) {
			certFileName = tmpCert
			pkFileName = tmpPk
			break
		}
		if i == max {
			c.UI.Error("Could not find a filename that doesn't already exist")
			return 1
		}
	}

	cert, err := ioutil.ReadFile(c.ca)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error reading CA: %s", err))
		return 1
	}
	key, err := ioutil.ReadFile(c.key)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error reading CA key: %s", err))
		return 1
	}

	if c.server {
		c.UI.Info(
			`==> WARNING: Server Certificates grants authority to become a
    server and access all state in the cluster including root keys
    and all ACL tokens. Do not distribute them to production hosts
    that are not server nodes. Store them as securely as CA keys.`)
	}
	c.UI.Info("==> Using " + c.ca + " and " + c.key)

	signer, err := tls.ParseSigner(string(key))
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	sn, err := tls.GenerateSerialNumber()
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	name += fmt.Sprintf(" %d", sn)

	pub, priv, err := tls.GenerateCert(signer, string(cert), sn, name, c.days, DNSNames, IPAddresses, extKeyUsage)
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	certFile, err := os.Create(certFileName)
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	certFile.WriteString(pub)
	c.UI.Output("==> Saved " + certFileName)

	pkFile, err := os.Create(pkFileName)
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	pkFile.WriteString(priv)
	c.UI.Output("==> Saved " + pkFileName)

	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const synopsis = "Create a new certificate"
const help = `
Usage: consul tls cert create [options]

  Create a new certificate

  $ consul tls cert create -server
  ==> WARNING: Server Certificates grants authority to become a
      server and access all state in the cluster including root keys
      and all ACL tokens. Do not distribute them to production hosts
      that are not server nodes. Store them as securely as CA keys.
  ==> Using consul-ca.pem and consul-ca-key.pem
  ==> Saved consul-server-dc1-0.pem
  ==> Saved consul-server-dc1-0-key.pem
  $ consul tls cert -client
  ==> Using consul-ca.pem and consul-ca-key.pem
  ==> Saved consul-client-0.pem
  ==> Saved consul-client-0-key.pem
  $ consul tls cert -cli -prefix my
  ==> Using consul-ca.pem and consul-ca-key.pem
  ==> Saved my-cli-0.pem
  ==> Saved my-cli-0-key.pem
  $ consul tls cert -server -ca my-ca.pem -key my-ca-key.pem -prefix my
  ==> WARNING: Server Certificates grants authority to become a
      server and access all state in the cluster including root keys
      and all ACL tokens. Do not distribute them to production hosts
      that are not server nodes. Store them as securely as CA keys.
  ==> Using my-ca.pem and my-ca-key.pem
  ==> Saved my-server-0.pem
  ==> Saved my-server-0-key.pem
`
