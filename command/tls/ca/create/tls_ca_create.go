package create

import (
	"flag"
	"fmt"
	"os"

	"github.com/hashicorp/consul/agent/connect"

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
	UI    cli.Ui
	flags *flag.FlagSet
	help  string
	days  int
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.IntVar(&c.days, "days", 1825, "Provide number of days the CA is valid for from now on, defaults to 5 years.")
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
	prefix := "consul"
	if len(c.flags.Args()) > 0 {
		prefix = c.flags.Args()[0]
	}

	certFileName := fmt.Sprintf("%s-ca.pem", prefix)
	pkFileName := fmt.Sprintf("%s-ca-key.pem", prefix)

	if !(tls.FileDoesNotExist(certFileName)) {
		c.UI.Error(certFileName + " already exists.")
		return 1
	}
	if !(tls.FileDoesNotExist(pkFileName)) {
		c.UI.Error(pkFileName + " already exists.")
		return 1
	}

	sn, err := connect.GenerateSerialNumber()
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	s, pk, err := connect.GeneratePrivateKey()
	if err != nil {
		c.UI.Error(err.Error())
	}
	ca, err := connect.GenerateCA(s, sn, c.days, nil)
	if err != nil {
		c.UI.Error(err.Error())
	}
	caFile, err := os.Create(certFileName)
	if err != nil {
		c.UI.Error(err.Error())
	}
	caFile.WriteString(ca)
	c.UI.Output("==> Saved " + certFileName)
	pkFile, err := os.Create(pkFileName)
	if err != nil {
		c.UI.Error(err.Error())
	}
	pkFile.WriteString(pk)
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
Usage: consul tls ca create [filename-prefix] [options]

  Create a new consul CA:

  $ consul tls ca create
  ==> saved consul-ca.pem
  ==> saved consul-ca-key.pem

  Or save it with your own prefix:

  $ consul tls ca create my
  ==> saved my-ca.pem
  ==> saved my-ca-key.pem
`
