package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/hashicorp/consul/command"
	"github.com/hashicorp/consul/command/version"
	"github.com/hashicorp/consul/lib"
	_ "github.com/hashicorp/consul/service_os"
	consulversion "github.com/hashicorp/consul/version"
	"github.com/mitchellh/cli"
)

func init() {
	lib.SeedMathRand()
}

func main() {
	os.Exit(realMain())
}

func realMain() int {
	log.SetOutput(ioutil.Discard)

	ui := &cli.BasicUi{Writer: os.Stdout, ErrorWriter: os.Stderr}
	cmds := command.Map(ui)
	var names []string
	for c := range cmds {
		names = append(names, c)
	}

	cli := &cli.CLI{
		Args:         os.Args[1:],
		Commands:     cmds,
		Autocomplete: true,
		Name:         "consul",
		HelpFunc:     cli.FilteredHelpFunc(names, cli.BasicHelpFunc("consul")),
		HelpWriter:   os.Stdout,
		ErrorWriter:  os.Stderr,
	}

	if cli.IsVersion() {
		cmd := version.New(ui, consulversion.GetHumanVersion())
		return cmd.Run(nil)
	}

	exitCode, err := cli.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error executing CLI: %s\n", err.Error())
		return 1
	}

	return exitCode
}
