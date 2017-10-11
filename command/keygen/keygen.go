package keygen

import (
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"

	"github.com/hashicorp/consul/command/flags"
	"github.com/mitchellh/cli"
)

func New(ui cli.Ui) *cmd {
	c := &cmd{UI: ui}
	c.initFlags()
	return c
}

type cmd struct {
	UI    cli.Ui
	flags *flag.FlagSet
}

func (c *cmd) initFlags() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	key := make([]byte, 16)
	n, err := rand.Reader.Read(key)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error reading random data: %s", err))
		return 1
	}
	if n != 16 {
		c.UI.Error(fmt.Sprintf("Couldn't read enough entropy. Generate more entropy!"))
		return 1
	}

	c.UI.Output(base64.StdEncoding.EncodeToString(key))
	return 0
}

func (c *cmd) Synopsis() string {
	return "Generates a new encryption key"
}

func (c *cmd) Help() string {
	s := `Usage: consul keygen

  Generates a new encryption key that can be used to configure the
  agent to encrypt traffic. The output of this command is already
  in the proper format that the agent expects.`

	return flags.Usage(s, c.flags, nil, nil)
}
