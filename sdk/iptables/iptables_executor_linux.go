// +build linux

package iptables

import (
	"bytes"
	"fmt"
	"os/exec"
)

// iptablesExecutor implements IptablesProvider using exec.Cmd.
type iptablesExecutor struct {
	commands []*exec.Cmd
}

func (i *iptablesExecutor) AddRule(name string, args ...string) {
	i.commands = append(i.commands, exec.Command(name, args...))
}

func (i *iptablesExecutor) ApplyRules() error {
	_, err := exec.LookPath("iptables")
	if err != nil {
		return err
	}

	for _, cmd := range i.commands {
		var cmdOutput bytes.Buffer
		cmd.Stdout = &cmdOutput
		cmd.Stderr = &cmdOutput
		err := cmd.Run()
		if err != nil {
			return fmt.Errorf("failed to run command: %s, err: %v, output: %s", cmd.String(), err, string(cmdOutput.Bytes()))
		}
	}

	return nil
}

func (i *iptablesExecutor) Rules() []string {
	var rules []string
	for _, cmd := range i.commands {
		rules = append(rules, cmd.String())
	}

	return rules
}
