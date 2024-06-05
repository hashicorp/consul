// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	mcli "github.com/mitchellh/cli"

	"github.com/hashicorp/consul/command"
	"github.com/hashicorp/consul/command/cli"
	"github.com/hashicorp/consul/command/version"
	_ "github.com/hashicorp/consul/service_os"
)

func main() {
	os.Exit(StartKvStorage())
	//os.Exit(realMain())
}

func realMain() int {
	log.SetOutput(io.Discard)

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

type KvStorageCommandArgs struct {
	ConfigFilePath string
	RootDir        string
	Bind           string
	Join           string
	RetryJoin      string
	NodeName       string
	Advertise      string
	FirstNode      bool
	RaftVersion    string
}

func StartKvStorage() int {
	rootDir, _ := filepath.Abs(".")
	cmdArgs := KvStorageCommandArgs{
		//ConfigFilePath: fmt.Sprintf("%v/consul.json", rootDir),
		RootDir:   rootDir,
		NodeName:  "viter-core",
		Bind:      "0.0.0.0",
		Join:      "",
		RetryJoin: "",
		Advertise: "192.168.22.59",
		//Advertise:   "192.168.176.41",
		FirstNode:   true,
		RaftVersion: "",
	}

	args := []string{
		"agent",
		//"-server",
		//"-raft-protocol=" + cmdArgs.RaftVersion,
		//fmt.Sprintf("-data-dir=%v/consul", cmdArgs.RootDir),
		//"-bind=" + cmdArgs.Bind,
		//"-join=" + cmdArgs.Join,
		//"-retry-join=" + cmdArgs.Join,
		//"-node=" + cmdArgs.NodeName,
		//"-advertise=" + cmdArgs.Advertise,
	}
	//if cmdArgs.FirstNode {
	//	args = append(args, "-bootstrap")
	//}
	args = append(args, "-dev")
	if cmdArgs.ConfigFilePath != "" {
		args = append(args, fmt.Sprintf("-config-file=%v", cmdArgs.ConfigFilePath))
	}

	ui := &cli.BasicUI{
		BasicUi: mcli.BasicUi{Writer: os.Stdout, ErrorWriter: os.Stderr},
	}
	cmds := command.RegisteredCommands(ui)
	cli := &mcli.CLI{
		Args:         args,
		Commands:     cmds,
		Autocomplete: true,
		Name:         "consul",
		HelpWriter:   os.Stdout,
		ErrorWriter:  os.Stderr,
	}

	exitCode, err := cli.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error executing CLI: %v\n", err)
		return 1
	}

	return exitCode
}
