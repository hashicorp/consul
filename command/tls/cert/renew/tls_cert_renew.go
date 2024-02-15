// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package renew

import (
	"flag"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/hashicorp/consul/agent/connect"
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
	UI           cli.Ui
	flags        *flag.FlagSet
	ca           string
	key          string
	existingcert string //Path to existing ****cert.pem file that has to be renewed
	existingkey  string //Path to key file of the existingcert has to be renewed
	days         int
	help         string
	dnsnames     flags.AppendSliceValue
	ipaddresses  flags.AppendSliceValue
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.StringVar(&c.ca, "ca", "#DOMAIN#-agent-ca.pem", "Provide path to the ca. Defaults to #DOMAIN#-agent-ca.pem.")
	c.flags.StringVar(&c.key, "key", "#DOMAIN#-agent-ca-key.pem", "Provide path to the key. Defaults to #DOMAIN#-agent-ca-key.pem.")
	c.flags.StringVar(&c.existingkey, "existingkey", "", "Provide path to the existing key like #DOMAIN#-agent-#-key.pem.")
	c.flags.StringVar(&c.existingcert, "existingcert", "", "Provide path to the existing cert like #DOMAIN#-agent-#.pem that has to be renewed.")
	c.flags.IntVar(&c.days, "days", 365, "Provide number of days the certificate is valid for from now on. Defaults to 1 year.")
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

	if c.existingcert == "" {
		c.UI.Error("Please provide the existingcert like #DOMAIN#-agent-#.pem that has to be renewed.")
		return 1
	}
	if c.existingkey == "" {
		c.UI.Error("Please provide the existingkey like #DOMAIN#-agent-#-key.pem.")
		return 1
	}

	certData, err := os.ReadFile(c.existingcert)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error reading existing Cert file: %s", err))
		return 1
	}
	existingCert, err := connect.ParseCert(string(certData))
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error parsing existing Cert file: %s", err))
		return 1
	}

	var DNSNames []string
	var IPAddresses []net.IP
	var name, prefix string

	DNSNames = append(DNSNames, existingCert.DNSNames...)
	for _, d := range c.dnsnames {
		if len(d) > 0 {
			DNSNames = append(DNSNames, strings.TrimSpace(d))
		}
	}

	IPAddresses = append(IPAddresses, existingCert.IPAddresses...)
	for _, i := range c.ipaddresses {
		if len(i) > 0 {
			IPAddresses = append(IPAddresses, net.ParseIP(strings.TrimSpace(i)))
		}
	}

	name = existingCert.Subject.CommonName
	parts := strings.Split(name, ".")
	agent := parts[0]
	var dc string
	var domain string
	if agent != "server" && agent != "client" && agent != "cli" {
		c.UI.Error("Neither client, cli nor server - should not happen")
		return 1
	}
	if len(parts) > 1 {
		dc = parts[1]
	} else {
		dc = "dc1"
	}
	if len(parts) > 2 {
		domain = parts[2]
	} else {
		domain = "consul"
	}
	prefix = fmt.Sprintf("%s-%s-%s", dc, agent, domain)

	var certFileName string
	max := 10000
	for i := 0; i <= max; i++ {
		tmpCert := fmt.Sprintf("%s-%d.pem", prefix, i)
		tmpPk := fmt.Sprintf("%s-%d-key.pem", prefix, i)
		if tls.FileDoesNotExist(tmpCert) && tls.FileDoesNotExist(tmpPk) {
			certFileName = tmpCert
			break
		}
		if i == max {
			c.UI.Error("Could not find a filename that doesn't already exist")
			return 1
		}
	}

	caFile := strings.Replace(c.ca, "#DOMAIN#", domain, 1)
	keyFile := strings.Replace(c.key, "#DOMAIN#", domain, 1)
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
	existingkey, err := os.ReadFile(c.existingkey)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error reading existing key: %s", err))
		return 1
	}

	if agent == "server" {
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

	pub, err := tlsutil.RenewCert(tlsutil.CertOpts{
		Signer: signer, CA: string(cert), Name: name, Days: c.days,
		DNSNames: DNSNames, IPAddresses: IPAddresses, ExtKeyUsage: existingCert.ExtKeyUsage,
	}, existingkey)
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

	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const synopsis = "Renew a existing certificate"
const help = `
Usage: consul tls cert renew [options]

  Renew existing certificate

  $ consul tls cert renew -server
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
