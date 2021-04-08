// +build linux

package iptables

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
		err := cmd.Run()
		if err != nil {
			output, err := cmd.CombinedOutput()

			if err != nil {
				return fmt.Errorf("failed to run command: %s, err: %v", cmd.String(), err)
			}

			return fmt.Errorf("failed to run command: %s, err: %v, output: %s", cmd.String(), err, string(output))
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
