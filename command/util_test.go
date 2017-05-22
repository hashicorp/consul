package command

import (
	"strings"
	"testing"

	"github.com/hashicorp/consul/version"
	"github.com/mitchellh/cli"
)

func init() {
	version.Version = "0.8.0"
}

func assertNoTabs(t *testing.T, c cli.Command) {
	if strings.ContainsRune(c.Help(), '\t') {
		t.Errorf("%#v help output contains tabs", c)
	}
}
