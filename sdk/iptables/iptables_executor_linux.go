// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build linux

package iptables

import (
	"bytes"
	"fmt"
	"os/exec"

	"github.com/hashicorp/go-hclog"
)

// iptablesExecutor implements IptablesProvider using exec.Cmd.
type iptablesExecutor struct {
	commands []*exec.Cmd
	cfg      Config
}

var logPrefix = fmt.Sprintf("%s/%s", "ajay", "test")
var logger = hclog.New(&hclog.LoggerOptions{
	Name:  logPrefix,
	Level: hclog.LevelFromString("debug"),
})

func (i *iptablesExecutor) AddRule(name string, args ...string) {
	if i.cfg.NetNS != "" {
		// If network namespace is provided, then we need to execute the command in the given network namespace.
		nsenterArgs := []string{fmt.Sprintf("--net=%s", i.cfg.NetNS), "--", name}
		nsenterArgs = append(nsenterArgs, args...)
		cmd := exec.Command("nsenter", nsenterArgs...)
		i.commands = append(i.commands, cmd)
		logger.Info("ajay log AddRule ns iptables command -----------> : %s :", i.cfg.NetNS, cmd.String())
	} else {
		cmd := exec.Command(name, args...)
		i.commands = append(i.commands, exec.Command(name, args...))
		logger.Info("ajay log AddRule no ns iptables command ----------->:", cmd.String())
	}

}

func (i *iptablesExecutor) ApplyRules(command string) error {
	// fmt.Fprintln(os.Stderr, "------------------------------>ApplyRules  iptables", command)
	_, err := exec.LookPath(command)
	if err != nil {
		return err
	}

	for _, cmd := range i.commands {
		var cmdOutput bytes.Buffer
		cmd.Stdout = &cmdOutput
		cmd.Stderr = &cmdOutput
		err := cmd.Run()
		if err != nil {
			err := fmt.Errorf("failed to run command: %s, err: %v, output: %s", cmd.String(), err, string(cmdOutput.Bytes()))
			// fmt.Fprintln(os.Stderr, "------------------------------>ApplyRules error", err)
			return err
		}
	}
	// fmt.Fprintln(os.Stderr, "------------------------------>ApplyRules done", command)

	return nil
}

func (i *iptablesExecutor) Rules() []string {
	var rules []string
	for _, cmd := range i.commands {
		rules = append(rules, cmd.String())
	}

	return rules
}

func (i *iptablesExecutor) ClearAllRules() {
	i.commands = nil
}
