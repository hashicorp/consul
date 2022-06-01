package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"

	mcli "github.com/mitchellh/cli"

	"github.com/hashicorp/consul/command"
	"github.com/hashicorp/consul/command/cli"
	"github.com/hashicorp/consul/command/version"
	"github.com/hashicorp/consul/lib"
	_ "github.com/hashicorp/consul/service_os"
)

func init() {
	lib.SeedMathRand()
}

func main() {
	os.Exit(realMain())
}

func realMain() int {
	log.SetOutput(ioutil.Discard)

	ui := &cli.BasicUI{
		BasicUi: mcli.BasicUi{Writer: os.Stdout, ErrorWriter: os.Stderr},
	}
	cmds := command.RegisteredCommands(ui)
	var names []string
	for c := range cmds {
		names = append(names, c)
	}

	cli := &mcli.CLI{
		Args:         os.Args[1:],
		Commands:     cmds,
		Autocomplete: true,
		Name:         "consul",
		HelpFunc:     mcli.FilteredHelpFunc(names, mcli.BasicHelpFunc("consul")),
		HelpWriter:   os.Stdout,
		ErrorWriter:  os.Stderr,
	}

	if cli.IsVersion() {
		cmd := version.New(ui)
		return cmd.Run(nil)
	}

	exitCode, err := cli.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error executing CLI: %v\n", err)
		return 1
	}

	return exitCode
}
