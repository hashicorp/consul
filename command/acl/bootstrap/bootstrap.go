package bootstrap

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/acl/token"
	"github.com/hashicorp/consul/command/flags"
	"github.com/mitchellh/cli"
)

func New(ui cli.Ui) *cmd {
	c := &cmd{UI: ui}
	c.init()
	return c
}

type cmd struct {
	UI     cli.Ui
	flags  *flag.FlagSet
	http   *flags.HTTPFlags
	help   string
	format string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.StringVar(
		&c.format,
		"format",
		token.PrettyFormat,
		fmt.Sprintf("Output format {%s}", strings.Join(token.GetSupportedFormats(), "|")),
	)
	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.ServerFlags())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	args = c.flags.Args()
	if l := len(args); l < 0 || l > 1 {
		c.UI.Error("This command takes up to one argument")
		return 1
	}

	var terminalToken []byte
	var err error

	if len(args) == 1 {
		switch args[0] {
		case "":
			terminalToken = []byte{}
		case "-":
			terminalToken, err = ioutil.ReadAll(os.Stdin)
		default:
			file := args[0]
			terminalToken, err = ioutil.ReadFile(file)
		}
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error reading provided token: %v", err))
			return 1
		}
	}

	// Remove newline from the token if it was passed by stdin
	boottoken := strings.TrimSuffix(string(terminalToken), "\n")

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	var t *api.ACLToken
	if len(boottoken) > 0 {
		t, _, err = client.ACL().BootstrapOpts(boottoken)
	} else {
		t, _, err = client.ACL().Bootstrap()
	}
	if err != nil {
		c.UI.Error(fmt.Sprintf("Failed ACL bootstrapping: %v", err))
		return 1
	}

	formatter, err := token.NewFormatter(c.format, false)
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	out, err := formatter.FormatToken(t)
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	if out != "" {
		c.UI.Info(out)
	}

	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return flags.Usage(c.help, nil)
}

const synopsis = "Bootstrap Consul's ACL system"

// TODO (ACL-V2) - maybe embed link to bootstrap reset docs
const help = `
Usage: consul acl bootstrap [options]

  The bootstrap command will request Consul to generate a new token with unlimited privileges to use
  for management purposes and output its details. This can only be done once and afterwards bootstrapping
  will be disabled. If all tokens are lost and you need to bootstrap again you can follow the bootstrap
  reset procedure
`
