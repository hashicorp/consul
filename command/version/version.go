package version

import (
	"flag"
	"fmt"
	"strings"

	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/command/flags"
	"github.com/hashicorp/consul/version"
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
	format string
	help   string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.StringVar(
		&c.format,
		"format",
		PrettyFormat,
		fmt.Sprintf("Output format {%s}", strings.Join(GetSupportedFormats(), "|")))
	c.help = flags.Usage(help, c.flags)

}

type RPCVersionInfo struct {
	Default int
	Min     int
	Max     int
}

type VersionInfo struct {
	HumanVersion string `json:"-"`
	Version      string
	Revision     string
	Prerelease   string
	RPC          RPCVersionInfo
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	formatter, err := NewFormatter(c.format)
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	out, err := formatter.Format(&VersionInfo{
		HumanVersion: version.GetHumanVersion(),
		Version:      version.Version,
		Revision:     version.GitCommit,
		Prerelease:   version.VersionPrerelease,
		RPC: RPCVersionInfo{
			Default: consul.DefaultRPCProtocol,
			Min:     int(consul.ProtocolVersionMin),
			Max:     consul.ProtocolVersionMax,
		},
	})
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	c.UI.Output(out)
	return 0
}

func (c *cmd) Synopsis() string {
	return "Prints the Consul version"
}

func (c *cmd) Help() string {
	return flags.Usage(c.help, nil)
}

const synopsis = "Output Consul version information"
const help = `
Usage: consul version [options]
`
