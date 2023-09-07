// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package create

import (
	"crypto/x509"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/hashicorp/consul/command/flags"
	"github.com/hashicorp/consul/command/tls"
	"github.com/hashicorp/consul/lib/file"
	"github.com/hashicorp/consul/tlsutil"
	"github.com/mitchellh/cli"
)

func New(ui cli.Ui) *cmd {
	c := &cmd{UI: ui}
	c.init()
	return c
}

type cmd struct {
	UI          cli.Ui
	flags       *flag.FlagSet
	ca          string
	key         string
	server      bool
	client      bool
	cli         bool
	dc          string
	days        int
	domain      string
	help        string
	node        string
	dnsnames    flags.AppendSliceValue
	ipaddresses flags.AppendSliceValue
	prefix      string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.StringVar(&c.ca, "ca", "#DOMAIN#-agent-ca.pem", "Provide path to the ca. Defaults to #DOMAIN#-agent-ca.pem.")
	c.flags.StringVar(&c.key, "key", "#DOMAIN#-agent-ca-key.pem", "Provide path to the key. Defaults to #DOMAIN#-agent-ca-key.pem.")
	c.flags.BoolVar(&c.server, "server", false, "Generate server certificate.")
	c.flags.BoolVar(&c.client, "client", false, "Generate client certificate.")
	c.flags.StringVar(&c.node, "node", "", "When generating a server cert and this is set an additional dns name is included of the form <node>.server.<datacenter>.<domain>.")
	c.flags.BoolVar(&c.cli, "cli", false, "Generate cli certificate.")
	c.flags.IntVar(&c.days, "days", 365, "Provide number of days the certificate is valid for from now on. Defaults to 1 year.")
	c.flags.StringVar(&c.dc, "dc", "dc1", "Provide the datacenter. Matters only for -server certificates. Defaults to dc1.")
	c.flags.StringVar(&c.domain, "domain", "consul", "Provide the domain. Matters only for -server certificates.")
	c.flags.Var(&c.dnsnames, "additional-dnsname", "Provide an additional dnsname for Subject Alternative Names. "+
		"localhost is always included. This flag may be provided multiple times.")
	c.flags.Var(&c.ipaddresses, "additional-ipaddress", "Provide an additional ipaddress for Subject Alternative Names. "+
		"127.0.0.1 is always included. This flag may be provided multiple times.")
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

	if c.node != "" && !c.server {
		c.UI.Error("-node requires -server")
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

	for _, i := range c.ipaddresses {
		if len(i) > 0 {
			IPAddresses = append(IPAddresses, net.ParseIP(strings.TrimSpace(i)))
		}
	}

	if c.server {
		name = fmt.Sprintf("server.%s.%s", c.dc, c.domain)

		if c.node != "" {
			nodeName := fmt.Sprintf("%s.server.%s.%s", c.node, c.dc, c.domain)
			DNSNames = append(DNSNames, nodeName)
		}
		DNSNames = append(DNSNames, name)
		DNSNames = append(DNSNames, "localhost")

		IPAddresses = append(IPAddresses, net.ParseIP("127.0.0.1"))
		extKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}
		prefix = fmt.Sprintf("%s-server-%s", c.dc, c.domain)

	} else if c.client {
		name = fmt.Sprintf("client.%s.%s", c.dc, c.domain)
		DNSNames = append(DNSNames, []string{name, "localhost"}...)
		IPAddresses = append(IPAddresses, net.ParseIP("127.0.0.1"))
		extKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth}
		prefix = fmt.Sprintf("%s-client-%s", c.dc, c.domain)
	} else if c.cli {
		name = fmt.Sprintf("cli.%s.%s", c.dc, c.domain)
		DNSNames = []string{name, "localhost"}
		prefix = fmt.Sprintf("%s-cli-%s", c.dc, c.domain)
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

	caFile := strings.Replace(c.ca, "#DOMAIN#", c.domain, 1)
	keyFile := strings.Replace(c.key, "#DOMAIN#", c.domain, 1)
	cert, err := os.ReadFile(caFile)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error reading CA: %s", err))
		return 1
	}
	key, err := os.ReadFile(keyFile)
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
	c.UI.Info("==> Using " + caFile + " and " + keyFile)

	signer, err := tlsutil.ParseSigner(string(key))
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	pub, priv, err := tlsutil.GenerateCert(tlsutil.CertOpts{
		Signer: signer, CA: string(cert), Name: name, Days: c.days,
		DNSNames: DNSNames, IPAddresses: IPAddresses, ExtKeyUsage: extKeyUsage,
	})
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	if err = tlsutil.Verify(string(cert), pub, name); err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	if err := file.WriteAtomicWithPerms(certFileName, []byte(pub), 0755, 0666); err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	c.UI.Output("==> Saved " + certFileName)

	if err := file.WriteAtomicWithPerms(pkFileName, []byte(priv), 0755, 0600); err != nil {
		c.UI.Error(err.Error())
		return 1
	}
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
  ==> Using consul-agent-ca.pem and consul-agent-ca-key.pem
  ==> Saved dc1-server-consul-0.pem
  ==> Saved dc1-server-consul-0-key.pem
  $ consul tls cert create -client
  ==> Using consul-agent-ca.pem and consul-agent-ca-key.pem
  ==> Saved dc1-client-consul-0.pem
  ==> Saved dc1-client-consul-0-key.pem
`
