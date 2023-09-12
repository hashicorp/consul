package create

import (
	"flag"
	"fmt"

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
	UI                    cli.Ui
	flags                 *flag.FlagSet
	help                  string
	days                  int
	domain                string
	clusterID             string
	constraint            bool
	commonName            string
	additionalConstraints flags.AppendSliceValue
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	// TODO: perhaps add a -years arg to better capture user intent given that leap years are a thing
	c.flags.IntVar(&c.days, "days", 1825, "Provide number of days the CA is valid for from now on. Defaults to 5 years.")
	c.flags.BoolVar(&c.constraint, "name-constraint", false, "Add name constraints for the CA. Results in rejecting "+
		"certificates for other DNS than specified. If turned on localhost and -domain will be added to the allowed "+
		"DNS. If the UI is going to be served over HTTPS its DNS has to be added with -additional-constraint. It is not "+
		"possible to add that after the fact! Defaults to false.")
	c.flags.StringVar(&c.domain, "domain", "consul", "Domain of consul cluster. Only used in combination with -name-constraint. Defaults to consul.")
	c.flags.StringVar(&c.clusterID, "cluster-id", "", "ClusterID of the consul cluster, requires -domain to be set as well. When used will set URIs with spiffeid.")
	c.flags.StringVar(&c.commonName, "common-name", "", "Common Name of CA. Defaults to Consul Agent CA.")
	c.flags.Var(&c.additionalConstraints, "additional-name-constraint", "Add name constraints for the CA. Results in rejecting certificates "+
		"for other DNS than specified. Can be used multiple times. Only used in combination with -name-constraint.")
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

	certFileName := fmt.Sprintf("%s-agent-ca.pem", c.domain)
	pkFileName := fmt.Sprintf("%s-agent-ca-key.pem", c.domain)

	if !(tls.FileDoesNotExist(certFileName)) {
		c.UI.Error(certFileName + " already exists.")
		return 1
	}
	if !(tls.FileDoesNotExist(pkFileName)) {
		c.UI.Error(pkFileName + " already exists.")
		return 1
	}

	constraints := []string{}
	if c.constraint {
		constraints = append(c.additionalConstraints, []string{c.domain, "localhost"}...)
	}

	ca, pk, err := tlsutil.GenerateCA(tlsutil.CAOpts{Name: c.commonName, Days: c.days, Domain: c.domain, PermittedDNSDomains: constraints, ClusterID: c.clusterID})
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	if err := file.WriteAtomicWithPerms(certFileName, []byte(ca), 0755, 0666); err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	c.UI.Output("==> Saved " + certFileName)

	if err := file.WriteAtomicWithPerms(pkFileName, []byte(pk), 0755, 0600); err != nil {
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

const synopsis = "Create a new consul CA"
const help = `
Usage: consul tls ca create [options]

  Create a new consul CA:

  $ consul tls ca create
  ==> saved consul-agent-ca.pem
  ==> saved consul-agent-ca-key.pem
`
