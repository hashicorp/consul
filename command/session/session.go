package session

import (
	"github.com/hashicorp/consul/command/flags"
	"github.com/mitchellh/cli"
)

func New(ui cli.Ui) *cmd {
	return &cmd{}
}

type cmd struct{}

func (c *cmd) Run(args []string) int {
	return cli.RunResultHelp
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return flags.Usage(help, nil)
}

const (
	synopsis = "Manage Consul's sessions"

	help = `
Usage: consul session <subcommands> [options] [args]

    This command has subcommands for managing Consul's sessions.
    Here are some simple examples, and more detailed examples are available
    in the subcommands or the documentation.

    Create a new session:

        $ consul session create -lock-delay=15s \
                                -name=my-service-lock \
                                -node=foobar \
                                -node-check=serfHealth \
                                -node-check=a \
                                -node-check=b \
                                -node-check=c \
                                -behavior=release \
                                -ttl=30s

    List all sessions in the cluster:

        $ consul acl session list

    Read a session information:

        $ consul acl session read b2caae8a-e80e-15f4-17aa-2be947c7968e

    Renew a session:

        $ consul session renew b2caae8a-e80e-15f4-17aa-2be947c7968e

    Delete a session:

        $ consul session delete b2caae8a-e80e-15f4-17aa-2be947c7968e
`
)
